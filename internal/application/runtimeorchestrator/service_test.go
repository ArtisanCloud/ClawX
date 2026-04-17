package runtimeorchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapCreatesRuntimeStateFiles(t *testing.T) {
	tmp := t.TempDir()
	svc := NewService()
	res, err := svc.Bootstrap(BootstrapOptions{
		WorkspaceRoot: tmp,
		AgentID:       "bid-all",
		WorkerRoles:   []string{"planner", "executor", "reviewer"},
	})
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	if res.WorkerCount != 3 {
		t.Fatalf("unexpected worker count: %d", res.WorkerCount)
	}
	for _, p := range []string{
		res.RuntimeDir,
		res.TasksFile,
		res.WorkerStatesFile,
		res.HeartbeatsFile,
		res.RuntimeMetaFile,
		res.DispatchStateFile,
	} {
		if _, statErr := os.Stat(p); statErr != nil {
			t.Fatalf("expected path exists: %s err=%v", p, statErr)
		}
	}
}

func TestBootstrapUsesDefaultWorkerRoles(t *testing.T) {
	tmp := t.TempDir()
	svc := NewService()
	res, err := svc.Bootstrap(BootstrapOptions{
		WorkspaceRoot: filepath.Clean(tmp),
		AgentID:       "main",
	})
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	if res.WorkerCount != defaultWorkerCount {
		t.Fatalf("unexpected default worker count: %d", res.WorkerCount)
	}
}

func TestBootstrapReinitializeIsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	svc := NewService()
	first, err := svc.Bootstrap(BootstrapOptions{
		WorkspaceRoot: tmp,
		AgentID:       "bid-all",
		WorkerRoles:   []string{"planner", "executor", "reviewer"},
	})
	if err != nil {
		t.Fatalf("first bootstrap failed: %v", err)
	}

	seedTask := `{"task_id":"t-1","status":"queued"}` + "\n"
	if err := os.WriteFile(first.TasksFile, []byte(seedTask), 0o644); err != nil {
		t.Fatalf("seed tasks file: %v", err)
	}

	second, err := svc.Bootstrap(BootstrapOptions{
		WorkspaceRoot: tmp,
		AgentID:       "bid-all",
		WorkerRoles:   []string{"planner", "executor", "reviewer"},
	})
	if err != nil {
		t.Fatalf("second bootstrap failed: %v", err)
	}

	if first.WorkerCount != second.WorkerCount {
		t.Fatalf("expected same worker count after re-bootstrap: first=%d second=%d", first.WorkerCount, second.WorkerCount)
	}
	if strings.Join(first.WorkerIDs, ",") != strings.Join(second.WorkerIDs, ",") {
		t.Fatalf("expected stable worker ids after re-bootstrap: first=%v second=%v", first.WorkerIDs, second.WorkerIDs)
	}

	body, err := os.ReadFile(second.TasksFile)
	if err != nil {
		t.Fatalf("read tasks file after re-bootstrap: %v", err)
	}
	if string(body) != seedTask {
		t.Fatalf("expected tasks file preserved after re-bootstrap, got: %q", string(body))
	}
}
