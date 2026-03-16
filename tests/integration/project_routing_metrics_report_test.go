package integration

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	phase5MetricsWindowDays    = 7
	phase5MetricsMinSamples    = 200
	phase5LatencyP95ThresholdM = 120.0
)

var phase5RateThresholds = map[string]float64{
	"SC-001": 95.0,
	"SC-002": 100.0,
	"SC-003": 95.0,
	"SC-004": 100.0,
	"SC-005": 100.0,
}

type phase5MetricEvent struct {
	Timestamp time.Time `json:"timestamp"`
	SCID      string    `json:"sc_id"`
	Success   *bool     `json:"success,omitempty"`
	LatencyMS float64   `json:"latency_ms,omitempty"`
}

type phase5MetricSummary struct {
	SCID           string
	Samples        int
	Successes      int
	Rate           float64
	RateThreshold  float64
	P95LatencyMS   float64
	LatencyGateMS  float64
	Passed         bool
	EvaluationNote string
}

type phase5MetricGateReport struct {
	Summaries   map[string]phase5MetricSummary
	AllPassed   bool
	FailedSCIDs []string
}

func TestPhase5ProjectRoutingMetricsReportGateWithSyntheticDataset(t *testing.T) {
	now := time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)
	events := make([]phase5MetricEvent, 0, 1700)
	events = append(events, buildPhase5RateEvents(now, "SC-001", 230, 220)...)
	events = append(events, buildPhase5RateEvents(now, "SC-002", 220, 220)...)
	events = append(events, buildPhase5RateEvents(now, "SC-003", 210, 205)...)
	events = append(events, buildPhase5RateEvents(now, "SC-004", 215, 215)...)
	events = append(events, buildPhase5RateEvents(now, "SC-005", 205, 205)...)
	events = append(events, buildPhase5LatencyEvents(now, "SC-006", 240, 78, 22)...)
	oldSuccess := true
	events = append(events, phase5MetricEvent{
		Timestamp: now.AddDate(0, 0, -(phase5MetricsWindowDays + 1)),
		SCID:      "SC-001",
		Success:   &oldSuccess,
	})

	report := evaluatePhase5MetricGate(events, now)
	if !report.AllPassed {
		t.Fatalf("expected synthetic dataset to pass gate, got failures: %v", report.FailedSCIDs)
	}

	for scID, summary := range report.Summaries {
		if summary.Samples < phase5MetricsMinSamples {
			t.Fatalf("%s sample count too small: %d", scID, summary.Samples)
		}
		if !summary.Passed {
			t.Fatalf("%s should pass: note=%s", scID, summary.EvaluationNote)
		}
	}
}

func TestPhase5ProjectRoutingMetricsReportFromJSONL(t *testing.T) {
	inputPath := strings.TrimSpace(os.Getenv("CLAWX_PHASE5_METRICS_JSONL"))
	if inputPath == "" {
		t.Skip("set CLAWX_PHASE5_METRICS_JSONL to collect SC-001~SC-006 metrics from real logs")
	}

	events, err := loadPhase5MetricEventsJSONL(inputPath)
	if err != nil {
		t.Fatalf("load metrics jsonl: %v", err)
	}

	now := time.Now().UTC()
	report := evaluatePhase5MetricGate(events, now)
	markdown := renderPhase5MetricGateMarkdown(report, now, inputPath)

	outputPath := strings.TrimSpace(os.Getenv("CLAWX_PHASE5_METRICS_REPORT"))
	if outputPath == "" {
		outputPath = filepath.Join("docs", "guides", "phase_5", "phase_5_metrics_report.md")
	}
	if err := os.WriteFile(outputPath, []byte(markdown), 0o644); err != nil {
		t.Fatalf("write metrics report: %v", err)
	}

	if !report.AllPassed {
		t.Fatalf("metrics gate failed, see report at %s (failed=%v)", outputPath, report.FailedSCIDs)
	}
}

func evaluatePhase5MetricGate(events []phase5MetricEvent, now time.Time) phase5MetricGateReport {
	windowStart := now.AddDate(0, 0, -phase5MetricsWindowDays)
	summaries := make(map[string]phase5MetricSummary, len(phase5RateThresholds)+1)
	for scID, threshold := range phase5RateThresholds {
		summaries[scID] = phase5MetricSummary{SCID: scID, RateThreshold: threshold}
	}
	summaries["SC-006"] = phase5MetricSummary{SCID: "SC-006", LatencyGateMS: phase5LatencyP95ThresholdM}

	latencyBySC := make(map[string][]float64)
	for _, event := range events {
		summary, ok := summaries[event.SCID]
		if !ok {
			continue
		}
		if event.Timestamp.Before(windowStart) || event.Timestamp.After(now) {
			continue
		}
		summary.Samples++
		if event.SCID == "SC-006" {
			if event.LatencyMS > 0 {
				latencyBySC[event.SCID] = append(latencyBySC[event.SCID], event.LatencyMS)
			}
		} else if event.Success != nil && *event.Success {
			summary.Successes++
		}
		summaries[event.SCID] = summary
	}

	failed := make([]string, 0)
	for scID, summary := range summaries {
		if scID == "SC-006" {
			latencies := latencyBySC[scID]
			if len(latencies) > 0 {
				summary.P95LatencyMS = phase5Percentile(latencies, 95)
			}
			summary.Passed = summary.Samples >= phase5MetricsMinSamples && summary.P95LatencyMS > 0 && summary.P95LatencyMS < summary.LatencyGateMS
			summary.EvaluationNote = fmt.Sprintf("samples=%d p95=%.2fms threshold=<%.2fms", summary.Samples, summary.P95LatencyMS, summary.LatencyGateMS)
		} else {
			if summary.Samples > 0 {
				summary.Rate = float64(summary.Successes) * 100.0 / float64(summary.Samples)
			}
			summary.Passed = summary.Samples >= phase5MetricsMinSamples && summary.Rate >= summary.RateThreshold
			summary.EvaluationNote = fmt.Sprintf("samples=%d success_rate=%.2f%% threshold=%.2f%%", summary.Samples, summary.Rate, summary.RateThreshold)
		}
		if !summary.Passed {
			failed = append(failed, scID)
		}
		summaries[scID] = summary
	}
	sort.Strings(failed)

	return phase5MetricGateReport{
		Summaries:   summaries,
		AllPassed:   len(failed) == 0,
		FailedSCIDs: failed,
	}
}

func loadPhase5MetricEventsJSONL(path string) ([]phase5MetricEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	events := make([]phase5MetricEvent, 0, 512)
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var event phase5MetricEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		if event.SCID == "" || event.Timestamp.IsZero() {
			return nil, fmt.Errorf("line %d: missing required fields timestamp/sc_id", lineNumber)
		}
		if event.SCID == "SC-006" {
			if event.LatencyMS <= 0 {
				return nil, fmt.Errorf("line %d: sc-006 requires latency_ms > 0", lineNumber)
			}
		} else if event.Success == nil {
			return nil, fmt.Errorf("line %d: %s requires success=true/false", lineNumber, event.SCID)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func renderPhase5MetricGateMarkdown(report phase5MetricGateReport, now time.Time, source string) string {
	builder := strings.Builder{}
	builder.WriteString("# Phase 5 Metrics Gate Report\n\n")
	builder.WriteString(fmt.Sprintf("- Generated At (UTC): %s\n", now.Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("- Source: `%s`\n", source))
	builder.WriteString(fmt.Sprintf("- Rolling Window: last %d days\n", phase5MetricsWindowDays))
	builder.WriteString(fmt.Sprintf("- Min Samples Per SC: %d\n\n", phase5MetricsMinSamples))

	builder.WriteString("| SC | Samples | Successes | Success Rate | p95 Latency | Threshold | Gate |\n")
	builder.WriteString("| --- | ---: | ---: | ---: | ---: | --- | --- |\n")
	for _, scID := range []string{"SC-001", "SC-002", "SC-003", "SC-004", "SC-005", "SC-006"} {
		summary := report.Summaries[scID]
		status := "PASS"
		if !summary.Passed {
			status = "BLOCK"
		}
		threshold := fmt.Sprintf("%.2f%%", summary.RateThreshold)
		rate := fmt.Sprintf("%.2f%%", summary.Rate)
		successes := fmt.Sprintf("%d", summary.Successes)
		latency := "-"
		if scID == "SC-006" {
			threshold = fmt.Sprintf("p95 < %.2fms", summary.LatencyGateMS)
			rate = "-"
			successes = "-"
			latency = fmt.Sprintf("%.2fms", summary.P95LatencyMS)
		}
		builder.WriteString(fmt.Sprintf("| %s | %d | %s | %s | %s | %s | %s |\n", scID, summary.Samples, successes, rate, latency, threshold, status))
	}

	builder.WriteString("\n")
	if report.AllPassed {
		builder.WriteString("Gate Result: PASS\n")
	} else {
		builder.WriteString(fmt.Sprintf("Gate Result: BLOCK (failed: %s)\n", strings.Join(report.FailedSCIDs, ", ")))
	}

	return builder.String()
}

func buildPhase5RateEvents(now time.Time, scID string, samples, successes int) []phase5MetricEvent {
	events := make([]phase5MetricEvent, 0, samples)
	for i := 0; i < samples; i++ {
		success := i < successes
		events = append(events, phase5MetricEvent{
			Timestamp: now.Add(-time.Duration(i%24) * time.Hour),
			SCID:      scID,
			Success:   &success,
		})
	}
	return events
}

func buildPhase5LatencyEvents(now time.Time, scID string, samples int, base, span float64) []phase5MetricEvent {
	events := make([]phase5MetricEvent, 0, samples)
	for i := 0; i < samples; i++ {
		latency := base + float64(i%10)*span/10.0
		events = append(events, phase5MetricEvent{
			Timestamp: now.Add(-time.Duration(i%24) * time.Hour),
			SCID:      scID,
			LatencyMS: latency,
		})
	}
	return events
}

func phase5Percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	rank := (p / 100.0) * float64(len(sorted)-1)
	lower := int(rank)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[lower]
	}
	fraction := rank - float64(lower)
	return sorted[lower] + fraction*(sorted[upper]-sorted[lower])
}
