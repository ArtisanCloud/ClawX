package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clawx/internal/application/runtimeorchestrator"
	"clawx/internal/infrastructure/config"
)

func TestLeadWorkerLoopRunCycleExecutesQueuedTask(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	resetExecutionGoalStoreForTest(t, home)

	workspace := filepath.Join(tmp, "workspace")
	bootstrap := runtimeorchestrator.NewService()
	if _, err := bootstrap.Bootstrap(runtimeorchestrator.BootstrapOptions{
		WorkspaceRoot: workspace,
		AgentID:       "bid-all",
		WorkerRoles:   []string{"executor", "reviewer"},
	}); err != nil {
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
	loop, err := newLeadWorkerLoop(runtime, workspace)
	if err != nil {
		t.Fatalf("new lead worker loop: %v", err)
	}
	t.Cleanup(loop.close)

	if _, err := loop.queue.Enqueue(runtimeorchestrator.RuntimeTask{
		Source: "runtime.exec",
		Intent: "runtime.exec",
		Payload: map[string]interface{}{
			"cmd":            "echo worker-ok > worker_result.txt",
			"cwd":            workspace,
			"conversation":   "conv-lead-loop-success",
			"parent_task_id": "task-parent-success",
			"reason":         "生成验证文件",
		},
		Status: runtimeorchestrator.TaskQueued,
	}); err != nil {
		t.Fatalf("enqueue task: %v", err)
	}

	if err := loop.runCycle(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("run cycle: %v", err)
	}

	items, err := loop.queue.List()
	if err != nil {
		t.Fatalf("queue list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one task, got: %d", len(items))
	}
	if items[0].Status != runtimeorchestrator.TaskSucceeded {
		t.Fatalf("expected task succeeded, got: %s", items[0].Status)
	}
	artifact := filepath.Join(workspace, "worker_result.txt")
	body, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if !strings.Contains(string(body), "worker-ok") {
		t.Fatalf("unexpected artifact content: %s", string(body))
	}
	execDoc := filepath.Join(workspace, taskExecutionDocName)
	execBody, err := os.ReadFile(execDoc)
	if err != nil {
		t.Fatalf("read execution doc: %v", err)
	}
	if !strings.Contains(string(execBody), "status=applied") {
		t.Fatalf("expected status=applied in TASK_EXECUTION.md, got: %s", string(execBody))
	}
	state, ok := getExecutionGoalState("conv-lead-loop-success")
	if !ok {
		t.Fatalf("expected execution goal state for delegated success")
	}
	if state.Status != "completed" {
		t.Fatalf("expected completed goal status for delegated success, got: %s", state.Status)
	}
	if !strings.Contains(state.LastResult, "parent_task_id=task-parent-success") || !strings.Contains(state.LastResult, "succeeded=1") {
		t.Fatalf("expected parent aggregated progress in last result, got: %s", state.LastResult)
	}
}

func TestLeadWorkerLoopRunCycleSkipsWhenLeadModeDisabled(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	bootstrap := runtimeorchestrator.NewService()
	if _, err := bootstrap.Bootstrap(runtimeorchestrator.BootstrapOptions{
		WorkspaceRoot: workspace,
		AgentID:       "bid-all",
		WorkerRoles:   []string{"executor"},
	}); err != nil {
		t.Fatalf("bootstrap runtime: %v", err)
	}
	metaPath := filepath.Join(workspace, ".clawx", "runtime", "runtime_meta.json")
	if err := os.WriteFile(metaPath, []byte("{\"lead_mode\":false}\n"), 0o644); err != nil {
		t.Fatalf("write runtime meta: %v", err)
	}

	runtime := agentRuntime{
		agentID: "bid-all",
		cwd:     workspace,
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{tmp},
			DefaultCWD:   workspace,
		},
	}
	loop, err := newLeadWorkerLoop(runtime, workspace)
	if err != nil {
		t.Fatalf("new lead worker loop: %v", err)
	}
	t.Cleanup(loop.close)

	if _, err := loop.queue.Enqueue(runtimeorchestrator.RuntimeTask{
		Source: "runtime.exec",
		Intent: "runtime.exec",
		Payload: map[string]interface{}{
			"cmd": "echo should-not-run > should_not_exist.txt",
			"cwd": workspace,
		},
		Status: runtimeorchestrator.TaskQueued,
	}); err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	if err := loop.runCycle(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("run cycle: %v", err)
	}
	items, err := loop.queue.List()
	if err != nil {
		t.Fatalf("queue list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one task, got: %d", len(items))
	}
	if items[0].Status != runtimeorchestrator.TaskQueued {
		t.Fatalf("expected task remain queued when lead mode disabled, got: %s", items[0].Status)
	}
	if _, err := os.Stat(filepath.Join(workspace, "should_not_exist.txt")); !os.IsNotExist(err) {
		t.Fatalf("task should not run when lead mode disabled")
	}
}

func TestLeadWorkerLoopRunCycleMarksFailureAndBlocksConversation(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	resetExecutionGoalStoreForTest(t, home)

	workspace := filepath.Join(tmp, "workspace")
	bootstrap := runtimeorchestrator.NewService()
	if _, err := bootstrap.Bootstrap(runtimeorchestrator.BootstrapOptions{
		WorkspaceRoot: workspace,
		AgentID:       "bid-all",
		WorkerRoles:   []string{"executor"},
	}); err != nil {
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
	loop, err := newLeadWorkerLoop(runtime, workspace)
	if err != nil {
		t.Fatalf("new lead worker loop: %v", err)
	}
	t.Cleanup(loop.close)

	conversationID := "conv-lead-loop-failed"
	if _, err := loop.queue.Enqueue(runtimeorchestrator.RuntimeTask{
		Source: "runtime.exec",
		Intent: "runtime.exec",
		Payload: map[string]interface{}{
			"cmd":            "false",
			"cwd":            workspace,
			"conversation":   conversationID,
			"parent_task_id": "task-parent-failed",
		},
		Status: runtimeorchestrator.TaskQueued,
	}); err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	if err := loop.runCycle(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("run cycle: %v", err)
	}

	items, err := loop.queue.List()
	if err != nil {
		t.Fatalf("queue list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one task, got: %d", len(items))
	}
	if items[0].Status != runtimeorchestrator.TaskFailed {
		t.Fatalf("expected task failed, got: %s", items[0].Status)
	}
	if got := readRuntimeTaskPayloadString(items[0].Payload, "last_error"); got == "" {
		t.Fatalf("expected last_error persisted to task payload, got empty payload=%+v", items[0].Payload)
	}
	if got := readRuntimeTaskPayloadString(items[0].Payload, "last_finished_at"); got == "" {
		t.Fatalf("expected last_finished_at persisted to task payload, got empty payload=%+v", items[0].Payload)
	}
	state, ok := getExecutionGoalState(conversationID)
	if !ok {
		t.Fatalf("expected execution goal state for failed task")
	}
	if state.Status != "blocked" {
		t.Fatalf("expected blocked goal status, got: %s", state.Status)
	}
	if !strings.Contains(state.LastResult, "parent_task_id=task-parent-failed") || !strings.Contains(state.LastResult, "failed=1") {
		t.Fatalf("expected parent aggregated failed progress in last result, got: %s", state.LastResult)
	}
}

func TestLeadWorkerLoopRunCycleTaskControlSelfHeal(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	bootstrap := runtimeorchestrator.NewService()
	if _, err := bootstrap.Bootstrap(runtimeorchestrator.BootstrapOptions{
		WorkspaceRoot: workspace,
		AgentID:       "bid-all",
		WorkerRoles:   []string{"executor"},
	}); err != nil {
		t.Fatalf("bootstrap runtime: %v", err)
	}

	activeFlag := filepath.Join(tmp, "service-active.flag")
	logPath := filepath.Join(tmp, "systemctl.log")
	systemctlPath := filepath.Join(tmp, "systemctl")
	systemctlScript := "#!/usr/bin/env bash\nset -euo pipefail\n" +
		"echo \"$@\" >> \"$FAKE_SYSTEMCTL_LOG\"\n" +
		"if [[ \"$1\" == \"--user\" && \"$2\" == \"is-active\" ]]; then\n" +
		"  if [[ -f \"$FAKE_SERVICE_ACTIVE_FLAG\" ]]; then\n" +
		"    echo \"active\"\n" +
		"  else\n" +
		"    echo \"inactive\"\n" +
		"  fi\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [[ \"$1\" == \"--user\" && \"$2\" == \"restart\" ]]; then\n" +
		"  : > \"$FAKE_SERVICE_ACTIVE_FLAG\"\n" +
		"  exit 0\n" +
		"fi\n"
	if err := os.WriteFile(systemctlPath, []byte(systemctlScript), 0o755); err != nil {
		t.Fatalf("write fake systemctl: %v", err)
	}

	t.Setenv("FAKE_SYSTEMCTL_LOG", logPath)
	t.Setenv("FAKE_SERVICE_ACTIVE_FLAG", activeFlag)
	t.Setenv("PATH", fmt.Sprintf("%s:%s", tmp, os.Getenv("PATH")))
	t.Setenv("CLAWX_RUNTIME_SERVICE_ALLOWLIST", "clawx-*")
	t.Setenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_ENABLED", "1")
	t.Setenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_TICK", "1ms")
	t.Setenv("CLAWX_RUNTIME_TASK_CONTROL_STATUS_SELF_HEAL", "1")

	runtime := agentRuntime{
		agentID: "bid-all",
		cwd:     workspace,
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{tmp},
			DefaultCWD:   workspace,
		},
	}
	loop, err := newLeadWorkerLoop(runtime, workspace)
	if err != nil {
		t.Fatalf("new lead worker loop: %v", err)
	}
	t.Cleanup(loop.close)

	if err := loop.runCycle(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("run cycle: %v", err)
	}

	logBody, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake systemctl log: %v", err)
	}
	if count := strings.Count(string(logBody), "--user restart clawx-bid-all.service"); count < 1 {
		t.Fatalf("expected lead worker self-heal to restart service, got=%d log=%s", count, string(logBody))
	}

	state, ok, err := loadWorkspaceTaskTrackingState(workspace)
	if err != nil || !ok {
		t.Fatalf("expected workspace state after lead self-heal, ok=%v err=%v", ok, err)
	}
	if op := readTaskTrackingString(state.TaskTrackingSnapshot, "last_task_control_operation"); !strings.EqualFold(strings.TrimSpace(op), "ensure_running") {
		t.Fatalf("expected last_task_control_operation ensure_running, got: %s", op)
	}
}

func TestLeadWorkerLoopRunCycleTaskControlSelfHealEmitsProgressNotification(t *testing.T) {
	resetConversationProgressNotifierForTest()
	t.Cleanup(resetConversationProgressNotifierForTest)

	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	bootstrap := runtimeorchestrator.NewService()
	if _, err := bootstrap.Bootstrap(runtimeorchestrator.BootstrapOptions{
		WorkspaceRoot: workspace,
		AgentID:       "bid-all",
		WorkerRoles:   []string{"executor"},
	}); err != nil {
		t.Fatalf("bootstrap runtime: %v", err)
	}

	activeFlag := filepath.Join(tmp, "service-active.flag")
	logPath := filepath.Join(tmp, "systemctl.log")
	systemctlPath := filepath.Join(tmp, "systemctl")
	systemctlScript := "#!/usr/bin/env bash\nset -euo pipefail\n" +
		"echo \"$@\" >> \"$FAKE_SYSTEMCTL_LOG\"\n" +
		"if [[ \"$1\" == \"--user\" && \"$2\" == \"is-active\" ]]; then\n" +
		"  if [[ -f \"$FAKE_SERVICE_ACTIVE_FLAG\" ]]; then\n" +
		"    echo \"active\"\n" +
		"  else\n" +
		"    echo \"inactive\"\n" +
		"  fi\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [[ \"$1\" == \"--user\" && \"$2\" == \"restart\" ]]; then\n" +
		"  : > \"$FAKE_SERVICE_ACTIVE_FLAG\"\n" +
		"  exit 0\n" +
		"fi\n"
	if err := os.WriteFile(systemctlPath, []byte(systemctlScript), 0o755); err != nil {
		t.Fatalf("write fake systemctl: %v", err)
	}

	notifyConversationID := "discord:-:1472233703659802706:1470834159067988171|ch=discord|inst=discord-default|agent=bid-all"
	notifyCh := make(chan string, 1)
	registerConversationProgressNotifier(notifyConversationID, func(_ context.Context, message string) error {
		notifyCh <- message
		return nil
	})

	now := time.Now().UTC()
	if err := ensureTaskTrackingScaffold(workspace, now); err != nil {
		t.Fatalf("ensure task tracking scaffold: %v", err)
	}
	if err := updateWorkspaceTaskState(workspace, now, map[string]interface{}{
		"task_control_route_hints": map[string]map[string]string{
			"discord:discord-default:direct:1470834159067988171:thread:1472233703659802706": {
				"service":         "clawx-bid-all",
				"health_url":      "http://127.0.0.1:19080/healthz",
				"operation":       "ensure_running",
				"updated_at":      now.Format(time.RFC3339),
				"conversation_id": notifyConversationID,
			},
		},
	}); err != nil {
		t.Fatalf("write task control route hint: %v", err)
	}

	t.Setenv("FAKE_SYSTEMCTL_LOG", logPath)
	t.Setenv("FAKE_SERVICE_ACTIVE_FLAG", activeFlag)
	t.Setenv("PATH", fmt.Sprintf("%s:%s", tmp, os.Getenv("PATH")))
	t.Setenv("CLAWX_RUNTIME_SERVICE_ALLOWLIST", "clawx-*")
	t.Setenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_ENABLED", "1")
	t.Setenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_TICK", "1ms")
	t.Setenv("CLAWX_RUNTIME_TASK_CONTROL_STATUS_SELF_HEAL", "1")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_DISABLED", "0")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_MIN_INTERVAL", "0")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_TIMEOUT", "1s")

	runtime := agentRuntime{
		agentID: "bid-all",
		cwd:     workspace,
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{tmp},
			DefaultCWD:   workspace,
		},
	}
	loop, err := newLeadWorkerLoop(runtime, workspace)
	if err != nil {
		t.Fatalf("new lead worker loop: %v", err)
	}
	t.Cleanup(loop.close)

	if err := loop.runCycle(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("run cycle: %v", err)
	}

	select {
	case message := <-notifyCh:
		if !strings.Contains(message, "auto_recovery=applied") {
			t.Fatalf("expected applied self-heal progress message, got: %s", message)
		}
		if !strings.Contains(message, "service=clawx-bid-all") {
			t.Fatalf("expected service in self-heal progress message, got: %s", message)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected self-heal progress notification")
	}
}

func TestLeadWorkerLoopRunCycleTaskControlSelfHealEmitsAlertRecoveryNotification(t *testing.T) {
	resetConversationProgressNotifierForTest()
	t.Cleanup(resetConversationProgressNotifierForTest)

	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	bootstrap := runtimeorchestrator.NewService()
	if _, err := bootstrap.Bootstrap(runtimeorchestrator.BootstrapOptions{
		WorkspaceRoot: workspace,
		AgentID:       "bid-all",
		WorkerRoles:   []string{"executor"},
	}); err != nil {
		t.Fatalf("bootstrap runtime: %v", err)
	}

	activeFlag := filepath.Join(tmp, "service-active.flag")
	logPath := filepath.Join(tmp, "systemctl.log")
	systemctlPath := filepath.Join(tmp, "systemctl")
	systemctlScript := "#!/usr/bin/env bash\nset -euo pipefail\n" +
		"echo \"$@\" >> \"$FAKE_SYSTEMCTL_LOG\"\n" +
		"if [[ \"$1\" == \"--user\" && \"$2\" == \"is-active\" ]]; then\n" +
		"  if [[ -f \"$FAKE_SERVICE_ACTIVE_FLAG\" ]]; then\n" +
		"    echo \"active\"\n" +
		"  else\n" +
		"    echo \"inactive\"\n" +
		"  fi\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [[ \"$1\" == \"--user\" && \"$2\" == \"restart\" ]]; then\n" +
		"  : > \"$FAKE_SERVICE_ACTIVE_FLAG\"\n" +
		"  exit 0\n" +
		"fi\n"
	if err := os.WriteFile(systemctlPath, []byte(systemctlScript), 0o755); err != nil {
		t.Fatalf("write fake systemctl: %v", err)
	}

	notifyConversationID := "lead-worker-self-heal:bid-all"
	notifyCh := make(chan string, 2)
	registerConversationProgressNotifier(notifyConversationID, func(_ context.Context, message string) error {
		notifyCh <- message
		return nil
	})

	now := time.Now().UTC()
	if err := ensureTaskTrackingScaffold(workspace, now); err != nil {
		t.Fatalf("ensure task tracking scaffold: %v", err)
	}
	if err := updateWorkspaceTaskState(workspace, now, map[string]interface{}{
		"last_task_control_self_heal_service": "clawx-bid-all",
		"last_task_control_self_heal_state":   "throttled",
		"last_task_control_self_heal_at":      now.Format(time.RFC3339),
		"service_self_heal_alert_levels":      map[string]string{"clawx-bid-all": "critical"},
		"service_self_heal_alert_states":      map[string]string{"clawx-bid-all": "throttled"},
	}); err != nil {
		t.Fatalf("seed self-heal alert state: %v", err)
	}

	t.Setenv("FAKE_SYSTEMCTL_LOG", logPath)
	t.Setenv("FAKE_SERVICE_ACTIVE_FLAG", activeFlag)
	t.Setenv("PATH", fmt.Sprintf("%s:%s", tmp, os.Getenv("PATH")))
	t.Setenv("CLAWX_RUNTIME_SERVICE_ALLOWLIST", "clawx-*")
	t.Setenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_ENABLED", "1")
	t.Setenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_TICK", "1ms")
	t.Setenv("CLAWX_RUNTIME_TASK_CONTROL_STATUS_SELF_HEAL", "1")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_DISABLED", "0")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_MIN_INTERVAL", "0")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_TIMEOUT", "1s")
	t.Setenv("CLAWX_TASK_CONTROL_SELF_HEAL_ALERT_WINDOW", "10m")

	runtime := agentRuntime{
		agentID: "bid-all",
		cwd:     workspace,
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{tmp},
			DefaultCWD:   workspace,
		},
	}
	loop, err := newLeadWorkerLoop(runtime, workspace)
	if err != nil {
		t.Fatalf("new lead worker loop: %v", err)
	}
	t.Cleanup(loop.close)

	if err := loop.runCycle(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("run cycle: %v", err)
	}

	select {
	case message := <-notifyCh:
		if !strings.Contains(message, "self_heal_alert_level=ok") {
			t.Fatalf("expected recovery alert message, got: %s", message)
		}
		if !strings.Contains(message, "previous_alert_level=critical") {
			t.Fatalf("expected previous critical alert evidence, got: %s", message)
		}
		if !strings.Contains(message, "playbook_1=当前已恢复") {
			t.Fatalf("expected recovery playbook in alert message, got: %s", message)
		}
		if strings.Contains(message, "auto_recovery=applied") {
			t.Fatalf("expected alert transition message only, got: %s", message)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected self-heal recovery alert notification")
	}

	time.Sleep(80 * time.Millisecond)
	select {
	case extra := <-notifyCh:
		t.Fatalf("expected single recovery alert notification, got extra: %s", extra)
	default:
	}

	state, ok, err := loadWorkspaceTaskTrackingState(workspace)
	if err != nil || !ok {
		t.Fatalf("expected workspace state after recovery alert, ok=%v err=%v", ok, err)
	}
	alertLevels := readTaskTrackingStringMap(state.TaskTrackingSnapshot, "service_self_heal_alert_levels")
	if strings.TrimSpace(strings.ToLower(alertLevels["clawx-bid-all"])) != "ok" {
		t.Fatalf("expected persisted alert level ok, got: %+v", alertLevels)
	}
}

func TestLeadWorkerLoopRunCycleTaskControlSelfHealRoutesWarningAlertToOpsConversation(t *testing.T) {
	resetConversationProgressNotifierForTest()
	t.Cleanup(resetConversationProgressNotifierForTest)

	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	bootstrap := runtimeorchestrator.NewService()
	if _, err := bootstrap.Bootstrap(runtimeorchestrator.BootstrapOptions{
		WorkspaceRoot: workspace,
		AgentID:       "bid-all",
		WorkerRoles:   []string{"executor"},
	}); err != nil {
		t.Fatalf("bootstrap runtime: %v", err)
	}

	healthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("down"))
	}))
	defer healthServer.Close()
	healthURL := strings.TrimSpace(healthServer.URL + "/healthz")

	systemctlPath := filepath.Join(tmp, "systemctl")
	systemctlScript := "#!/usr/bin/env bash\nset -euo pipefail\n" +
		"if [[ \"$1\" == \"--user\" && \"$2\" == \"is-active\" ]]; then\n" +
		"  echo \"inactive\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [[ \"$1\" == \"--user\" && \"$2\" == \"restart\" ]]; then\n" +
		"  echo \"restart failed\" >&2\n" +
		"  exit 1\n" +
		"fi\n"
	if err := os.WriteFile(systemctlPath, []byte(systemctlScript), 0o755); err != nil {
		t.Fatalf("write fake systemctl: %v", err)
	}

	businessConversationID := "lead-worker-self-heal:bid-all"
	opsConversationID := "ops-self-heal-conv"
	businessCh := make(chan string, 1)
	opsCh := make(chan string, 1)
	registerConversationProgressNotifier(businessConversationID, func(_ context.Context, message string) error {
		businessCh <- message
		return nil
	})
	registerConversationProgressNotifier(opsConversationID, func(_ context.Context, message string) error {
		opsCh <- message
		return nil
	})

	now := time.Now().UTC()
	if err := ensureTaskTrackingScaffold(workspace, now); err != nil {
		t.Fatalf("ensure task tracking scaffold: %v", err)
	}
	if err := updateWorkspaceTaskState(workspace, now, map[string]interface{}{
		"service_health_urls": map[string]string{
			"clawx-bid-all": healthURL,
		},
		"last_task_control_service": "clawx-bid-all",
	}); err != nil {
		t.Fatalf("seed workspace state: %v", err)
	}

	t.Setenv("PATH", fmt.Sprintf("%s:%s", tmp, os.Getenv("PATH")))
	t.Setenv("CLAWX_RUNTIME_SERVICE_ALLOWLIST", "clawx-*")
	t.Setenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_ENABLED", "1")
	t.Setenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_TICK", "1ms")
	t.Setenv("CLAWX_RUNTIME_TASK_CONTROL_STATUS_SELF_HEAL", "1")
	t.Setenv("CLAWX_RUNTIME_TASK_CONTROL_SELF_HEAL_FAILURE_THRESHOLD", "3")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_DISABLED", "0")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_MIN_INTERVAL", "0")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_TIMEOUT", "1s")
	t.Setenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_ALERT_CONVERSATIONS", opsConversationID)
	t.Setenv("CLAWX_TASK_CONTROL_SELF_HEAL_ALERT_WINDOW", "10m")

	runtime := agentRuntime{
		agentID: "bid-all",
		cwd:     workspace,
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{tmp},
			DefaultCWD:   workspace,
		},
	}
	loop, err := newLeadWorkerLoop(runtime, workspace)
	if err != nil {
		t.Fatalf("new lead worker loop: %v", err)
	}
	t.Cleanup(loop.close)

	if err := loop.runCycle(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("run cycle: %v", err)
	}

	select {
	case message := <-opsCh:
		if !strings.Contains(message, "self_heal_alert_level=warning") {
			t.Fatalf("expected warning alert in ops channel, got: %s", message)
		}
		if !strings.Contains(message, "self_heal_alert_reason=health_down") {
			t.Fatalf("expected warning alert reason in ops channel, got: %s", message)
		}
		if !strings.Contains(message, "playbook_1=先复测健康探针：curl -fsS -m 5 "+healthURL) {
			t.Fatalf("expected warning playbook step in ops alert, got: %s", message)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected warning alert notification in ops channel")
	}

	time.Sleep(80 * time.Millisecond)
	select {
	case message := <-businessCh:
		t.Fatalf("expected no warning alert in business channel, got: %s", message)
	default:
	}
}

func TestResolveLeadWorkerSelfHealAlertConversationIDs(t *testing.T) {
	t.Setenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_ALERT_CONVERSATIONS", "ops-a, ops-b;ops-a\nops-c")
	t.Setenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_ALERT_CONVERSATION", "")
	got := resolveLeadWorkerSelfHealAlertConversationIDs()
	expected := []string{"ops-a", "ops-b", "ops-c"}
	if len(got) != len(expected) {
		t.Fatalf("unexpected alert conversation count: got=%v expected=%v", got, expected)
	}
	for idx, item := range expected {
		if got[idx] != item {
			t.Fatalf("unexpected alert conversation at index %d: got=%q expected=%q", idx, got[idx], item)
		}
	}
}

func TestShouldSuppressLeadWorkerSelfHealAlertTransition(t *testing.T) {
	now := time.Now().UTC()
	minInterval := 2 * time.Minute
	mergeWindow := 5 * time.Minute

	if !shouldSuppressLeadWorkerSelfHealAlertTransition(
		taskControlSelfHealAlert{Level: "warning", State: "failed"},
		"warning",
		"failed",
		"health_down",
		"health_down",
		now.Add(-30*time.Second),
		now,
		minInterval,
		mergeWindow,
	) {
		t.Fatalf("expected warning alert suppressed within min interval")
	}

	if shouldSuppressLeadWorkerSelfHealAlertTransition(
		taskControlSelfHealAlert{Level: "critical", State: "throttled"},
		"warning",
		"failed",
		"health_down",
		"health_down",
		now.Add(-30*time.Second),
		now,
		minInterval,
		mergeWindow,
	) {
		t.Fatalf("expected warning->critical escalation to bypass suppression")
	}

	if shouldSuppressLeadWorkerSelfHealAlertTransition(
		taskControlSelfHealAlert{Level: "ok", State: "not_needed"},
		"critical",
		"throttled",
		"health_down",
		"",
		now.Add(-30*time.Second),
		now,
		minInterval,
		mergeWindow,
	) {
		t.Fatalf("expected recovery alert not suppressed by interval gate")
	}

	if shouldSuppressLeadWorkerSelfHealAlertTransition(
		taskControlSelfHealAlert{Level: "warning", State: "failed"},
		"warning",
		"failed",
		"service_not_active",
		"release_changed",
		now.Add(-30*time.Second),
		now,
		minInterval,
		mergeWindow,
	) {
		t.Fatalf("expected warning alert reason transition to bypass suppression")
	}
}

func TestResolveLeadWorkerSelfHealAlertDurationConfig(t *testing.T) {
	t.Setenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_ALERT_MIN_INTERVAL", "90s")
	t.Setenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_ALERT_MERGE_WINDOW", "240")
	if got := resolveLeadWorkerSelfHealAlertMinInterval(); got != 90*time.Second {
		t.Fatalf("unexpected min interval: %s", got)
	}
	if got := resolveLeadWorkerSelfHealAlertMergeWindow(); got != 240*time.Second {
		t.Fatalf("unexpected merge window: %s", got)
	}
}

func TestBuildLeadWorkerSelfHealAlertPlaybook(t *testing.T) {
	critical := buildLeadWorkerSelfHealAlertPlaybook("clawx-bid-all", "critical", "throttled", "health_down", "http://127.0.0.1:19080/healthz")
	if len(critical) < 3 {
		t.Fatalf("expected critical playbook with 3 steps, got: %v", critical)
	}
	if !strings.Contains(critical[0], "curl -fsS -m 5 http://127.0.0.1:19080/healthz") {
		t.Fatalf("unexpected critical step 1: %s", critical[0])
	}
	if !strings.Contains(critical[len(critical)-1], "runtime.task.control operation=ensure_running service=clawx-bid-all") {
		t.Fatalf("unexpected critical last step: %s", critical[len(critical)-1])
	}

	recovery := buildLeadWorkerSelfHealAlertPlaybook("clawx-bid-all", "ok", "not_needed", "", "")
	if len(recovery) != 1 {
		t.Fatalf("expected recovery playbook with single step, got: %v", recovery)
	}
	if !strings.Contains(recovery[0], "当前已恢复") {
		t.Fatalf("unexpected recovery playbook: %s", recovery[0])
	}

	release := buildLeadWorkerSelfHealAlertPlaybook("clawx-bid-all", "warning", "failed", "release_changed", "")
	if len(release) < 2 {
		t.Fatalf("expected release_changed playbook with 2 steps, got: %v", release)
	}
	if !strings.Contains(release[0], "runtime.release.status operation=current service=clawx-bid-all") {
		t.Fatalf("unexpected release_changed step 1: %s", release[0])
	}

	serviceInactive := buildLeadWorkerSelfHealAlertPlaybook("clawx-bid-all", "warning", "failed", "service_not_active", "")
	if len(serviceInactive) < 2 {
		t.Fatalf("expected service_not_active playbook with 2 steps, got: %v", serviceInactive)
	}
	if !strings.Contains(serviceInactive[0], "systemctl --user status clawx-bid-all.service --no-pager") {
		t.Fatalf("unexpected service_not_active step 1: %s", serviceInactive[0])
	}
}

func TestLeadWorkerLoopRunCycleAttestationIncludesDecisionContext(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	resetExecutionGoalStoreForTest(t, home)

	workspace := filepath.Join(tmp, "workspace")
	bootstrap := runtimeorchestrator.NewService()
	if _, err := bootstrap.Bootstrap(runtimeorchestrator.BootstrapOptions{
		WorkspaceRoot: workspace,
		AgentID:       "bid-all",
		WorkerRoles:   []string{"executor"},
	}); err != nil {
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
	loop, err := newLeadWorkerLoop(runtime, workspace)
	if err != nil {
		t.Fatalf("new lead worker loop: %v", err)
	}
	t.Cleanup(loop.close)

	conversationID := "conv-lead-loop-decision-context"
	if _, err := loop.queue.Enqueue(runtimeorchestrator.RuntimeTask{
		Source: "runtime.exec",
		Intent: "runtime.exec",
		Payload: map[string]interface{}{
			"cmd":                                "echo worker-decision-ok > worker_decision_result.txt",
			"cwd":                                workspace,
			"conversation":                       conversationID,
			"parent_task_id":                     "task-parent-decision",
			"reason":                             "runtime_exec_decision_build_continue_fallback",
			"fallback_source":                    "workspace",
			"runtime_exec_decision_mode":         "build.continue",
			"runtime_exec_decision_apply_source": "persisted_state",
			"runtime_exec_decision_lock_source":  "user_phrase",
		},
		Status: runtimeorchestrator.TaskQueued,
	}); err != nil {
		t.Fatalf("enqueue task: %v", err)
	}

	if err := loop.runCycle(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("run cycle: %v", err)
	}

	items, err := loop.queue.List()
	if err != nil {
		t.Fatalf("queue list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one task, got: %d", len(items))
	}
	if items[0].Status != runtimeorchestrator.TaskSucceeded {
		t.Fatalf("expected task succeeded, got: %s", items[0].Status)
	}
	execID := readRuntimeTaskPayloadString(items[0].Payload, "last_exec_id")
	if strings.TrimSpace(execID) == "" {
		t.Fatalf("expected last_exec_id persisted to task payload, payload=%+v", items[0].Payload)
	}

	records := listRecentRuntimeExecAttestations(conversationID, 4)
	if len(records) == 0 {
		t.Fatalf("expected attestation records for conversation=%s", conversationID)
	}
	var matched runtimeExecAttestationRecord
	found := false
	for _, rec := range records {
		if strings.TrimSpace(rec.ExecID) != strings.TrimSpace(execID) {
			continue
		}
		matched = rec
		found = true
		break
	}
	if !found {
		t.Fatalf("expected exec_id=%s in attestation records=%+v", execID, records)
	}
	if strings.TrimSpace(matched.PlanReason) != "runtime_lead_worker_dispatch" {
		t.Fatalf("expected lead worker plan reason, got: %+v", matched)
	}
	if strings.TrimSpace(matched.StepReason) != "runtime_exec_decision_build_continue_fallback" {
		t.Fatalf("expected lead worker step reason from queue payload, got: %+v", matched)
	}
	if strings.TrimSpace(matched.RuntimeExecDecisionMode) != "build.continue" {
		t.Fatalf("expected decision mode propagated from queue payload, got: %+v", matched)
	}
	if strings.TrimSpace(matched.RuntimeExecDecisionSource) != "user_phrase" {
		t.Fatalf("expected decision source propagated from queue payload, got: %+v", matched)
	}
	if strings.TrimSpace(matched.RuntimeExecDecisionApplySource) != "persisted_state" {
		t.Fatalf("expected decision apply source propagated from queue payload, got: %+v", matched)
	}
	if strings.TrimSpace(matched.RuntimeExecDecisionLockSource) != "user_phrase" {
		t.Fatalf("expected decision lock source propagated from queue payload, got: %+v", matched)
	}
	if strings.TrimSpace(matched.RuntimeExecDecisionFallbackSource) != "workspace" {
		t.Fatalf("expected fallback source propagated from queue payload, got: %+v", matched)
	}
}
