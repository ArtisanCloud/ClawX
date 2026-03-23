package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clawx/internal/domain/execution"
)

func TestAppendCodexTraceWritesSessionLogAndIndex(t *testing.T) {
	logRoot := t.TempDir()
	t.Setenv("CLAWX_LOG_DIR", logRoot)

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
		128,
		1024,
		256,
		1280,
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
	if !strings.Contains(string(body), `"prompt_cached_tokens":128`) {
		t.Fatalf("missing prompt cached token field: %s", string(body))
	}
	if !strings.Contains(string(body), `"prompt_tokens":1024`) {
		t.Fatalf("missing prompt token field: %s", string(body))
	}
	if !strings.Contains(string(body), `"completion_tokens":256`) {
		t.Fatalf("missing completion token field: %s", string(body))
	}
	if !strings.Contains(string(body), `"total_tokens":1280`) {
		t.Fatalf("missing total token field: %s", string(body))
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
