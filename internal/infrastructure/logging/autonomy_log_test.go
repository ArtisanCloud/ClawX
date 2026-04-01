package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutonomyRecorderAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "autonomy.jsonl")
	recorder, err := NewAutonomyRecorder(path, 1024*1024, 2)
	if err != nil {
		t.Fatalf("new recorder: %v", err)
	}
	defer recorder.Close()

	if err := recorder.Append(AutonomyAuditRecord{
		Phase:        "classify",
		Status:       "failed",
		FailureClass: "network",
		Reason:       "network_unreachable_or_timeout",
	}); err != nil {
		t.Fatalf("append record: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read autonomy file: %v", err)
	}
	if !strings.Contains(string(body), `"phase":"classify"`) {
		t.Fatalf("expected phase in record, got: %s", string(body))
	}
}

func TestAggregateAutonomyMetrics(t *testing.T) {
	input := strings.Join([]string{
		`{"phase":"classify","status":"failed","failure_class":"network"}`,
		`{"phase":"attempt","status":"auto_attempted","failure_class":"network"}`,
		`{"phase":"escalate","status":"prompted","failure_class":"network"}`,
		`{"phase":"result","status":"escalated","failure_class":"network"}`,
	}, "\n")
	metrics, err := AggregateAutonomyMetrics(strings.NewReader(input))
	if err != nil {
		t.Fatalf("aggregate autonomy metrics: %v", err)
	}
	if metrics.Total != 4 {
		t.Fatalf("unexpected total: %d", metrics.Total)
	}
	if metrics.ByPhase["classify"] != 1 || metrics.ByStatus["prompted"] != 1 || metrics.ByClass["network"] != 4 {
		t.Fatalf("unexpected metrics buckets: %#v", metrics)
	}
	if metrics.Escalations != 2 {
		t.Fatalf("unexpected escalations: %d", metrics.Escalations)
	}
}
