package runtimeorchestrator

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDispatcherIdleFirstAndStickyResource(t *testing.T) {
	tmp := t.TempDir()
	svc := NewService()
	boot, err := svc.Bootstrap(BootstrapOptions{
		WorkspaceRoot: tmp,
		AgentID:       "bid-all",
		WorkerRoles:   []string{"planner", "executor", "reviewer"},
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	queue := NewQueue(boot.TasksFile)
	registry := NewRegistry(boot.WorkerStatesFile, boot.HeartbeatsFile)
	dispatcher := NewDispatcher(queue, registry, boot.DispatchStateFile)

	task1, err := queue.Enqueue(RuntimeTask{
		Source: "runtime.exec",
		Intent: "runtime.exec",
		Payload: map[string]interface{}{
			"resource_key": "source-alpha",
		},
		Status: TaskQueued,
	})
	if err != nil {
		t.Fatalf("enqueue task1: %v", err)
	}
	decisions, err := dispatcher.DispatchQueued(time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), 1)
	if err != nil {
		t.Fatalf("dispatch task1: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 dispatch decision, got=%d", len(decisions))
	}
	if decisions[0].TaskID != task1.TaskID {
		t.Fatalf("unexpected dispatched task id: %s", decisions[0].TaskID)
	}
	if decisions[0].Reason != "idle_first" {
		t.Fatalf("expected idle_first on first dispatch, got=%s", decisions[0].Reason)
	}
	firstWorkerID := decisions[0].WorkerID

	if _, ok, err := queue.UpdateStatus(task1.TaskID, TaskSucceeded); err != nil {
		t.Fatalf("complete task1: %v", err)
	} else if !ok {
		t.Fatalf("expected task1 status update")
	}
	if err := registry.UpdateWorkerState(decisions[0].WorkerID, "idle", "", "executor", time.Date(2026, 3, 31, 0, 1, 0, 0, time.UTC)); err != nil {
		t.Fatalf("set worker idle: %v", err)
	}

	task2, err := queue.Enqueue(RuntimeTask{
		Source: "runtime.exec",
		Intent: "runtime.exec",
		Payload: map[string]interface{}{
			"resource_key": "source-alpha",
		},
		Status: TaskQueued,
	})
	if err != nil {
		t.Fatalf("enqueue task2: %v", err)
	}
	decisions, err = dispatcher.DispatchQueued(time.Date(2026, 3, 31, 0, 2, 0, 0, time.UTC), 1)
	if err != nil {
		t.Fatalf("dispatch task2: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 dispatch decision for task2, got=%d", len(decisions))
	}
	if decisions[0].TaskID != task2.TaskID {
		t.Fatalf("unexpected task2 dispatched id: %s", decisions[0].TaskID)
	}
	if decisions[0].Reason != "sticky_resource" {
		t.Fatalf("expected sticky_resource on second dispatch, got=%s", decisions[0].Reason)
	}
	if decisions[0].WorkerID != firstWorkerID {
		t.Fatalf("expected sticky worker %s, got=%s", firstWorkerID, decisions[0].WorkerID)
	}
}

func TestPickDispatchWorkerStickyFallbackToIdle(t *testing.T) {
	idle := []workerState{
		{WorkerID: "w-executor-2", Role: "executor", Status: "idle"},
		{WorkerID: "w-reviewer-3", Role: "reviewer", Status: "idle"},
	}
	worker, reason := pickDispatchWorker(idle, map[string]string{"res-a": "w-missing"}, "res-a")
	if worker.WorkerID != "w-executor-2" {
		t.Fatalf("expected fallback to first idle worker, got=%s", worker.WorkerID)
	}
	if reason != "idle_first" {
		t.Fatalf("expected idle_first fallback, got=%s", reason)
	}
}

func TestDispatcherStateFileAutoCreate(t *testing.T) {
	tmp := t.TempDir()
	queue := NewQueue(filepath.Join(tmp, "tasks.jsonl"))
	registry := NewRegistry(filepath.Join(tmp, "worker_states.json"), filepath.Join(tmp, "heartbeats.json"))
	dispatcher := NewDispatcher(queue, registry, filepath.Join(tmp, "dispatch_state.json"))
	if _, err := dispatcher.DispatchQueued(time.Now().UTC(), 1); err != nil {
		t.Fatalf("dispatch queued with empty queue should not fail: %v", err)
	}
}
