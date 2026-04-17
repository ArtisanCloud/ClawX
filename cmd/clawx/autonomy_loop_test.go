package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"clawx/internal/application/command"
	"clawx/internal/application/service"
	"clawx/internal/domain/execution"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
	chatiface "clawx/internal/interfaces/chat"
)

func TestEvaluateAutonomyLoopStateTransitions(t *testing.T) {
	execDecision := service.Decision{
		Kind: service.DecisionExecute,
		Message: chatiface.Message{
			Text: "修复测试",
		},
	}
	state := newAutonomyLoopState(execDecision, 3)
	state.Round = 1

	next, cont := evaluateAutonomyLoopState(execDecision, state, true, actionPlanApplyResult{Status: "partial"}, "第1步失败", autonomyProgressReport{}, false)
	if !cont || next.Phase != autonomyLoopPhaseContinue || next.StopReason != "status_partial" {
		t.Fatalf("partial should continue: %+v continue=%v", next, cont)
	}

	state = next
	state.Round = 2
	next, cont = evaluateAutonomyLoopState(execDecision, state, true, actionPlanApplyResult{Status: "applied"}, "结论：已执行完成。测试结果：go test ./... 通过。", autonomyProgressReport{}, false)
	if cont || !next.Completed || next.Phase != autonomyLoopPhaseDone {
		t.Fatalf("completed narrative should stop: %+v continue=%v", next, cont)
	}

	state = newAutonomyLoopState(execDecision, 3)
	state.Round = 1
	next, cont = evaluateAutonomyLoopState(execDecision, state, true, actionPlanApplyResult{Status: "applied"}, "已完成初始化，下一步将执行回归测试。", autonomyProgressReport{}, false)
	if !cont || next.Phase != autonomyLoopPhaseContinue || next.StopReason != "narrative_requests_followup" {
		t.Fatalf("continuation narrative should continue: %+v continue=%v", next, cont)
	}

	state = newAutonomyLoopState(execDecision, 3)
	state.Round = 1
	next, cont = evaluateAutonomyLoopState(execDecision, state, true, actionPlanApplyResult{Status: "applied", ReadOnlyQuery: true}, "查询结果：当前版本是 v1.2.3", autonomyProgressReport{}, false)
	if cont || !next.Completed || next.StopReason != "readonly_query_completed" {
		t.Fatalf("read-only query should stop immediately: %+v continue=%v", next, cont)
	}

	state = newAutonomyLoopState(execDecision, 3)
	state.Round = 1
	next, cont = evaluateAutonomyLoopState(execDecision, state, false, actionPlanApplyResult{}, "当前共有 3 个智能体。", autonomyProgressReport{}, false)
	if cont || next.Phase != autonomyLoopPhaseDone || next.StopReason != "narrative_answer_without_plan" {
		t.Fatalf("no action with narrative answer should stop: %+v continue=%v", next, cont)
	}

	state = newAutonomyLoopState(execDecision, 3)
	state.Round = 1
	next, cont = evaluateAutonomyLoopState(execDecision, state, false, actionPlanApplyResult{}, "", autonomyProgressReport{}, false)
	if !cont || next.ConsecutiveNoAction != 1 {
		t.Fatalf("first empty no-action should continue once: %+v continue=%v", next, cont)
	}
	next.Round = 2
	next, cont = evaluateAutonomyLoopState(execDecision, next, false, actionPlanApplyResult{}, "", autonomyProgressReport{}, false)
	if cont || next.StopReason != "no_action_plan" {
		t.Fatalf("second empty no-action should stop: %+v continue=%v", next, cont)
	}
}

func TestEvaluateAutonomyLoopStateProgressReport(t *testing.T) {
	execDecision := service.Decision{
		Kind: service.DecisionExecute,
		Message: chatiface.Message{
			Text: "修复测试",
		},
	}

	state := newAutonomyLoopState(execDecision, 2)
	state.Round = 2
	next, cont := evaluateAutonomyLoopState(execDecision, state, true, actionPlanApplyResult{Status: "applied"}, "", autonomyProgressReport{
		Type: "progress_report",
		Goal: "完成回归测试",
		Done: true,
	}, true)
	if cont || !next.Completed || next.StopReason != "progress_report_done" {
		t.Fatalf("done progress report should stop as completed: %+v continue=%v", next, cont)
	}
	if next.Goal != "完成回归测试" {
		t.Fatalf("expected progress report goal override, got: %s", next.Goal)
	}

	state = newAutonomyLoopState(execDecision, 3)
	state.Round = 1
	next, cont = evaluateAutonomyLoopState(execDecision, state, true, actionPlanApplyResult{Status: "applied"}, "", autonomyProgressReport{
		Type:           "progress_report",
		Done:           false,
		RemainingSteps: []string{"执行回归测试"},
	}, true)
	if !cont || next.StopReason != "progress_report_remaining_steps" || next.Phase != autonomyLoopPhaseContinue {
		t.Fatalf("remaining steps should continue: %+v continue=%v", next, cont)
	}

	state = newAutonomyLoopState(execDecision, 1)
	state.Round = 1
	next, cont = evaluateAutonomyLoopState(execDecision, state, true, actionPlanApplyResult{Status: "applied"}, "", autonomyProgressReport{
		Type:           "progress_report",
		Done:           false,
		RemainingSteps: []string{"补充证据"},
	}, true)
	if cont || next.StopReason != "round_limit_reached_with_remaining_steps" || next.Phase != autonomyLoopPhaseStopped {
		t.Fatalf("remaining steps on max round should stop: %+v continue=%v", next, cont)
	}

	state = newAutonomyLoopState(execDecision, 2)
	state.Round = 1
	next, cont = evaluateAutonomyLoopState(execDecision, state, true, actionPlanApplyResult{Status: "applied"}, "", autonomyProgressReport{
		Type: "progress_report",
		Done: false,
	}, true)
	if !cont || next.StopReason != "progress_report_missing_remaining_steps" || next.Phase != autonomyLoopPhaseContinue {
		t.Fatalf("missing remaining_steps should continue for contract repair: %+v continue=%v", next, cont)
	}

	state = newAutonomyLoopState(execDecision, 1)
	state.Round = 1
	next, cont = evaluateAutonomyLoopState(execDecision, state, true, actionPlanApplyResult{Status: "applied"}, "", autonomyProgressReport{
		Type: "progress_report",
		Done: false,
	}, true)
	if cont || next.StopReason != "round_limit_reached_missing_remaining_steps" || next.Phase != autonomyLoopPhaseStopped {
		t.Fatalf("missing remaining_steps on max round should stop: %+v continue=%v", next, cont)
	}
}

func TestBuildAutonomyLoopFollowUpInput(t *testing.T) {
	state := autonomyLoopState{
		Goal:             "修复测试失败",
		MaxRounds:        2,
		Round:            1,
		Phase:            autonomyLoopPhaseContinue,
		LastActionStatus: "partial",
		StopReason:       "status_partial",
	}
	text := buildAutonomyLoopFollowUpInput(state, "第1步失败：exit status 1")
	if !strings.Contains(text, "[ClawX Autonomous Loop]") {
		t.Fatalf("missing autonomy loop header: %s", text)
	}
	if !strings.Contains(text, "目标：修复测试失败") {
		t.Fatalf("missing goal context: %s", text)
	}
	if !strings.Contains(text, "loop_phase=continue") {
		t.Fatalf("missing loop phase context: %s", text)
	}
	if !strings.Contains(text, "[Execution Feedback]") {
		t.Fatalf("missing execution feedback section: %s", text)
	}
	if !strings.Contains(text, "progress_report") {
		t.Fatalf("missing progress report contract hint: %s", text)
	}
	if !strings.Contains(text, "remaining_steps 必须至少 1 项") {
		t.Fatalf("missing strict remaining_steps requirement: %s", text)
	}
}

func TestRenderAutonomyProgressNarrative(t *testing.T) {
	done := renderAutonomyProgressNarrative(autonomyProgressReport{
		Type:     "progress_report",
		Done:     true,
		Evidence: []string{"go test ./... 通过"},
	})
	if !strings.Contains(done, "进展更新：") || !strings.Contains(done, "完成状态：已完成") || !strings.Contains(done, "摘要：当前目标已完成") || !strings.Contains(done, "关键证据：") {
		t.Fatalf("unexpected done progress narrative: %s", done)
	}

	pending := renderAutonomyProgressNarrative(autonomyProgressReport{
		Type:           "progress_report",
		Done:           false,
		Summary:        "已完成环境准备，待回归",
		RemainingSteps: []string{"执行回归测试"},
		NextAction:     "运行 go test ./...",
	})
	if !strings.Contains(pending, "摘要：已完成环境准备，待回归") || !strings.Contains(pending, "剩余步骤") || !strings.Contains(pending, "下一步") {
		t.Fatalf("unexpected pending progress narrative: %s", pending)
	}
}

func TestRenderAutonomyProgressNarrativeDefaultNextAction(t *testing.T) {
	pending := renderAutonomyProgressNarrative(autonomyProgressReport{
		Type:           "progress_report",
		Done:           false,
		RemainingSteps: []string{"补充测试覆盖"},
	})
	if !strings.Contains(pending, "下一步：按剩余步骤继续推进并补充关键证据。") {
		t.Fatalf("expected default next-action guidance, got: %s", pending)
	}
}

func TestResolveAutonomyLoopMaxRounds(t *testing.T) {
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_ROUNDS", "")
	if got := resolveAutonomyLoopMaxRounds(); got != 6 {
		t.Fatalf("expected default=6, got %d", got)
	}
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_ROUNDS", "0")
	if got := resolveAutonomyLoopMaxRounds(); got != 1 {
		t.Fatalf("expected floor=1, got %d", got)
	}
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_ROUNDS", "9")
	if got := resolveAutonomyLoopMaxRounds(); got != 9 {
		t.Fatalf("expected explicit rounds=9 within cap, got %d", got)
	}
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_ROUNDS", "99")
	if got := resolveAutonomyLoopMaxRounds(); got != 12 {
		t.Fatalf("expected cap=12, got %d", got)
	}
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_ROUNDS", "3")
	if got := resolveAutonomyLoopMaxRounds(); got != 3 {
		t.Fatalf("expected explicit rounds=3, got %d", got)
	}
}

func TestResolveAutonomyLoopMaxSegments(t *testing.T) {
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_SEGMENTS", "")
	if got := resolveAutonomyLoopMaxSegments(); got != 0 {
		t.Fatalf("expected default=0 (unlimited), got %d", got)
	}
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_SEGMENTS", "0")
	if got := resolveAutonomyLoopMaxSegments(); got != 0 {
		t.Fatalf("expected explicit 0 to keep unlimited, got %d", got)
	}
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_SEGMENTS", "-3")
	if got := resolveAutonomyLoopMaxSegments(); got != 0 {
		t.Fatalf("expected negative value to fallback unlimited, got %d", got)
	}
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_SEGMENTS", "5")
	if got := resolveAutonomyLoopMaxSegments(); got != 5 {
		t.Fatalf("expected explicit segments=5 within cap, got %d", got)
	}
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_SEGMENTS", "99")
	if got := resolveAutonomyLoopMaxSegments(); got != 99 {
		t.Fatalf("expected explicit segments=99 within cap, got %d", got)
	}
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_SEGMENTS", "9999")
	if got := resolveAutonomyLoopMaxSegments(); got != 256 {
		t.Fatalf("expected cap=256, got %d", got)
	}
}

func prepareExecutionGoalStoreForTest(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	globalExecutionGoalStore = &executionGoalStore{
		byConv: map[string]executionGoalState{},
	}
}

func TestApplyAutonomousActionPlanLoopSyncsExecutionGoalStateCompleted(t *testing.T) {
	prepareExecutionGoalStoreForTest(t)
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_ROUNDS", "2")

	runtime := agentRuntime{agentID: "bid-all"}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-goal-sync-completed",
		Message: chatiface.Message{
			Text: "继续推进 bid-all 发布验收",
		},
	}
	raw := `{"type":"progress_report","goal":"完成 bid-all 发布验收","done":true,"done_criteria":["健康检查通过"],"evidence":["/healthz 返回 200"],"summary":"发布验收已完成。","next_action":"通知用户验收结果"}`
	_, _ = applyAutonomousActionPlanLoop(
		context.Background(),
		runtime,
		decision,
		raw,
		"discord",
		"default",
		"scope:discord:default:conv-goal-sync-completed",
		newConversationAgentOverrides(),
		map[string]agentRuntime{"bid-all": runtime},
		"bid-all",
		".",
		command.SessionCommand{},
	)

	state, ok := getExecutionGoalState(decision.ConversationID)
	if !ok {
		t.Fatalf("expected execution goal state persisted")
	}
	if state.Status != "completed" {
		t.Fatalf("expected completed status, got: %s", state.Status)
	}
	if state.Goal != "完成 bid-all 发布验收" {
		t.Fatalf("unexpected goal: %s", state.Goal)
	}
	if state.AgentID != "bid-all" {
		t.Fatalf("unexpected agent id: %s", state.AgentID)
	}
	if len(state.DoneCriteria) != 1 || state.DoneCriteria[0] != "健康检查通过" {
		t.Fatalf("unexpected done criteria: %+v", state.DoneCriteria)
	}
	if len(state.Evidence) != 1 || state.Evidence[0] != "/healthz 返回 200" {
		t.Fatalf("unexpected evidence: %+v", state.Evidence)
	}
	if len(state.RemainingSteps) != 0 {
		t.Fatalf("expected empty remaining steps, got: %+v", state.RemainingSteps)
	}
	if strings.TrimSpace(state.NextAction) != "" {
		t.Fatalf("expected empty next action for completed state, got: %q", state.NextAction)
	}
}

func TestApplyAutonomousActionPlanLoopSyncsExecutionGoalStateBlockedOnRoundLimit(t *testing.T) {
	prepareExecutionGoalStoreForTest(t)
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_ROUNDS", "1")

	runtime := agentRuntime{agentID: "bid-all"}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-goal-sync-blocked",
		Message: chatiface.Message{
			Text: "继续推进 bid-all 发布验收",
		},
	}
	raw := `{"type":"progress_report","goal":"完成 bid-all 发布验收","done":false,"remaining_steps":["回归测试","验收冒烟"],"summary":"还剩关键步骤。","next_action":"执行 go test ./..."}`
	out, _ := applyAutonomousActionPlanLoop(
		context.Background(),
		runtime,
		decision,
		raw,
		"discord",
		"default",
		"scope:discord:default:conv-goal-sync-blocked",
		newConversationAgentOverrides(),
		map[string]agentRuntime{"bid-all": runtime},
		"bid-all",
		".",
		command.SessionCommand{},
	)
	if !strings.Contains(out, "自治暂停：已达到连续执行轮次上限（第 1/1 轮）。") {
		t.Fatalf("expected round-limit pause summary in output, got: %s", out)
	}
	if !strings.Contains(out, "结论：已触发轮次上限保护，等待你确认后继续。") {
		t.Fatalf("expected readable conclusion in round-limit notice, got: %s", out)
	}
	if !strings.Contains(out, "完成状态：已暂停（等待确认）。") {
		t.Fatalf("expected status line in round-limit notice, got: %s", out)
	}
	if !strings.Contains(out, "当前阶段：触发轮次上限保护，已暂停等待确认。") {
		t.Fatalf("expected stage line in round-limit notice, got: %s", out)
	}
	if !strings.Contains(out, "下一步：直接发送下一条消息（如“继续”）即可从当前进度续跑") {
		t.Fatalf("expected clear continue guidance in output, got: %s", out)
	}
	if !strings.Contains(out, "剩余步骤：回归测试；验收冒烟") {
		t.Fatalf("expected remaining steps in pause summary, got: %s", out)
	}

	state, ok := getExecutionGoalState(decision.ConversationID)
	if !ok {
		t.Fatalf("expected execution goal state persisted")
	}
	if state.Status != "blocked" {
		t.Fatalf("expected blocked status, got: %s", state.Status)
	}
	if state.StopReason != "round_limit_reached_with_remaining_steps" {
		t.Fatalf("unexpected stop reason: %s", state.StopReason)
	}
	if len(state.RemainingSteps) == 0 || state.RemainingSteps[0] != "回归测试" {
		t.Fatalf("unexpected remaining steps: %+v", state.RemainingSteps)
	}
	if !strings.Contains(state.NextAction, "go test ./...") {
		t.Fatalf("unexpected next action: %s", state.NextAction)
	}
}

func TestFormatAutonomyFollowUpFailureNotice(t *testing.T) {
	prepareExecutionGoalStoreForTest(t)
	setExecutionGoalState("conv-followup-failed", executionGoalState{
		Goal:           "推进 bid-all 发布",
		Status:         "running",
		RemainingSteps: []string{"执行回归测试", "检查健康探针"},
		NextAction:     "先执行 go test ./...",
	})
	msg := formatAutonomyFollowUpFailureNotice("conv-followup-failed", autonomyLoopState{
		Goal:      "推进 bid-all 发布",
		Round:     2,
		MaxRounds: 3,
	}, errors.New("backend timeout while running follow-up action"))
	if !strings.Contains(msg, "处理失败：续跑时遇到错误，已暂停等待你确认。") {
		t.Fatalf("expected failure receipt in notice, got: %s", msg)
	}
	if !strings.Contains(msg, "结论：续跑执行失败，需先处理错误再继续。") {
		t.Fatalf("expected readable conclusion in failure notice, got: %s", msg)
	}
	if !strings.Contains(msg, "完成状态：失败（已暂停）。") {
		t.Fatalf("expected status line in failure notice, got: %s", msg)
	}
	if !strings.Contains(msg, "失败位置：第 2/3 轮续跑。") {
		t.Fatalf("expected loop position in notice, got: %s", msg)
	}
	if !strings.Contains(msg, "当前阶段：续跑执行失败，已暂停等待确认。") {
		t.Fatalf("expected stage line in failure notice, got: %s", msg)
	}
	if !strings.Contains(msg, "失败摘要：backend timeout while running follow-up action") {
		t.Fatalf("expected error summary in notice, got: %s", msg)
	}
	if !strings.Contains(msg, "当前目标：推进 bid-all 发布") || !strings.Contains(msg, "剩余步骤：执行回归测试；检查健康探针") {
		t.Fatalf("expected goal and remaining steps in notice, got: %s", msg)
	}
	if !strings.Contains(msg, "下一步动作：先执行 go test ./...") {
		t.Fatalf("expected normalized next-action label in notice, got: %s", msg)
	}
	if !strings.Contains(msg, "下一步：直接发送下一条消息（如“继续”）即可按当前进度重试") {
		t.Fatalf("expected continue guidance in notice, got: %s", msg)
	}
}

func TestFormatAutonomyRoundLimitNoticeIncludesExecutionSource(t *testing.T) {
	prepareExecutionGoalStoreForTest(t)
	conversationID := "conv-round-limit-source"
	setExecutionGoalState(conversationID, executionGoalState{
		Goal:   "推进 bid-all 发布",
		Status: "running",
	})
	_ = appendRuntimeExecAttestation(runtimeExecAttestationRecord{
		ExecID:                            newRuntimeExecID(),
		ConversationID:                    conversationID,
		AgentID:                           "bid-all",
		Command:                           "pip install -r requirements.txt",
		Success:                           false,
		RuntimeExecDecisionMode:           "build.continue",
		RuntimeExecDecisionApplySource:    "persisted_state",
		RuntimeExecDecisionLockSource:     "user_phrase",
		RuntimeExecDecisionFallbackSource: "workspace",
	})

	msg := formatAutonomyRoundLimitNotice(conversationID, autonomyLoopState{
		Goal:      "推进 bid-all 发布",
		Round:     3,
		MaxRounds: 3,
	})
	if !strings.Contains(msg, "执行来源：mode=build.continue apply_source=persisted_state lock_source=user_phrase fallback_source=workspace") {
		t.Fatalf("expected execution source in round-limit notice, got: %s", msg)
	}
}

func TestFormatAutonomyFollowUpFailureNoticeIncludesExecutionSource(t *testing.T) {
	prepareExecutionGoalStoreForTest(t)
	conversationID := "conv-followup-failed-source"
	setExecutionGoalState(conversationID, executionGoalState{
		Goal:   "推进 bid-all 发布",
		Status: "running",
	})
	_ = appendRuntimeExecAttestation(runtimeExecAttestationRecord{
		ExecID:                            newRuntimeExecID(),
		ConversationID:                    conversationID,
		AgentID:                           "bid-all",
		Command:                           "curl -sS -X POST http://127.0.0.1:8000/api/sources",
		Success:                           false,
		RuntimeExecDecisionMode:           "service.deep_repair",
		RuntimeExecDecisionApplySource:    "persisted_state",
		RuntimeExecDecisionLockSource:     "user_phrase",
		RuntimeExecDecisionFallbackSource: "env",
	})

	msg := formatAutonomyFollowUpFailureNotice(conversationID, autonomyLoopState{
		Goal:      "推进 bid-all 发布",
		Round:     2,
		MaxRounds: 3,
	}, errors.New("backend timeout while running follow-up action"))
	if !strings.Contains(msg, "执行来源：mode=service.deep_repair apply_source=persisted_state lock_source=user_phrase fallback_source=env") {
		t.Fatalf("expected execution source in follow-up failure notice, got: %s", msg)
	}
}

func TestApplyAutonomousActionPlanLoopRendersProgressOnlyOutput(t *testing.T) {
	runtime := agentRuntime{agentID: "main"}
	decision := service.Decision{
		Kind: service.DecisionExecute,
		Message: chatiface.Message{
			Text: "修复测试",
		},
	}
	raw := `{"type":"progress_report","goal":"修复测试","done":true,"evidence":["go test ./... 通过"]}`
	out, responseAgentID := applyAutonomousActionPlanLoop(
		context.Background(),
		runtime,
		decision,
		raw,
		"discord",
		"default",
		"scope:discord:default:conv",
		newConversationAgentOverrides(),
		map[string]agentRuntime{"main": runtime},
		"main",
		".",
		command.SessionCommand{},
	)
	if strings.Contains(out, `"type":"progress_report"`) {
		t.Fatalf("expected structured payload stripped from final output: %s", out)
	}
	if !strings.Contains(out, "当前目标已完成") || !strings.Contains(out, "关键证据") {
		t.Fatalf("expected readable progress summary for user, got: %s", out)
	}
	if strings.TrimSpace(responseAgentID) != "main" {
		t.Fatalf("expected response agent id main, got: %s", responseAgentID)
	}
}

func TestApplyAutonomousActionPlanLoopWithRouterFollowUp(t *testing.T) {
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_ROUNDS", "3")

	backend := &scriptedLoopBackend{
		name: "scripted-loop",
		outputs: []string{
			`{"type":"progress_report","goal":"修复测试","done":true,"summary":"回归测试完成。","evidence":["go test ./... 通过"]}`,
		},
	}
	store := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(store, store, nil)
	router := service.NewRouter(config.Snapshot{
		AllowedRoots: []string{"."},
		DefaultCWD:   ".",
		Timeout:      3 * time.Second,
	}, manager, backend)

	runtime := agentRuntime{
		agentID: "main",
		router:  router,
	}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-loop-router",
		ProjectID:      "main",
		RouteKey:       "discord:default:conv-loop-router",
		Message: chatiface.Message{
			Text:           "修复测试",
			ConversationID: "conv-loop-router",
			UserID:         "u1",
		},
	}

	initial := "先执行计划。\n```json\n{\"type\":\"progress_report\",\"goal\":\"修复测试\",\"done\":false,\"remaining_steps\":[\"执行回归测试\"]}\n```\n```json\n{\"type\":\"action_plan\",\"mode\":\"execute\",\"actions\":[{\"kind\":\"agent.use\",\"agent_id\":\"main\"}]}\n```"
	out, responseAgentID := applyAutonomousActionPlanLoop(
		context.Background(),
		runtime,
		decision,
		initial,
		"discord",
		"default",
		"scope:discord:default:conv-loop-router",
		newConversationAgentOverrides(),
		map[string]agentRuntime{"main": runtime},
		"main",
		".",
		command.SessionCommand{
			Mode:           command.ModeContinue,
			ConversationID: decision.ConversationID,
			WindowID:       "discord:default:conv-loop-router",
			ProjectID:      decision.ProjectID,
			RouteKey:       decision.RouteKey,
			UserID:         decision.Message.UserID,
			Input:          "placeholder",
			Backend:        "scripted-loop",
			CWD:            ".",
		},
	)

	if strings.Contains(out, `"type":"progress_report"`) {
		t.Fatalf("expected final output without progress_report payload, got: %s", out)
	}
	if !strings.Contains(out, "回归测试完成。") || !strings.Contains(out, "关键证据") {
		t.Fatalf("expected readable progress summary from follow-up output, got: %s", out)
	}
	if strings.TrimSpace(responseAgentID) != "main" {
		t.Fatalf("expected response agent id main, got: %s", responseAgentID)
	}

	requests := backend.Requests()
	if len(requests) != 1 {
		t.Fatalf("expected exactly one follow-up backend call, got %d", len(requests))
	}
	if !strings.Contains(requests[0].Input, "[ClawX Autonomous Loop]") || !strings.Contains(requests[0].Input, "loop_round=1/3") {
		t.Fatalf("expected autonomy loop context in follow-up input, got: %s", requests[0].Input)
	}
}

func TestApplyAutonomousActionPlanLoopAutoSegmentsOnRoundLimit(t *testing.T) {
	prepareExecutionGoalStoreForTest(t)
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_ROUNDS", "1")
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_SEGMENTS", "2")

	backend := &scriptedLoopBackend{
		name: "scripted-loop",
		outputs: []string{
			`{"type":"progress_report","goal":"修复测试","done":true,"summary":"跨段续跑完成。","evidence":["go test ./... 通过"]}`,
		},
	}
	store := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(store, store, nil)
	router := service.NewRouter(config.Snapshot{
		AllowedRoots: []string{"."},
		DefaultCWD:   ".",
		Timeout:      3 * time.Second,
	}, manager, backend)

	runtime := agentRuntime{
		agentID: "main",
		router:  router,
	}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-loop-segment-auto",
		ProjectID:      "main",
		RouteKey:       "discord:default:conv-loop-segment-auto",
		Message: chatiface.Message{
			Text:           "修复测试",
			ConversationID: "conv-loop-segment-auto",
			UserID:         "u1",
		},
	}

	initial := "先执行计划。\n```json\n{\"type\":\"progress_report\",\"goal\":\"修复测试\",\"done\":false,\"remaining_steps\":[\"执行回归测试\"]}\n```\n```json\n{\"type\":\"action_plan\",\"mode\":\"execute\",\"actions\":[{\"kind\":\"agent.use\",\"agent_id\":\"main\"}]}\n```"
	out, responseAgentID := applyAutonomousActionPlanLoop(
		context.Background(),
		runtime,
		decision,
		initial,
		"discord",
		"default",
		"scope:discord:default:conv-loop-segment-auto",
		newConversationAgentOverrides(),
		map[string]agentRuntime{"main": runtime},
		"main",
		".",
		command.SessionCommand{
			Mode:           command.ModeContinue,
			ConversationID: decision.ConversationID,
			WindowID:       "discord:default:conv-loop-segment-auto",
			ProjectID:      decision.ProjectID,
			RouteKey:       decision.RouteKey,
			UserID:         decision.Message.UserID,
			Input:          "placeholder",
			Backend:        "scripted-loop",
			CWD:            ".",
		},
	)

	if strings.Contains(out, "自治暂停：已达到连续执行轮次上限") {
		t.Fatalf("expected auto segment rollover instead of round-limit pause, got: %s", out)
	}
	if !strings.Contains(out, "跨段续跑完成") || !strings.Contains(out, "关键证据") {
		t.Fatalf("expected readable completion output after auto segment rollover, got: %s", out)
	}
	if strings.TrimSpace(responseAgentID) != "main" {
		t.Fatalf("expected response agent id main, got: %s", responseAgentID)
	}

	requests := backend.Requests()
	if len(requests) != 1 {
		t.Fatalf("expected one rollover follow-up backend call, got %d", len(requests))
	}
	if !strings.Contains(requests[0].Input, "[ClawX Autonomous Segment Rollover]") {
		t.Fatalf("expected segment rollover marker in follow-up input, got: %s", requests[0].Input)
	}
	if !strings.Contains(requests[0].Input, "loop_segment=2/2") {
		t.Fatalf("expected segment index in follow-up input, got: %s", requests[0].Input)
	}

	state, ok := getExecutionGoalState(decision.ConversationID)
	if !ok {
		t.Fatalf("expected execution goal state persisted")
	}
	if state.Status != "completed" {
		t.Fatalf("expected completed status after rollover, got: %s", state.Status)
	}
}

func TestApplyAutonomousActionPlanLoopAutoSegmentsUnlimitedByDefault(t *testing.T) {
	prepareExecutionGoalStoreForTest(t)
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_ROUNDS", "1")
	t.Setenv("CLAWX_AUTONOMY_LOOP_MAX_SEGMENTS", "")

	backend := &scriptedLoopBackend{
		name: "scripted-loop",
		outputs: []string{
			`{"type":"progress_report","goal":"修复测试","done":true,"summary":"默认无限分段续跑完成。","evidence":["go test ./... 通过"]}`,
		},
	}
	store := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(store, store, nil)
	router := service.NewRouter(config.Snapshot{
		AllowedRoots: []string{"."},
		DefaultCWD:   ".",
		Timeout:      3 * time.Second,
	}, manager, backend)

	runtime := agentRuntime{
		agentID: "main",
		router:  router,
	}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-loop-segment-unlimited-default",
		ProjectID:      "main",
		RouteKey:       "discord:default:conv-loop-segment-unlimited-default",
		Message: chatiface.Message{
			Text:           "修复测试",
			ConversationID: "conv-loop-segment-unlimited-default",
			UserID:         "u1",
		},
	}

	initial := "先执行计划。\n```json\n{\"type\":\"progress_report\",\"goal\":\"修复测试\",\"done\":false,\"remaining_steps\":[\"执行回归测试\"]}\n```\n```json\n{\"type\":\"action_plan\",\"mode\":\"execute\",\"actions\":[{\"kind\":\"agent.use\",\"agent_id\":\"main\"}]}\n```"
	out, _ := applyAutonomousActionPlanLoop(
		context.Background(),
		runtime,
		decision,
		initial,
		"discord",
		"default",
		"scope:discord:default:conv-loop-segment-unlimited-default",
		newConversationAgentOverrides(),
		map[string]agentRuntime{"main": runtime},
		"main",
		".",
		command.SessionCommand{
			Mode:           command.ModeContinue,
			ConversationID: decision.ConversationID,
			WindowID:       "discord:default:conv-loop-segment-unlimited-default",
			ProjectID:      decision.ProjectID,
			RouteKey:       decision.RouteKey,
			UserID:         decision.Message.UserID,
			Input:          "placeholder",
			Backend:        "scripted-loop",
			CWD:            ".",
		},
	)
	if strings.Contains(out, "自治暂停：已达到连续执行轮次上限") {
		t.Fatalf("expected unlimited auto segment rollover without round-limit pause, got: %s", out)
	}
	if !strings.Contains(out, "默认无限分段续跑完成") {
		t.Fatalf("expected readable completion output after unlimited rollover, got: %s", out)
	}

	requests := backend.Requests()
	if len(requests) != 1 {
		t.Fatalf("expected one rollover follow-up backend call, got %d", len(requests))
	}
	if !strings.Contains(requests[0].Input, "loop_segment=2/unlimited") {
		t.Fatalf("expected unlimited segment label in follow-up input, got: %s", requests[0].Input)
	}
}

type scriptedLoopBackend struct {
	name    string
	outputs []string

	mu       sync.Mutex
	requests []execution.Request
}

func (b *scriptedLoopBackend) Name() string {
	if strings.TrimSpace(b.name) == "" {
		return "scripted-loop"
	}
	return b.name
}

func (b *scriptedLoopBackend) Execute(_ context.Context, request execution.Request) (execution.Result, error) {
	b.mu.Lock()
	b.requests = append(b.requests, request)
	idx := len(b.requests) - 1
	output := `{"type":"progress_report","done":true,"summary":"完成。"}`
	if idx < len(b.outputs) && strings.TrimSpace(b.outputs[idx]) != "" {
		output = b.outputs[idx]
	}
	b.mu.Unlock()

	now := time.Now().UTC()
	return execution.Result{
		BackendSessionID: fmt.Sprintf("backend-%s-%d", request.SessionID, idx+1),
		Output:           output,
		State:            execution.ResultSuccess,
		StartedAt:        now,
		CompletedAt:      now,
	}, nil
}

func (b *scriptedLoopBackend) Cancel(_ context.Context, _ string) error {
	return nil
}

func (b *scriptedLoopBackend) HealthCheck(_ context.Context) error {
	return nil
}

func (b *scriptedLoopBackend) Requests() []execution.Request {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]execution.Request, len(b.requests))
	copy(out, b.requests)
	return out
}
