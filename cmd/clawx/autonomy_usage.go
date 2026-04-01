package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"clawx/internal/application/autonomy"
	"clawx/internal/application/service"
	"clawx/internal/infrastructure/config"
	stdlogging "clawx/internal/infrastructure/logging"
)

var autonomyLogMu sync.Mutex
var autonomyRecorder *stdlogging.AutonomyRecorder

func appendAutonomyAuditRecord(record stdlogging.AutonomyAuditRecord) error {
	autonomyLogMu.Lock()
	defer autonomyLogMu.Unlock()
	recorder, err := ensureAutonomyRecorderLocked()
	if err != nil {
		return err
	}
	return recorder.Append(record)
}

func ensureAutonomyRecorderLocked() (*stdlogging.AutonomyRecorder, error) {
	if autonomyRecorder != nil {
		return autonomyRecorder, nil
	}
	maxMB := parsePositiveIntEnv("CLAWX_AUTONOMY_LOG_MAX_MB", 20)
	maxBackups := parsePositiveIntEnv("CLAWX_AUTONOMY_LOG_MAX_BACKUPS", 5)
	logPath := strings.TrimSpace(os.Getenv("CLAWX_AUTONOMY_LOG_FILE"))
	if logPath == "" {
		logPath = filepath.Join(config.StateDir(), "logs", "autonomy.jsonl")
	}
	recorder, err := stdlogging.NewAutonomyRecorder(logPath, int64(maxMB)*1024*1024, maxBackups)
	if err != nil {
		return nil, err
	}
	autonomyRecorder = recorder
	return autonomyRecorder, nil
}

func emitAutonomyFlow(
	channel string,
	instanceID string,
	runtime agentRuntime,
	decision service.Decision,
	phase string,
	status string,
	classification autonomy.FailureClassification,
	detail string,
	execErr error,
) {
	phase = strings.TrimSpace(strings.ToLower(phase))
	status = strings.TrimSpace(strings.ToLower(status))
	record := stdlogging.AutonomyAuditRecord{
		Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
		Channel:        strings.TrimSpace(channel),
		Instance:       strings.TrimSpace(instanceID),
		AgentID:        strings.TrimSpace(runtime.agentID),
		ConversationID: strings.TrimSpace(decision.ConversationID),
		ProjectID:      strings.TrimSpace(decision.ProjectID),
		Phase:          phase,
		Status:         status,
		FailureClass:   string(classification.Class),
		Recoverable:    classification.Recoverable,
		Reason:         strings.TrimSpace(classification.Reason),
		Detail:         strings.TrimSpace(detail),
	}
	if execErr != nil {
		record.Error = tracePreview(execErr.Error(), 240)
	}
	_ = appendAutonomyAuditRecord(record)
	emitTrace("autonomy_flow", map[string]any{
		"channel":         record.Channel,
		"instance":        record.Instance,
		"agent_id":        record.AgentID,
		"conversation_id": record.ConversationID,
		"project_id":      record.ProjectID,
		"phase":           record.Phase,
		"status":          record.Status,
		"failure_class":   record.FailureClass,
		"recoverable":     record.Recoverable,
		"reason":          record.Reason,
		"detail":          record.Detail,
		"error":           record.Error,
	})
}
