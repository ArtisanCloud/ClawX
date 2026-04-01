package runtimeorchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clawx/internal/infrastructure/logging"
)

func TestRecoveryEngineRequeueAndReassignFromStaleWorker(t *testing.T) {
	tmp := t.TempDir()
	svc := NewService()
	boot, err := svc.Bootstrap(BootstrapOptions{
		WorkspaceRoot: tmp,
		AgentID:       "bid-all",
		WorkerRoles:   []string{"executor", "reviewer"},
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	queue := NewQueue(boot.TasksFile)
	registry := NewRegistry(boot.WorkerStatesFile, boot.HeartbeatsFile)
	dispatcher := NewDispatcher(queue, registry, boot.DispatchStateFile)

	task, err := queue.Enqueue(RuntimeTask{
		Source: "runtime.exec",
		Intent: "runtime.exec",
		Payload: map[string]interface{}{
			"resource_key": "source-a",
		},
		Status: TaskQueued,
	})
	if err != nil {
		t.Fatalf("enqueue task: %v", err)
	}
	base := time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)
	decisions, err := dispatcher.DispatchQueued(base, 1)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected one dispatch decision, got=%d", len(decisions))
	}
	staleWorker := decisions[0].WorkerID
	if err := registry.UpdateWorkerState(staleWorker, "busy", task.TaskID, "executor", base.Add(-40*time.Minute)); err != nil {
		t.Fatalf("set stale worker state: %v", err)
	}
	for _, workerID := range []string{"w-executor-1", "w-reviewer-2"} {
		if workerID == staleWorker {
			continue
		}
		if err := registry.ReportHeartbeat(workerID, base); err != nil {
			t.Fatalf("report heartbeat for active worker %s: %v", workerID, err)
		}
		if err := registry.UpdateWorkerState(workerID, "idle", "", "reviewer", base); err != nil {
			t.Fatalf("set idle for active worker %s: %v", workerID, err)
		}
	}

	logPath := filepath.Join(tmp, "runtime_orchestrator.jsonl")
	recorder, err := logging.NewRuntimeOrchestratorRecorder(logPath, 1024*1024, 2)
	if err != nil {
		t.Fatalf("new recorder: %v", err)
	}
	defer recorder.Close()
	engine := NewRecoveryEngine(queue, registry, dispatcher, recorder, "bid-all")

	result, err := engine.Recover(base, 30*time.Minute, 1)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if len(result.StaleWorkers) == 0 {
		t.Fatalf("expected stale workers detected")
	}
	if len(result.RequeuedTaskIDs) != 1 || result.RequeuedTaskIDs[0] != task.TaskID {
		t.Fatalf("expected requeued task %s, got=%v", task.TaskID, result.RequeuedTaskIDs)
	}
	if len(result.ReassignedTaskIDs) != 1 || result.ReassignedTaskIDs[0] != task.TaskID {
		t.Fatalf("expected reassigned task %s, got=%v", task.TaskID, result.ReassignedTaskIDs)
	}
	items, err := queue.List()
	if err != nil {
		t.Fatalf("queue list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected single task in queue list, got=%d", len(items))
	}
	if items[0].Status != TaskRunning {
		t.Fatalf("expected recovered task running again, got=%s", items[0].Status)
	}
	if strings.TrimSpace(items[0].AssignedWorkerID) == "" || items[0].AssignedWorkerID == staleWorker {
		t.Fatalf("expected reassigned worker not stale worker, got=%s", items[0].AssignedWorkerID)
	}
	if items[0].Retry < 1 {
		t.Fatalf("expected retry increment after requeue, got=%d", items[0].Retry)
	}

	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read recovery log: %v", err)
	}
	content := string(body)
	if !strings.Contains(content, `"event":"task_requeue"`) || !strings.Contains(content, `"event":"task_reassign"`) {
		t.Fatalf("expected recovery audit events, got: %s", content)
	}
}
