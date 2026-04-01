package logging

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
)

type AutonomyAuditRecord struct {
	Timestamp      string `json:"timestamp"`
	Channel        string `json:"channel,omitempty"`
	Instance       string `json:"instance,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	ProjectID      string `json:"project_id,omitempty"`
	Phase          string `json:"phase,omitempty"`
	Status         string `json:"status,omitempty"`
	FailureClass   string `json:"failure_class,omitempty"`
	Recoverable    bool   `json:"recoverable,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Error          string `json:"error,omitempty"`
	Detail         string `json:"detail,omitempty"`
}

type AutonomyRecorder struct {
	mu     sync.Mutex
	writer *RotatingWriter
}

func NewAutonomyRecorder(path string, maxBytes int64, maxBackups int) (*AutonomyRecorder, error) {
	writer, err := NewRotatingWriter(path, maxBytes, maxBackups)
	if err != nil {
		return nil, err
	}
	return &AutonomyRecorder{writer: writer}, nil
}

func (r *AutonomyRecorder) Append(record AutonomyAuditRecord) error {
	if r == nil || r.writer == nil {
		return fmt.Errorf("autonomy recorder is not initialized")
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

func (r *AutonomyRecorder) Close() error {
	if r == nil || r.writer == nil {
		return nil
	}
	return r.writer.Close()
}

type AutonomyMetrics struct {
	Total       int
	ByPhase     map[string]int
	ByStatus    map[string]int
	ByClass     map[string]int
	Escalations int
}

func AggregateAutonomyMetrics(reader io.Reader) (AutonomyMetrics, error) {
	metrics := AutonomyMetrics{
		ByPhase:  make(map[string]int),
		ByStatus: make(map[string]int),
		ByClass:  make(map[string]int),
	}
	scanner := bufio.NewScanner(reader)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record AutonomyAuditRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return AutonomyMetrics{}, fmt.Errorf("parse autonomy line %d: %w", lineNo, err)
		}
		metrics.Total++
		phase := nonEmptyOrDefault(record.Phase, "unknown")
		status := nonEmptyOrDefault(record.Status, "unknown")
		class := nonEmptyOrDefault(record.FailureClass, "unknown")
		metrics.ByPhase[phase]++
		metrics.ByStatus[status]++
		metrics.ByClass[class]++
		if status == "escalated" || status == "prompted" {
			metrics.Escalations++
		}
	}
	if err := scanner.Err(); err != nil {
		return AutonomyMetrics{}, err
	}
	return metrics, nil
}

func RenderAutonomyMetricsReport(metrics AutonomyMetrics) string {
	var b strings.Builder
	b.WriteString("Autonomy Metrics\n")
	b.WriteString(fmt.Sprintf("total=%d escalations=%d\n", metrics.Total, metrics.Escalations))
	b.WriteString(renderAutonomyCountSection("by_phase", metrics.ByPhase))
	b.WriteString(renderAutonomyCountSection("by_status", metrics.ByStatus))
	b.WriteString(renderAutonomyCountSection("by_failure_class", metrics.ByClass))
	return strings.TrimSpace(b.String())
}

func renderAutonomyCountSection(name string, values map[string]int) string {
	if len(values) == 0 {
		return fmt.Sprintf("%s: (empty)\n", name)
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(name)
	b.WriteString(":\n")
	for _, key := range keys {
		b.WriteString(fmt.Sprintf("- %s count=%d\n", key, values[key]))
	}
	return b.String()
}
