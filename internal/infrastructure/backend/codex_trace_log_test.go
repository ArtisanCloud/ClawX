package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"synapsex/internal/domain/execution"
)

func TestAppendCodexTraceWritesSessionLogAndIndex(t *testing.T) {
	logRoot := t.TempDir()
	t.Setenv("SYNAPSEX_LOG_DIR", logRoot)

	req := execution.Request{
		SessionID:        "sess-abc-1",
		BackendSessionID: "thread-xyz",
		CWD:              "/tmp/project",
		Input:            "hello",
	}

	startedAt := time.Now().UTC().Add(-2 * time.Second)
	completedAt := time.Now().UTC()
	err := appendCodexTrace(
		req,
		"codex",
		[]string{"exec", "--json"},
		execution.ResultSuccess,
		startedAt,
		completedAt,
		"thread-xyz",
		"ok",
		`{"type":"thread.started","thread_id":"thread-xyz"}`,
		"",
		nil,
	)
	if err != nil {
		t.Fatalf("appendCodexTrace: %v", err)
	}

	sessionLog := filepath.Join(logRoot, "codex", "sess-abc-1.jsonl")
	body, err := os.ReadFile(sessionLog)
	if err != nil {
		t.Fatalf("read session log: %v", err)
	}
	if !strings.Contains(string(body), `"session_id":"sess-abc-1"`) {
		t.Fatalf("unexpected session log body: %s", string(body))
	}

	indexLog := filepath.Join(logRoot, "index.jsonl")
	indexBody, err := os.ReadFile(indexLog)
	if err != nil {
		t.Fatalf("read index log: %v", err)
	}
	if !strings.Contains(string(indexBody), `"log_file":"`+sessionLog+`"`) {
		t.Fatalf("unexpected index log body: %s", string(indexBody))
	}
}
