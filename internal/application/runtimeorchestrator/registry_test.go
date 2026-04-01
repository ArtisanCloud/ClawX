package runtimeorchestrator

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRegistryHeartbeatAndStaleDetection(t *testing.T) {
	tmp := t.TempDir()
	reg := NewRegistry(
		filepath.Join(tmp, "worker_states.json"),
		filepath.Join(tmp, "heartbeats.json"),
	)

	base := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	if err := reg.ReportHeartbeat("w-executor-1", base); err != nil {
		t.Fatalf("report heartbeat: %v", err)
	}
	if err := reg.UpdateWorkerState("w-executor-1", "busy", "t-1", "executor", base.Add(2*time.Minute)); err != nil {
		t.Fatalf("update worker state: %v", err)
	}

	workers, err := reg.Workers()
	if err != nil {
		t.Fatalf("workers: %v", err)
	}
	if len(workers) != 1 {
		t.Fatalf("unexpected worker len: %d", len(workers))
	}
	if workers[0].Status != "busy" {
		t.Fatalf("unexpected worker status: %s", workers[0].Status)
	}
	if workers[0].CurrentTaskID != "t-1" {
		t.Fatalf("unexpected current task: %s", workers[0].CurrentTaskID)
	}

	stale, err := reg.StaleWorkers(base.Add(35*time.Minute), 30*time.Minute)
	if err != nil {
		t.Fatalf("stale workers: %v", err)
	}
	if len(stale) != 1 || stale[0] != "w-executor-1" {
		t.Fatalf("unexpected stale workers: %+v", stale)
	}
}

func TestRegistryIdleWorkers(t *testing.T) {
	tmp := t.TempDir()
	reg := NewRegistry(
		filepath.Join(tmp, "worker_states.json"),
		filepath.Join(tmp, "heartbeats.json"),
	)
	base := time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)
	if err := reg.UpdateWorkerState("w-executor-1", "idle", "", "executor", base); err != nil {
		t.Fatalf("seed worker1: %v", err)
	}
	if err := reg.UpdateWorkerState("w-reviewer-2", "online", "", "reviewer", base); err != nil {
		t.Fatalf("seed worker2: %v", err)
	}
	if err := reg.UpdateWorkerState("w-planner-3", "busy", "t-1", "planner", base); err != nil {
		t.Fatalf("seed worker3: %v", err)
	}

	idle, err := reg.IdleWorkers()
	if err != nil {
		t.Fatalf("idle workers: %v", err)
	}
	if len(idle) != 2 {
		t.Fatalf("expected 2 idle workers, got=%d", len(idle))
	}
	if idle[0].WorkerID != "w-executor-1" || idle[1].WorkerID != "w-reviewer-2" {
		t.Fatalf("unexpected idle worker order: %+v", idle)
	}
}
