package logging

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type RuntimeOrchestratorAuditRecord struct {
	Timestamp string `json:"timestamp"`
	AgentID   string `json:"agent_id,omitempty"`
	Event     string `json:"event,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	WorkerID  string `json:"worker_id,omitempty"`
	Status    string `json:"status,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

type RuntimeOrchestratorRecorder struct {
	mu     sync.Mutex
	writer *RotatingWriter
}

func NewRuntimeOrchestratorRecorder(path string, maxBytes int64, maxBackups int) (*RuntimeOrchestratorRecorder, error) {
	writer, err := NewRotatingWriter(path, maxBytes, maxBackups)
	if err != nil {
		return nil, err
	}
	return &RuntimeOrchestratorRecorder{writer: writer}, nil
}

func (r *RuntimeOrchestratorRecorder) Append(record RuntimeOrchestratorAuditRecord) error {
	if r == nil || r.writer == nil {
		return fmt.Errorf("runtime orchestrator recorder is not initialized")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.TrimSpace(record.Timestamp) == "" {
		record.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	_, err = r.writer.Write(body)
	return err
}

func (r *RuntimeOrchestratorRecorder) Close() error {
	if r == nil || r.writer == nil {
		return nil
	}
	return r.writer.Close()
}
