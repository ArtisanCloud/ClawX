package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"clawx/internal/infrastructure/config"
	stdlogging "clawx/internal/infrastructure/logging"
)

type runtimeExecAttestationRecord struct {
	Timestamp                         string `json:"timestamp"`
	ExecID                            string `json:"exec_id"`
	ConversationID                    string `json:"conversation_id,omitempty"`
	AgentID                           string `json:"agent_id,omitempty"`
	CWD                               string `json:"cwd,omitempty"`
	Command                           string `json:"command"`
	PlanReason                        string `json:"plan_reason,omitempty"`
	StepReason                        string `json:"step_reason,omitempty"`
	RuntimeExecDecisionMode           string `json:"runtime_exec_decision_mode,omitempty"`
	RuntimeExecDecisionSource         string `json:"runtime_exec_decision_source,omitempty"`
	RuntimeExecDecisionApplySource    string `json:"runtime_exec_decision_apply_source,omitempty"`
	RuntimeExecDecisionLockSource     string `json:"runtime_exec_decision_lock_source,omitempty"`
	RuntimeExecDecisionFallbackSource string `json:"runtime_exec_decision_fallback_source,omitempty"`
	ExitCode                          int    `json:"exit_code"`
	DurationMS                        int64  `json:"duration_ms"`
	Success                           bool   `json:"success"`
	OutputDigest                      string `json:"output_digest,omitempty"`
	OutputPreview                     string `json:"output_preview,omitempty"`
	ErrorSummary                      string `json:"error_summary,omitempty"`
}

var runtimeExecAttestationMu sync.Mutex
var runtimeExecAttestationWriter *stdlogging.RotatingWriter
var runtimeExecSeq uint64
var runtimeExecAttestationIndex = make(map[string]runtimeExecAttestationRecord)
var runtimeExecAttestationOrder []string

func newRuntimeExecID() string {
	seq := atomic.AddUint64(&runtimeExecSeq, 1)
	return fmt.Sprintf("rexec-%d-%d", time.Now().UTC().UnixNano(), seq)
}

func appendRuntimeExecAttestation(record runtimeExecAttestationRecord) error {
	runtimeExecAttestationMu.Lock()
	defer runtimeExecAttestationMu.Unlock()
	if strings.TrimSpace(record.Timestamp) == "" {
		record.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	writer, err := ensureRuntimeExecAttestationWriterLocked()
	if err != nil {
		return err
	}
	if _, err := writer.Write(body); err != nil {
		return err
	}
	if strings.TrimSpace(record.ExecID) != "" {
		runtimeExecAttestationIndex[record.ExecID] = record
		runtimeExecAttestationOrder = append(runtimeExecAttestationOrder, record.ExecID)
		if len(runtimeExecAttestationOrder) > 5000 {
			drop := len(runtimeExecAttestationOrder) - 5000
			runtimeExecAttestationOrder = runtimeExecAttestationOrder[drop:]
		}
	}
	return nil
}

func ensureRuntimeExecAttestationWriterLocked() (*stdlogging.RotatingWriter, error) {
	if runtimeExecAttestationWriter != nil {
		return runtimeExecAttestationWriter, nil
	}
	maxMB := parsePositiveIntEnv("CLAWX_RUNTIME_EXEC_LOG_MAX_MB", 20)
	maxBackups := parsePositiveIntEnv("CLAWX_RUNTIME_EXEC_LOG_MAX_BACKUPS", 5)
	logPath := strings.TrimSpace(os.Getenv("CLAWX_RUNTIME_EXEC_LOG_FILE"))
	if logPath == "" {
		logPath = filepath.Join(config.StateDir(), "logs", "runtime_exec.jsonl")
	}
	writer, err := stdlogging.NewRotatingWriter(logPath, int64(maxMB)*1024*1024, maxBackups)
	if err != nil {
		return nil, err
	}
	runtimeExecAttestationWriter = writer
	return runtimeExecAttestationWriter, nil
}

func hasRuntimeExecAttestation(conversationID string, execIDs []string) bool {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" || len(execIDs) == 0 {
		return false
	}
	runtimeExecAttestationMu.Lock()
	defer runtimeExecAttestationMu.Unlock()
	for _, id := range execIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return false
		}
		record, ok := runtimeExecAttestationIndex[id]
		if !ok {
			return false
		}
		if strings.TrimSpace(record.ConversationID) != conversationID {
			return false
		}
	}
	return true
}

func listRecentRuntimeExecAttestations(conversationID string, limit int) []runtimeExecAttestationRecord {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil
	}
	if limit <= 0 {
		limit = 3
	}
	runtimeExecAttestationMu.Lock()
	defer runtimeExecAttestationMu.Unlock()
	out := make([]runtimeExecAttestationRecord, 0, limit)
	for i := len(runtimeExecAttestationOrder) - 1; i >= 0; i-- {
		id := strings.TrimSpace(runtimeExecAttestationOrder[i])
		if id == "" {
			continue
		}
		record, ok := runtimeExecAttestationIndex[id]
		if !ok {
			continue
		}
		if strings.TrimSpace(record.ConversationID) != conversationID {
			continue
		}
		out = append(out, record)
		if len(out) >= limit {
			break
		}
	}
	// reverse to chronological order
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func listRuntimeExecAttestationsByIDs(execIDs []string) []runtimeExecAttestationRecord {
	if len(execIDs) == 0 {
		return nil
	}
	runtimeExecAttestationMu.Lock()
	defer runtimeExecAttestationMu.Unlock()

	out := make([]runtimeExecAttestationRecord, 0, len(execIDs))
	seen := make(map[string]struct{}, len(execIDs))
	for _, raw := range execIDs {
		execID := strings.TrimSpace(raw)
		if execID == "" {
			continue
		}
		if _, ok := seen[execID]; ok {
			continue
		}
		seen[execID] = struct{}{}
		record, ok := runtimeExecAttestationIndex[execID]
		if !ok {
			continue
		}
		out = append(out, record)
	}
	return out
}

func digestOutput(output string) string {
	sum := sha256.Sum256([]byte(output))
	return hex.EncodeToString(sum[:])
}
