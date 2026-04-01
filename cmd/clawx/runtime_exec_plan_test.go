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
		"/tmp/runtime_exec.jsonl",
		"exec_ids: rexec-1,rexec-2,rexec-3",
		0,
		false,
		"",
	)
	for _, section := range []string{"结论：", "问题：", "证据：", "下一步："} {
		if !strings.Contains(got, section) {
			t.Fatalf("expected section %q in message: %s", section, got)
		}
	}
}
