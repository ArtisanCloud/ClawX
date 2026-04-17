package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clawx/internal/application/service"
	"clawx/internal/infrastructure/config"
	chatiface "clawx/internal/interfaces/chat"
)

func TestBuildExecutionInputIncludesAttachmentsForDirectExecution(t *testing.T) {
	decision := service.Decision{
		Kind: service.DecisionExecute,
		Message: chatiface.Message{
			Text: "转换成长图",
			Attachments: []chatiface.Attachment{
				{
					Name:        "test.pdf",
					URL:         "https://cdn.discordapp.com/test.pdf",
					LocalPath:   "/home/ubuntu/.clawx/workspaces/image_tools/.agents/main/context/attachments/files/abc/test.pdf",
					ContentType: "application/pdf",
					SizeBytes:   9195520,
				},
			},
		},
	}

	input := buildExecutionInput(decision, agentRuntime{})
	if !strings.Contains(input, "[Attachments]") {
		t.Fatalf("expected attachments section, got: %q", input)
	}
	if !strings.Contains(input, "test.pdf") {
		t.Fatalf("expected attachment name in input, got: %q", input)
	}
	if !strings.Contains(input, "application/pdf") {
		t.Fatalf("expected content type in input, got: %q", input)
	}
	if !strings.Contains(input, "local_path=") {
		t.Fatalf("expected local path in input, got: %q", input)
	}
}

func TestBuildExecutionInputInjectsAgentInventoryContext(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stateDir := filepath.Join(home, ".clawx")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	configBody := `{
  "providers": {
    "profiles": {
      "codex": {
        "kind": "codex-cli",
        "command": "codex"
      }
    }
  },
  "agents": {
    "default": "main",
    "list": [
      {"id":"main","profile":"codex","workspace":"/home/ubuntu/.clawx/workspaces/main","timeoutSeconds":600,"default":true},
      {"id":"local-smoke","profile":"codex","workspace":"/home/ubuntu/.clawx/workspaces/local-smoke","timeoutSeconds":600},
      {"id":"bid-all智能体","profile":"codex","workspace":"/home/ubuntu/.clawx/workspaces/bid-all智能体","timeoutSeconds":600}
    ]
  }
}`
	if err := os.WriteFile(filepath.Join(stateDir, "config.json"), []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	decision := service.Decision{
		Kind: service.DecisionExecute,
		Message: chatiface.Message{
			Text: "现在我们有多少个智能体？",
		},
	}
	input := buildExecutionInput(decision, agentRuntime{agentID: "main"})
	if !strings.Contains(input, "tool.agent_inventory") {
		t.Fatalf("expected agent inventory tool context, got: %q", input)
	}
	if !strings.Contains(input, "total_registered_agents=3") {
		t.Fatalf("expected total agent count in context, got: %q", input)
	}
	if !strings.Contains(input, "id=bid-all智能体") {
		t.Fatalf("expected bid-all智能体 listed in context, got: %q", input)
	}
	if !strings.Contains(input, "[Staged Routing Snapshot]") {
		t.Fatalf("expected staged routing snapshot in input, got: %q", input)
	}
	if !strings.Contains(input, "route.use_route_planner=") {
		t.Fatalf("expected staged routing plan fields in input, got: %q", input)
	}
	if !strings.Contains(input, "fallback.enabled=") {
		t.Fatalf("expected staged fallback fields in input, got: %q", input)
	}
	if !strings.Contains(input, "execution.can_execute=") {
		t.Fatalf("expected staged execution gate field in input, got: %q", input)
	}
	if !strings.Contains(input, "[Autonomous Recovery Playbook]") {
		t.Fatalf("expected autonomous recovery playbook in input, got: %q", input)
	}
	if !strings.Contains(input, "禁止第一轮直接向用户索要环境参数/镜像/离线包") {
		t.Fatalf("expected autonomy-first recovery rule in input, got: %q", input)
	}
	if !strings.Contains(input, "[Execution Blocker Contract]") {
		t.Fatalf("expected execution blocker contract in input, got: %q", input)
	}
	if !strings.Contains(input, "\"type\": \"execution_blocker\"") {
		t.Fatalf("expected execution blocker schema in input, got: %q", input)
	}
	if !strings.Contains(input, "\"evidence_exec_ids\"") {
		t.Fatalf("expected execution blocker attestation ids in input, got: %q", input)
	}
	if !strings.Contains(input, "[Unified Action Plan Contract]") {
		t.Fatalf("expected unified action plan contract in input, got: %q", input)
	}
	if !strings.Contains(input, "\"type\": \"action_plan\"") {
		t.Fatalf("expected action plan schema in input, got: %q", input)
	}
	if !strings.Contains(input, "runtime.task.delegate") {
		t.Fatalf("expected runtime.task.delegate in action kind contract, got: %q", input)
	}
	if !strings.Contains(input, "runtime.task.delegates") {
		t.Fatalf("expected runtime.task.delegates in action kind contract, got: %q", input)
	}
	if !strings.Contains(input, "runtime.task.retry") {
		t.Fatalf("expected runtime.task.retry in action kind contract, got: %q", input)
	}
	if !strings.Contains(input, "runtime.task.cancel") {
		t.Fatalf("expected runtime.task.cancel in action kind contract, got: %q", input)
	}
	if !strings.Contains(input, "旧版 control_plan / requirement_sync / runtime_exec_plan 已废弃") {
		t.Fatalf("expected deprecated protocol warning in input, got: %q", input)
	}
	if !strings.Contains(input, "[Progress Report Contract]") {
		t.Fatalf("expected progress report contract in input, got: %q", input)
	}
	if !strings.Contains(input, "\"type\": \"progress_report\"") {
		t.Fatalf("expected progress report schema in input, got: %q", input)
	}
}

func TestBuildExecutionInputIncludesContinuationSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".clawx"), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}

	convID := "discord:-:c1:u1|ch=discord|inst=default|agent=bid-all"
	if err := appendRuntimeExecAttestation(runtimeExecAttestationRecord{
		ExecID:         "rexec-test-1",
		ConversationID: convID,
		AgentID:        "bid-all",
		CWD:            "/home/ubuntu/.clawx/workspaces/bid-all",
		Command:        "python3.11 -m venv .venv311",
		Success:        true,
		OutputPreview:  "ok",
	}); err != nil {
		t.Fatalf("append attestation #1: %v", err)
	}
	if err := appendRuntimeExecAttestation(runtimeExecAttestationRecord{
		ExecID:         "rexec-test-2",
		ConversationID: convID,
		AgentID:        "bid-all",
		CWD:            "/home/ubuntu/.clawx/workspaces/bid-all",
		Command:        "source .venv311/bin/activate && pip install -e . --no-build-isolation",
		Success:        false,
		OutputPreview:  "build backend failed",
		ErrorSummary:   "exit status 1",
	}); err != nil {
		t.Fatalf("append attestation #2: %v", err)
	}

	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: convID,
		Message: chatiface.Message{
			Text: "继续",
		},
	}
	input := buildExecutionInput(decision, agentRuntime{agentID: "bid-all"})
	if !strings.Contains(input, "[Execution Continuation Snapshot]") {
		t.Fatalf("expected continuation snapshot, got: %q", input)
	}
	if !strings.Contains(input, "continuation_hint=advance_from_recent_runtime_exec") {
		t.Fatalf("expected continuation hint, got: %q", input)
	}
	if !strings.Contains(input, "不要重复最近已成功命令") {
		t.Fatalf("expected continuation rule in input, got: %q", input)
	}
}

func TestBuildExecutionInputContinuationSnapshotIncludesDecisionContext(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".clawx"), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}

	convID := "discord:-:c2:u2|ch=discord|inst=default|agent=bid-all"
	if err := appendRuntimeExecAttestation(runtimeExecAttestationRecord{
		ExecID:                            "rexec-test-decision-ctx",
		ConversationID:                    convID,
		AgentID:                           "bid-all",
		CWD:                               "/home/ubuntu/.clawx/workspaces/bid-all",
		Command:                           "echo build-continue",
		Success:                           true,
		OutputPreview:                     "ok",
		RuntimeExecDecisionMode:           "build.continue",
		RuntimeExecDecisionApplySource:    "persisted_state",
		RuntimeExecDecisionLockSource:     "user_phrase",
		RuntimeExecDecisionFallbackSource: "workspace",
	}); err != nil {
		t.Fatalf("append attestation with decision context: %v", err)
	}

	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: convID,
		Message: chatiface.Message{
			Text: "继续",
		},
	}
	input := buildExecutionInput(decision, agentRuntime{agentID: "bid-all"})
	if !strings.Contains(input, "decision_mode=build.continue") {
		t.Fatalf("expected decision_mode in continuation snapshot, got: %q", input)
	}
	if !strings.Contains(input, "decision_apply_source=persisted_state") {
		t.Fatalf("expected decision_apply_source in continuation snapshot, got: %q", input)
	}
	if !strings.Contains(input, "decision_lock_source=user_phrase") {
		t.Fatalf("expected decision_lock_source in continuation snapshot, got: %q", input)
	}
	if !strings.Contains(input, "decision_fallback_source=workspace") {
		t.Fatalf("expected decision_fallback_source in continuation snapshot, got: %q", input)
	}
}

func TestBuildExecutionInputIncludesResumeTaskSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	globalExecutionGoalStore = &executionGoalStore{
		byConv: map[string]executionGoalState{},
	}

	convID := "discord:-:resume:u1|ch=discord|inst=default|agent=bid-all"
	setExecutionGoalState(convID, executionGoalState{
		TaskID:         "task-demo-001",
		AgentID:        "bid-all",
		Goal:           "完成 bid-all 发布",
		Status:         "running",
		RemainingSteps: []string{"执行健康检查", "回归测试"},
		NextAction:     "重启服务并验收 /healthz",
		LastResult:     "发布脚本执行成功",
	})

	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: convID,
		Message: chatiface.Message{
			Text: "继续任务 task-demo-001",
		},
	}
	input := buildExecutionInput(decision, agentRuntime{agentID: "bid-all"})
	if !strings.Contains(input, "[Resume Task Snapshot]") {
		t.Fatalf("expected resume snapshot in input, got: %q", input)
	}
	if !strings.Contains(input, "resume_task_id=task-demo-001") {
		t.Fatalf("expected resume task id, got: %q", input)
	}
	if !strings.Contains(input, "resume_next_action=重启服务并验收 /healthz") {
		t.Fatalf("expected resume next action, got: %q", input)
	}
}

func TestBuildExecutionInputIncludesTaskControlHintSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	globalExecutionGoalStore = &executionGoalStore{
		byConv: map[string]executionGoalState{},
	}

	workspace := filepath.Join(home, ".clawx", "workspaces", "bid-all")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	now := time.Now().UTC()
	if err := ensureTaskTrackingScaffold(workspace, now); err != nil {
		t.Fatalf("ensure task tracking scaffold: %v", err)
	}
	if err := updateWorkspaceTaskState(workspace, now, map[string]interface{}{
		"conversation_id":                     "conv-task-hint",
		"health_service":                      "clawx-bid-all",
		"health_url":                          "http://127.0.0.1:19080/healthz",
		"last_task_control_operation":         "ensure_running",
		"last_task_control_service":           "clawx-bid-all",
		"last_task_control_health_url":        "http://127.0.0.1:19080/healthz",
		"last_task_control_at":                now.Format(time.RFC3339),
		"last_task_control_self_heal_service": "clawx-bid-all",
		"last_task_control_self_heal_state":   "failed",
		"last_task_control_self_heal_reason":  "health_down",
		"last_task_control_self_heal_at":      now.Format(time.RFC3339),
		"service_health_urls": map[string]string{
			"clawx-bid-all": "http://127.0.0.1:19080/healthz",
		},
		"service_self_heal_alert_levels":      map[string]string{"clawx-bid-all": "warning"},
		"service_self_heal_alert_states":      map[string]string{"clawx-bid-all": "failed"},
		"service_self_heal_alert_reasons":     map[string]string{"clawx-bid-all": "health_down"},
		"service_self_heal_alert_notified_at": map[string]string{"clawx-bid-all": now.Format(time.RFC3339)},
	}); err != nil {
		t.Fatalf("update workspace task state: %v", err)
	}

	runtime := agentRuntime{
		agentID: "bid-all",
		cwd:     workspace,
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{workspace},
			DefaultCWD:   workspace,
			Agents: map[string]config.Agent{
				"bid-all": {
					ID:        "bid-all",
					Workspace: workspace,
				},
			},
		},
	}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-task-hint",
		Message: chatiface.Message{
			Text: "继续",
		},
	}
	input := buildExecutionInput(decision, runtime)
	if !strings.Contains(input, "task_control_hint_available=true") {
		t.Fatalf("expected task control hint availability in continuation snapshot, got: %q", input)
	}
	if !strings.Contains(input, "task_control_hint_service=clawx-bid-all") {
		t.Fatalf("expected task control hint service, got: %q", input)
	}
	if !strings.Contains(input, "task_control_hint_health_url=http://127.0.0.1:19080/healthz") {
		t.Fatalf("expected task control hint health url, got: %q", input)
	}
	if !strings.Contains(input, "task_control_hint_operation=ensure_running") {
		t.Fatalf("expected task control hint operation in continuation snapshot, got: %q", input)
	}
	if !strings.Contains(input, "task_control.last_action_available=true") {
		t.Fatalf("expected task control last action flag in staged routing snapshot, got: %q", input)
	}
	if !strings.Contains(input, "task_control.last_operation=ensure_running") {
		t.Fatalf("expected task control last operation in staged routing snapshot, got: %q", input)
	}
	if !strings.Contains(input, "task_control.last_service=clawx-bid-all") {
		t.Fatalf("expected task control last service in staged routing snapshot, got: %q", input)
	}
	if !strings.Contains(input, "task_control.last_health_url=http://127.0.0.1:19080/healthz") {
		t.Fatalf("expected task control last health url in staged routing snapshot, got: %q", input)
	}
	if !strings.Contains(input, "task_control.self_heal_available=true") {
		t.Fatalf("expected task control self-heal snapshot availability, got: %q", input)
	}
	if !strings.Contains(input, "task_control.self_heal_state=failed") {
		t.Fatalf("expected task control self-heal state in staged routing snapshot, got: %q", input)
	}
	if !strings.Contains(input, "task_control.self_heal_reason=health_down") {
		t.Fatalf("expected task control self-heal reason in staged routing snapshot, got: %q", input)
	}
	if !strings.Contains(input, "task_control.self_heal_alert_level=warning") {
		t.Fatalf("expected task control self-heal alert level in staged routing snapshot, got: %q", input)
	}
}

func TestBuildExecutionInputPrefersRouteScopedTaskControlHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	globalExecutionGoalStore = &executionGoalStore{
		byConv: map[string]executionGoalState{},
	}

	workspace := filepath.Join(home, ".clawx", "workspaces", "bid-all")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	now := time.Now().UTC()
	if err := ensureTaskTrackingScaffold(workspace, now); err != nil {
		t.Fatalf("ensure task tracking scaffold: %v", err)
	}

	globalURL := "http://127.0.0.1:19081/healthz"
	routeURL := "http://127.0.0.1:19082/healthz"
	convID := "discord:-:route-hint:u1|ch=discord|inst=default|agent=bid-all"
	routeScope := routingScopeKey("discord", "default", "discord:-:route-hint:u1")
	if err := updateWorkspaceTaskState(workspace, now, map[string]interface{}{
		"conversation_id":             "legacy-conv",
		"health_service":              "clawx-bid-all",
		"health_url":                  globalURL,
		"last_task_control_operation": "restart",
		"last_task_control_at":        now.Format(time.RFC3339),
		"service_health_urls": map[string]string{
			"clawx-bid-all": globalURL,
		},
		"task_control_route_hints": map[string]map[string]string{
			routeScope: {
				"service":         "clawx-bid-all",
				"health_url":      routeURL,
				"operation":       "ensure_running",
				"updated_at":      now.Format(time.RFC3339),
				"conversation_id": convID,
			},
		},
	}); err != nil {
		t.Fatalf("update workspace task state: %v", err)
	}

	runtime := agentRuntime{
		agentID: "bid-all",
		cwd:     workspace,
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{workspace},
			DefaultCWD:   workspace,
			Agents: map[string]config.Agent{
				"bid-all": {
					ID:        "bid-all",
					Workspace: workspace,
				},
			},
		},
	}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: convID,
		Message: chatiface.Message{
			Text: "继续",
		},
	}
	input := buildExecutionInput(decision, runtime)
	if !strings.Contains(input, "task_control_hint_health_url="+routeURL) {
		t.Fatalf("expected route-scoped task control hint url, got: %q", input)
	}
	if !strings.Contains(input, "task_control_hint_source=workspace_state_route_scope") {
		t.Fatalf("expected route-scoped task control hint source, got: %q", input)
	}
	if strings.Contains(input, "task_control_hint_health_url="+globalURL) {
		t.Fatalf("expected global hint not selected when route-scoped hint exists, got: %q", input)
	}
}

func TestBuildExecutionInputInjectsRuntimeExecDecisionSnapshot(t *testing.T) {
	testCases := []struct {
		name          string
		message       string
		expectedMode  string
		expectedExtra string
	}{
		{
			name:          "service deep repair",
			message:       "继续深修",
			expectedMode:  "service.deep_repair",
			expectedExtra: "runtime_exec_decision.rule=允许执行迁移/数据修复动作；不要退化为仅重启。",
		},
		{
			name:          "service retry only",
			message:       "仅重试",
			expectedMode:  "service.retry_only",
			expectedExtra: "runtime_exec_decision.rule=仅执行重启 + 健康检查；禁止迁移或数据改写。",
		},
		{
			name:          "build continue",
			message:       "继续构建修复",
			expectedMode:  "build.continue",
			expectedExtra: "runtime_exec_decision.rule=继续构建链自动修复，可执行依赖修复与重新构建。",
		},
		{
			name:          "build switch source",
			message:       "切换依赖源",
			expectedMode:  "build.switch_source",
			expectedExtra: "runtime_exec_decision.rule=优先切换镜像/离线源后再重试构建。",
		},
		{
			name:          "pause",
			message:       "暂停",
			expectedMode:  "paused",
			expectedExtra: "runtime_exec_decision.rule=停止自动执行，只输出暂停确认并等待新指令。",
		},
		{
			name:          "alternate command",
			message:       "替代命令: go test ./cmd/clawx -run TestBuildExecutionInputInjectsRuntimeExecDecisionSnapshot",
			expectedMode:  "blocked.command_override",
			expectedExtra: "runtime_exec_decision.alternate_command=go test ./cmd/clawx -run TestBuildExecutionInputInjectsRuntimeExecDecisionSnapshot",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			decision := service.Decision{
				Kind: service.DecisionExecute,
				Message: chatiface.Message{
					Text: tc.message,
				},
			}
			input := buildExecutionInput(decision, agentRuntime{agentID: "bid-all"})
			if !strings.Contains(input, "[Runtime Exec Decision Snapshot]") {
				t.Fatalf("expected runtime exec decision snapshot, got: %q", input)
			}
			if !strings.Contains(input, "runtime_exec_decision.mode="+tc.expectedMode) {
				t.Fatalf("expected runtime exec decision mode %q, got: %q", tc.expectedMode, input)
			}
			if !strings.Contains(input, "runtime_exec_decision.source=user_phrase") {
				t.Fatalf("expected runtime exec decision source, got: %q", input)
			}
			if tc.expectedExtra != "" && !strings.Contains(input, tc.expectedExtra) {
				t.Fatalf("expected runtime exec decision detail %q, got: %q", tc.expectedExtra, input)
			}
		})
	}
}

func TestBuildExecutionInputDoesNotInjectRuntimeExecDecisionForGenericContinue(t *testing.T) {
	decision := service.Decision{
		Kind: service.DecisionExecute,
		Message: chatiface.Message{
			Text: "继续",
		},
	}
	input := buildExecutionInput(decision, agentRuntime{agentID: "bid-all"})
	if strings.Contains(input, "[Runtime Exec Decision Snapshot]") {
		t.Fatalf("expected no runtime exec decision snapshot for generic continue, got: %q", input)
	}
}

func TestBuildExecutionInputInjectsRuntimeExecDecisionSnapshotFromPersistedState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	globalExecutionGoalStore = &executionGoalStore{
		byConv: map[string]executionGoalState{},
	}
	convID := "discord:-:persisted:u1|ch=discord|inst=default|agent=bid-all"
	setExecutionGoalState(convID, executionGoalState{
		Goal:                      "持续修复服务异常",
		Status:                    "running",
		RuntimeExecDecisionMode:   "service.retry_only",
		RuntimeExecDecisionSource: "user_phrase",
	})

	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: convID,
		Message: chatiface.Message{
			Text: "继续",
		},
	}
	input := buildExecutionInput(decision, agentRuntime{agentID: "bid-all"})
	if !strings.Contains(input, "[Runtime Exec Decision Snapshot]") {
		t.Fatalf("expected runtime exec decision snapshot, got: %q", input)
	}
	if !strings.Contains(input, "runtime_exec_decision.mode=service.retry_only") {
		t.Fatalf("expected persisted decision mode injected, got: %q", input)
	}
	if !strings.Contains(input, "runtime_exec_decision.source=persisted_state") {
		t.Fatalf("expected persisted decision source injected, got: %q", input)
	}
}

func TestBuildExecutionInputClearsPersistedRuntimeExecDecisionByUserPhrase(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	globalExecutionGoalStore = &executionGoalStore{
		byConv: map[string]executionGoalState{},
	}
	convID := "discord:-:persisted-clear:u1|ch=discord|inst=default|agent=bid-all"
	setExecutionGoalState(convID, executionGoalState{
		Goal:                      "持续修复服务异常",
		Status:                    "running",
		RuntimeExecDecisionMode:   "service.retry_only",
		RuntimeExecDecisionSource: "user_phrase",
	})

	clearDecision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: convID,
		Message: chatiface.Message{
			Text: "恢复默认策略",
		},
	}
	clearInput := buildExecutionInput(clearDecision, agentRuntime{agentID: "bid-all"})
	if !strings.Contains(clearInput, "runtime_exec_decision.mode=clear") {
		t.Fatalf("expected clear decision snapshot, got: %q", clearInput)
	}
	if !strings.Contains(clearInput, "runtime_exec_decision.source=user_phrase") {
		t.Fatalf("expected clear decision source, got: %q", clearInput)
	}
	state, ok := getExecutionGoalState(convID)
	if !ok {
		t.Fatalf("expected execution goal state exists")
	}
	if strings.TrimSpace(state.RuntimeExecDecisionMode) != "" {
		t.Fatalf("expected persisted decision cleared, got: %q", state.RuntimeExecDecisionMode)
	}

	continueDecision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: convID,
		Message: chatiface.Message{
			Text: "继续",
		},
	}
	continueInput := buildExecutionInput(continueDecision, agentRuntime{agentID: "bid-all"})
	if strings.Contains(continueInput, "[Runtime Exec Decision Snapshot]") {
		t.Fatalf("expected no persisted decision snapshot after clear, got: %q", continueInput)
	}
}
