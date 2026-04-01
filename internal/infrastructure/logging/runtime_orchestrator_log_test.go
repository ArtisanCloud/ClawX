package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeOrchestratorRecorderAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime_orchestrator.jsonl")
	recorder, err := NewRuntimeOrchestratorRecorder(path, 1024*1024, 2)
	if err != nil {
		t.Fatalf("new recorder: %v", err)
	}
	defer recorder.Close()

	if err := recorder.Append(RuntimeOrchestratorAuditRecord{
		AgentID:  "bid-all",
		Event:    "recovery_reassign",
		TaskID:   "t-1",
		WorkerID: "w-executor-2",
		Status:   "success",
	}); err != nil {
		t.Fatalf("append record: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	content := string(body)
	if !strings.Contains(content, `"event":"recovery_reassign"`) {
		t.Fatalf("expected event in log content, got: %s", content)
	}
}
