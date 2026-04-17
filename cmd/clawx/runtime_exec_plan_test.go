package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clawx/internal/application/runtimeorchestrator"
	"clawx/internal/infrastructure/config"
)

func TestApplyRuntimeExecPlanExecute(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	runtime := agentRuntime{
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{tmp},
			DefaultCWD:   tmp,
		},
	}
	plan := runtimeExecPlan{
		Type:   "runtime.exec",
		Mode:   "execute",
		Reason: "验证 runtime.exec 成功链路",
		Commands: []runtimeExecCommand{
			{Cmd: "echo hello", CWD: workspace},
		},
	}
	result := applyRuntimeExecPlan(context.Background(), runtime, plan, workspace, "conv-exec")
	if !result.Applied {
		t.Fatalf("expected applied result: %+v", result)
	}
	if result.Executed != 1 || result.Failed != 0 {
		t.Fatalf("unexpected counters: %+v", result)
	}
	if !strings.Contains(result.Message, "已执行完成") {
		t.Fatalf("expected concise success output, got: %s", result.Message)
	}
	if !strings.Contains(result.Message, "完成状态：已完成。") {
		t.Fatalf("expected completed status in success output, got: %s", result.Message)
	}
	if !strings.Contains(result.Message, "执行凭证：rexec-") {
		t.Fatalf("expected attestation id list in output, got: %s", result.Message)
	}
	if !strings.Contains(result.Message, "产物：") {
		t.Fatalf("expected success artifact summary in output, got: %s", result.Message)
	}
	planDoc := filepath.Join(workspace, "TASK_PLAN.md")
	if body, err := os.ReadFile(planDoc); err != nil {
		t.Fatalf("read plan doc: %v", err)
	} else if !strings.Contains(string(body), "验证 runtime.exec 成功链路") {
		t.Fatalf("expected goal persisted in TASK_PLAN.md, got: %s", string(body))
	}
	execDoc := filepath.Join(workspace, "TASK_EXECUTION.md")
	if body, err := os.ReadFile(execDoc); err != nil {
		t.Fatalf("read execution doc: %v", err)
	} else if !strings.Contains(string(body), "status=applied") {
		t.Fatalf("expected execution status persisted, got: %s", string(body))
	}
	stateDoc := filepath.Join(workspace, ".clawx", "workspace-state.json")
	if body, err := os.ReadFile(stateDoc); err != nil {
		t.Fatalf("read workspace-state: %v", err)
	} else if !strings.Contains(string(body), "\"taskTracking\"") {
		t.Fatalf("expected taskTracking snapshot in state file, got: %s", string(body))
	}
}

func TestApplyRuntimeExecPlanAttestationIncludesDecisionContext(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	homeDir := filepath.Join(tmp, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", homeDir)
	resetExecutionGoalStoreForTest(t, homeDir)

	conversationID := "conv-runtime-exec-attestation-decision-context"
	setExecutionGoalState(conversationID, executionGoalState{
		Goal:                      "持续修复构建链",
		Status:                    "running",
		RuntimeExecDecisionMode:   "build.continue",
		RuntimeExecDecisionSource: "user_phrase",
	})

	runtime := agentRuntime{
		agentID: "bid-all",
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{tmp},
			DefaultCWD:   tmp,
		},
	}
	plan := runtimeExecPlan{
		Type:                           "runtime.exec",
		Mode:                           "execute",
		Reason:                         "验证 runtime.exec 执行审计上下文",
		RuntimeExecDecisionMode:        "build.continue",
		RuntimeExecDecisionApplySource: "persisted_state",
		RuntimeExecDecisionLockSource:  "user_phrase",
		Commands: []runtimeExecCommand{
			{
				Cmd:            "echo hello",
				CWD:            workspace,
				Reason:         "runtime_exec_decision_build_continue_fallback",
				FallbackSource: "workspace",
			},
		},
	}
	result := applyRuntimeExecPlan(context.Background(), runtime, plan, workspace, conversationID)
	if !result.Applied || result.Executed != 1 {
		t.Fatalf("expected applied runtime exec for attestation test, got: %+v", result)
	}
	if !strings.Contains(result.Message, "执行来源：mode=build.continue apply_source=persisted_state lock_source=user_phrase fallback_source=workspace") {
		t.Fatalf("expected decision context line in user message, got: %s", result.Message)
	}
	records := listRecentRuntimeExecAttestations(conversationID, 2)
	if len(records) == 0 {
		t.Fatalf("expected attestation records for conversation=%s", conversationID)
	}
	last := records[len(records)-1]
	if strings.TrimSpace(last.RuntimeExecDecisionMode) != "build.continue" {
		t.Fatalf("expected decision mode in attestation, got: %+v", last)
	}
	if strings.TrimSpace(last.RuntimeExecDecisionSource) != "user_phrase" {
		t.Fatalf("expected decision source in attestation, got: %+v", last)
	}
	if strings.TrimSpace(last.RuntimeExecDecisionApplySource) != "persisted_state" {
		t.Fatalf("expected decision apply source in attestation, got: %+v", last)
	}
	if strings.TrimSpace(last.RuntimeExecDecisionLockSource) != "user_phrase" {
		t.Fatalf("expected decision lock source in attestation, got: %+v", last)
	}
	if strings.TrimSpace(last.PlanReason) != "验证 runtime.exec 执行审计上下文" {
		t.Fatalf("expected plan reason in attestation, got: %+v", last)
	}
	if strings.TrimSpace(last.StepReason) != "runtime_exec_decision_build_continue_fallback" {
		t.Fatalf("expected step reason in attestation, got: %+v", last)
	}
	if strings.TrimSpace(last.RuntimeExecDecisionFallbackSource) != "workspace" {
		t.Fatalf("expected fallback source in attestation, got: %+v", last)
	}
}

func TestSummarizeSuccessEvidenceArtifact(t *testing.T) {
	step := runtimeExecCommand{
		Cmd: "mkdir -p docs && cat > /tmp/demo/PLAN.md <<'EOF'\nhello\nEOF",
	}
	got := summarizeSuccessEvidence(step, "/tmp/demo", "")
	if !strings.Contains(got, "已生成文件 /tmp/demo/PLAN.md") {
		t.Fatalf("unexpected success summary: %s", got)
	}
}

func TestDetectSoftExecutionFailure_HTTPStatus(t *testing.T) {
	if err := detectSoftExecutionFailure("curl -I http://127.0.0.1:8000/api/health", "HTTP/1.1 500 Internal Server Error"); err == nil {
		t.Fatalf("expected soft failure for 500")
	}
	if err := detectSoftExecutionFailure("curl -I http://127.0.0.1:8000/api/health", "server: uvicorn\ncontent-type: text/plain; charset=utf-8\nInternal Server Error"); err == nil {
		t.Fatalf("expected soft failure for uvicorn internal server error output")
	}
	if err := detectSoftExecutionFailure("curl -I http://127.0.0.1:8000/api/health", "HTTP/2 405"); err == nil {
		t.Fatalf("expected soft failure for 405")
	}
	if err := detectSoftExecutionFailure("echo ok", "HTTP/1.1 500"); err != nil {
		t.Fatalf("non-curl command should not trigger soft failure: %v", err)
	}
}

func TestApplyRuntimeExecPlanRejectDangerous(t *testing.T) {
	tmp := t.TempDir()
	runtime := agentRuntime{
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{tmp},
			DefaultCWD:   tmp,
		},
	}
	plan := runtimeExecPlan{
		Type: "runtime.exec",
		Mode: "execute",
		Commands: []runtimeExecCommand{
			{Cmd: "rm -rf /", CWD: tmp},
		},
	}
	result := applyRuntimeExecPlan(context.Background(), runtime, plan, tmp, "conv-reject")
	if result.Applied || result.Executed != 0 || result.Failed != 1 {
		t.Fatalf("expected rejected command, got: %+v", result)
	}
	if !strings.Contains(result.Message, "都失败了") {
		t.Fatalf("expected human-readable failed summary, got: %s", result.Message)
	}
	if !strings.Contains(result.Message, "完成状态：失败。") {
		t.Fatalf("expected failed status in blocked output, got: %s", result.Message)
	}
	if !strings.Contains(result.Message, "恢复策略：检测到高风险命令，已停止自治执行") {
		t.Fatalf("expected hard-stop recovery strategy for dangerous command, got: %s", result.Message)
	}
	if !strings.Contains(result.Message, "下一步：检测到高风险命令") {
		t.Fatalf("expected explicit next step for blocked command, got: %s", result.Message)
	}
}

func TestApplyRuntimeExecPlanLeadModeDispatchOnly(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	svc := runtimeorchestrator.NewService()
	boot, err := svc.Bootstrap(runtimeorchestrator.BootstrapOptions{
		WorkspaceRoot: workspace,
		AgentID:       "bid-all",
		WorkerRoles:   []string{"planner", "executor", "reviewer"},
	})
	if err != nil {
		t.Fatalf("bootstrap runtime: %v", err)
	}

	runtime := agentRuntime{
		agentID: "bid-all",
		cwd:     workspace,
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{tmp},
			DefaultCWD:   workspace,
		},
	}
	plan := runtimeExecPlan{
		Type: "runtime.exec",
		Mode: "execute",
		Commands: []runtimeExecCommand{
			{Cmd: "echo lead-direct-exec-block > lead_guard.txt", CWD: workspace},
		},
	}
	result := applyRuntimeExecPlan(context.Background(), runtime, plan, workspace, "conv-lead")
	if !result.Applied || result.Status != "applied" {
		t.Fatalf("expected scheduled apply result, got: %+v", result)
	}
	if result.Executed != 0 {
		t.Fatalf("lead mode should not directly execute command: %+v", result)
	}
	if !strings.Contains(result.Message, "Lead 调度模式") {
		t.Fatalf("expected lead scheduling message, got: %s", result.Message)
	}
	if _, err := os.Stat(filepath.Join(workspace, "lead_guard.txt")); !os.IsNotExist(err) {
		t.Fatalf("lead mode should not create command artifact directly")
	}
	items, err := runtimeorchestrator.NewQueue(boot.TasksFile).List()
	if err != nil {
		t.Fatalf("list queued tasks: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one queued task, got=%d", len(items))
	}
	if items[0].Status != runtimeorchestrator.TaskRunning {
		t.Fatalf("expected task assigned to worker, got status=%s", items[0].Status)
	}
	if strings.TrimSpace(items[0].AssignedWorkerID) == "" {
		t.Fatalf("expected assigned worker id")
	}
}

func TestRenderRuntimeExecUserMessageStructuredSections(t *testing.T) {
	got := renderRuntimeExecUserMessage(
		"partial",
		3,
		2,
		1,
		"step 2 执行失败：curl: (22) The requested URL returned error: 500",
		"curl -sS -X POST http://127.0.0.1:8000/api/sources",
		nil,
		nil,
		"/tmp/runtime_exec.jsonl",
		"exec_ids: rexec-1,rexec-2,rexec-3",
		0,
		false,
		"",
		"执行来源：mode=build.continue apply_source=persisted_state lock_source=user_phrase fallback_source=workspace",
	)
	for _, section := range []string{"结论：", "问题：", "证据：", "下一步："} {
		if !strings.Contains(got, section) {
			t.Fatalf("expected section %q in message: %s", section, got)
		}
	}
	if !strings.Contains(got, "完成状态：部分失败。") {
		t.Fatalf("expected partial status section in message: %s", got)
	}
	if !strings.Contains(got, "执行来源：mode=build.continue apply_source=persisted_state lock_source=user_phrase fallback_source=workspace") {
		t.Fatalf("expected decision context line in message: %s", got)
	}
	if !strings.Contains(got, "恢复策略：") {
		t.Fatalf("expected recovery strategy section in message: %s", got)
	}
}

func TestRuntimeExecRecoveryStrategyDecisionNeeded(t *testing.T) {
	got := runtimeExecRecoveryStrategy("step 1 命令被拒绝（dangerous command pattern）", 0, true)
	if !strings.Contains(got, "高风险命令") {
		t.Fatalf("expected blocked decision strategy, got: %s", got)
	}
}

func TestRuntimeExecRecoveryStrategyDecisionNeededService(t *testing.T) {
	got := runtimeExecRecoveryStrategy("http probe returned 500", 0, true)
	if !strings.Contains(got, "继续深修") {
		t.Fatalf("expected service decision strategy with continue-deep option, got: %s", got)
	}
}

func TestRuntimeExecRecoveryStrategyDecisionNeededBuild(t *testing.T) {
	got := runtimeExecRecoveryStrategy("build backend failed", 0, true)
	if !strings.Contains(got, "继续构建修复") {
		t.Fatalf("expected build decision strategy with continue-build option, got: %s", got)
	}
}

func TestRuntimeExecRecoveryStrategyAutoRepairRounds(t *testing.T) {
	got := runtimeExecRecoveryStrategy("http probe returned 500", 2, false)
	if !strings.Contains(got, "已执行 2 轮自治修复") {
		t.Fatalf("expected auto-repair strategy with rounds, got: %s", got)
	}
}

func TestRuntimeExecDecisionOptionsService(t *testing.T) {
	got := runtimeExecDecisionOptions("http probe returned 500", true)
	if !strings.Contains(got, "继续深修") || !strings.Contains(got, "仅重试") {
		t.Fatalf("expected service decision options, got: %s", got)
	}
}

func TestRuntimeExecDecisionOptionsBuild(t *testing.T) {
	got := runtimeExecDecisionOptions("build backend failed", true)
	if !strings.Contains(got, "继续构建修复") || !strings.Contains(got, "切换依赖源") {
		t.Fatalf("expected build decision options, got: %s", got)
	}
}

func TestRenderRuntimeExecUserMessageIncludesDecisionOptionsWhenNeeded(t *testing.T) {
	got := renderRuntimeExecUserMessage(
		"failed",
		1,
		0,
		1,
		"http probe returned 500",
		"curl -sS -X POST http://127.0.0.1:8000/api/sources",
		nil,
		nil,
		"/tmp/runtime_exec.jsonl",
		"exec_ids: rexec-1",
		0,
		true,
		"同一服务类问题连续出现（4 次）。我已完成日志诊断与重启修复。你可以回复“继续深修”（允许迁移/数据修复）、“仅重试”（只做重启+健康检查）或“暂停”。",
		"",
	)
	if !strings.Contains(got, "你可以直接回复：继续深修 / 仅重试 / 暂停。") {
		t.Fatalf("expected structured decision options in user message, got: %s", got)
	}
}

func TestRenderRuntimeExecUserMessageDecisionNextStepUsesCompactPrompt(t *testing.T) {
	got := renderRuntimeExecUserMessage(
		"failed",
		1,
		0,
		1,
		"http probe returned 500",
		"curl -sS -X POST http://127.0.0.1:8000/api/sources",
		nil,
		nil,
		"/tmp/runtime_exec.jsonl",
		"exec_ids: rexec-1",
		0,
		true,
		"同一服务类问题连续出现（4 次）。我已完成日志诊断与重启修复。你可以回复“继续深修”（允许迁移/数据修复）、“仅重试”（只做重启+健康检查）或“暂停”。",
		"",
	)
	if !strings.Contains(got, "决策背景：同一服务类问题连续出现（4 次）。我已完成日志诊断与重启修复。") {
		t.Fatalf("expected compact decision background in user message, got: %s", got)
	}
	if !strings.Contains(got, "下一步：请从上面的选项里回复一个指令，我会立即继续执行。") {
		t.Fatalf("expected compact next-step decision prompt, got: %s", got)
	}
}

func TestSummarizeRuntimeExecDecisionReasonStripsOptions(t *testing.T) {
	raw := "同一服务类问题连续出现（4 次）。我已完成日志诊断与重启修复。你可以直接回复：继续深修 / 仅重试 / 暂停。"
	got := summarizeRuntimeExecDecisionReason(raw)
	if strings.Contains(got, "继续深修 / 仅重试 / 暂停") {
		t.Fatalf("expected option list stripped from decision summary, got: %s", got)
	}
	if !strings.Contains(got, "同一服务类问题连续出现（4 次）") {
		t.Fatalf("expected decision summary core context retained, got: %s", got)
	}
}

func TestBuildRuntimeExecDecisionContextLine(t *testing.T) {
	plan := runtimeExecPlan{
		RuntimeExecDecisionMode:        "build.continue",
		RuntimeExecDecisionApplySource: "persisted_state",
		RuntimeExecDecisionLockSource:  "user_phrase",
		Commands: []runtimeExecCommand{
			{Cmd: "echo a", FallbackSource: "workspace"},
			{Cmd: "echo b", FallbackSource: "env"},
			{Cmd: "echo c", FallbackSource: "workspace"},
		},
	}
	line := buildRuntimeExecDecisionContextLine(plan)
	want := "执行来源：mode=build.continue apply_source=persisted_state lock_source=user_phrase fallback_source=env,workspace"
	if line != want {
		t.Fatalf("unexpected decision context line: got=%q want=%q", line, want)
	}
}

func TestShouldSuppressDuplicateRuntimeExec(t *testing.T) {
	conversationID := "conv-dedupe-runtime-exec"
	cmdline := "pip install -e . --index-url https://pypi.org/simple"
	cwd := "/tmp/demo"
	digest := digestOutput("Name or service not known")
	for i := 0; i < 2; i++ {
		if err := appendRuntimeExecAttestation(runtimeExecAttestationRecord{
			ExecID:         newRuntimeExecID(),
			ConversationID: conversationID,
			CWD:            cwd,
			Command:        cmdline,
			Success:        false,
			OutputDigest:   digest,
			OutputPreview:  "Name or service not known",
			ErrorSummary:   "exit status 1",
		}); err != nil {
			t.Fatalf("append attestation: %v", err)
		}
	}
	suppressed, matched := shouldSuppressDuplicateRuntimeExec(conversationID, cwd, cmdline)
	if !suppressed {
		t.Fatalf("expected duplicate failed command to be suppressed")
	}
	if !strings.Contains(matched, "pip install") {
		t.Fatalf("expected matched command summary, got: %s", matched)
	}
}

func TestCollectUniqueFailureEvidence(t *testing.T) {
	conversationID := "conv-failure-evidence-dedupe"
	firstID := newRuntimeExecID()
	secondID := newRuntimeExecID()
	thirdID := newRuntimeExecID()
	records := []runtimeExecAttestationRecord{
		{
			ExecID:         firstID,
			ConversationID: conversationID,
			Command:        "cmd-1",
			Success:        false,
			OutputDigest:   digestOutput("network unreachable"),
			ErrorSummary:   "network unreachable",
			OutputPreview:  "network unreachable",
		},
		{
			ExecID:         secondID,
			ConversationID: conversationID,
			Command:        "cmd-1",
			Success:        false,
			OutputDigest:   digestOutput("network unreachable"),
			ErrorSummary:   "network unreachable",
			OutputPreview:  "network unreachable",
		},
		{
			ExecID:         thirdID,
			ConversationID: conversationID,
			Command:        "cmd-2",
			Success:        false,
			OutputDigest:   digestOutput("permission denied"),
			ErrorSummary:   "permission denied",
			OutputPreview:  "permission denied",
		},
	}
	for _, record := range records {
		if err := appendRuntimeExecAttestation(record); err != nil {
			t.Fatalf("append attestation: %v", err)
		}
	}
	evidence := collectUniqueFailureEvidence([]string{firstID, secondID, thirdID}, 5)
	if len(evidence) != 2 {
		t.Fatalf("expected deduplicated failure evidence count=2, got=%d evidence=%v", len(evidence), evidence)
	}
	if !strings.Contains(strings.Join(evidence, " | "), "network unreachable") || !strings.Contains(strings.Join(evidence, " | "), "permission denied") {
		t.Fatalf("expected both evidence classes retained, got: %v", evidence)
	}
}

func TestAugmentDecisionFromFailureHistoryPermissionThreshold(t *testing.T) {
	conversationID := "conv-permission-threshold"
	globalRuntimeExecFailureTracker.clear(conversationID)
	t.Cleanup(func() { globalRuntimeExecFailureTracker.clear(conversationID) })

	reason := "step 1 执行失败：outside allowed roots"
	for i := 1; i <= 3; i++ {
		needDecision, decisionReason := augmentDecisionFromFailureHistory(conversationID, reason, false, "")
		if i < 3 && needDecision {
			t.Fatalf("should not escalate before threshold, round=%d reason=%s", i, decisionReason)
		}
		if i == 3 && (!needDecision || !strings.Contains(decisionReason, "权限问题连续出现")) {
			t.Fatalf("expected permission escalation at threshold, round=%d reason=%s", i, decisionReason)
		}
		if i == 3 && !strings.Contains(decisionReason, "回复“继续”") {
			t.Fatalf("expected permission escalation keep-continue template, got: %s", decisionReason)
		}
	}
}

func TestAugmentDecisionFromFailureHistoryServiceThreshold(t *testing.T) {
	conversationID := "conv-service-threshold"
	globalRuntimeExecFailureTracker.clear(conversationID)
	t.Cleanup(func() { globalRuntimeExecFailureTracker.clear(conversationID) })

	reason := "step 2 执行失败：http probe returned 500"
	for i := 1; i <= 4; i++ {
		needDecision, decisionReason := augmentDecisionFromFailureHistory(conversationID, reason, false, "")
		if i < 4 && needDecision {
			t.Fatalf("should not escalate before threshold, round=%d reason=%s", i, decisionReason)
		}
		if i == 4 && (!needDecision || !strings.Contains(decisionReason, "服务类问题连续出现")) {
			t.Fatalf("expected service escalation at threshold, round=%d reason=%s", i, decisionReason)
		}
		if i == 4 && !strings.Contains(decisionReason, "回复“继续深修”") {
			t.Fatalf("expected service escalation decision template, got: %s", decisionReason)
		}
	}
}

func TestAugmentDecisionFromFailureHistoryBuildThreshold(t *testing.T) {
	conversationID := "conv-build-threshold"
	globalRuntimeExecFailureTracker.clear(conversationID)
	t.Cleanup(func() { globalRuntimeExecFailureTracker.clear(conversationID) })

	reason := "step 3 执行失败：build backend failed"
	for i := 1; i <= 4; i++ {
		needDecision, decisionReason := augmentDecisionFromFailureHistory(conversationID, reason, false, "")
		if i < 4 && needDecision {
			t.Fatalf("should not escalate before threshold, round=%d reason=%s", i, decisionReason)
		}
		if i == 4 && (!needDecision || !strings.Contains(decisionReason, "构建类问题连续出现")) {
			t.Fatalf("expected build escalation at threshold, round=%d reason=%s", i, decisionReason)
		}
		if i == 4 && !strings.Contains(decisionReason, "回复“继续构建修复”") {
			t.Fatalf("expected build escalation decision template, got: %s", decisionReason)
		}
	}
}
