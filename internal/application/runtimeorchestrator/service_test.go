package runtimeorchestrator

import (
	"os"
	"path/filepath"
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
