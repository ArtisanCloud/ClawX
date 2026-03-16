package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"clawx/internal/domain/execution"
)

var codexLogMu sync.Mutex

type codexRunTrace struct {
	Timestamp        time.Time             `json:"timestamp"`
	SessionID        string                `json:"session_id"`
	BackendSessionID string                `json:"backend_session_id"`
	CWD              string                `json:"cwd"`
	Command          string                `json:"command"`
	Args             []string              `json:"args"`
	State            execution.ResultState `json:"state"`
	DurationMS       int64                 `json:"duration_ms"`
	Output           string                `json:"output"`
	Stdout           string                `json:"stdout"`
	Stderr           string                `json:"stderr"`
	Error            string                `json:"error,omitempty"`
}

type traceIndexEntry struct {
	Timestamp        time.Time `json:"timestamp"`
	Kind             string    `json:"kind"`
	SessionID        string    `json:"session_id"`
	BackendSessionID string    `json:"backend_session_id"`
	LogFile          string    `json:"log_file"`
}

func appendCodexTrace(
	request execution.Request,
	command string,
	args []string,
	state execution.ResultState,
	startedAt time.Time,
	completedAt time.Time,
	backendSessionID string,
	output string,
	stdout string,
	stderr string,
	runErr error,
) error {
	logRoot := clawxLogsRoot()
	logFile := filepath.Join(logRoot, "codex", sanitizeFileSegment(request.SessionID)+".jsonl")

	trace := codexRunTrace{
		Timestamp:        completedAt.UTC(),
		SessionID:        strings.TrimSpace(request.SessionID),
		BackendSessionID: strings.TrimSpace(backendSessionID),
		CWD:              strings.TrimSpace(request.CWD),
		Command:          strings.TrimSpace(command),
		Args:             append([]string(nil), args...),
		State:            state,
		DurationMS:       completedAt.Sub(startedAt).Milliseconds(),
		Output:           output,
		Stdout:           stdout,
		Stderr:           stderr,
	}
	if runErr != nil {
		trace.Error = runErr.Error()
	}

	index := traceIndexEntry{
		Timestamp:        completedAt.UTC(),
		Kind:             "codex-run",
		SessionID:        trace.SessionID,
		BackendSessionID: trace.BackendSessionID,
		LogFile:          logFile,
	}

	codexLogMu.Lock()
	defer codexLogMu.Unlock()

	if err := appendJSONL(logFile, trace); err != nil {
		return err
	}
	return appendJSONL(filepath.Join(logRoot, "index.jsonl"), index)
}

func appendJSONL(path string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	body = append(body, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create log dir for %q: %w", path, err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file %q: %w", path, err)
	}
	defer f.Close()

	if _, err := f.Write(body); err != nil {
		return fmt.Errorf("append log file %q: %w", path, err)
	}
	return nil
}

func clawxLogsRoot() string {
	if explicit := strings.TrimSpace(os.Getenv("CLAWX_LOG_DIR")); explicit != "" {
		return explicit
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".clawx", "logs")
	}
	return filepath.Join(home, ".clawx", "logs")
}

func sanitizeFileSegment(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "unknown-session"
	}

	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "unknown-session"
	}
	return b.String()
}
