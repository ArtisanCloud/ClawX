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
	phase6MetricsWindowDays    = 7
	phase6MetricsMinSamples    = 200
	phase6LatencyP95ThresholdM = 120.0
)

var phase6RateThresholds = map[string]float64{
	"SC-001": 100.0,
	"SC-002": 95.0,
	"SC-003": 100.0,
	"SC-004": 100.0,
	"SC-005": 100.0,
}

type phase6MetricEvent struct {
	Timestamp time.Time `json:"timestamp"`
	SCID      string    `json:"sc_id"`
	Success   *bool     `json:"success,omitempty"`
	LatencyMS float64   `json:"latency_ms,omitempty"`
}

type phase6MetricSummary struct {
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

type phase6MetricGateReport struct {
	Summaries   map[string]phase6MetricSummary
	AllPassed   bool
	FailedSCIDs []string
}

func TestPhase6MemoryMetricsReportGateWithSyntheticDataset(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	events := make([]phase6MetricEvent, 0, 1700)
	events = append(events, buildPhase6RateEvents(now, "SC-001", 210, 210)...)
	events = append(events, buildPhase6RateEvents(now, "SC-002", 220, 214)...)
	events = append(events, buildPhase6RateEvents(now, "SC-003", 205, 205)...)
	events = append(events, buildPhase6RateEvents(now, "SC-004", 215, 215)...)
	events = append(events, buildPhase6RateEvents(now, "SC-005", 205, 205)...)
	events = append(events, buildPhase6LatencyEvents(now, "SC-006", 240, 74, 26)...)
	oldSuccess := false
	events = append(events, phase6MetricEvent{
		Timestamp: now.AddDate(0, 0, -(phase6MetricsWindowDays + 1)),
		SCID:      "SC-002",
		Success:   &oldSuccess,
	})

	report := evaluatePhase6MetricGate(events, now)
	if !report.AllPassed {
		t.Fatalf("expected synthetic dataset to pass gate, got failures: %v", report.FailedSCIDs)
	}

	for scID, summary := range report.Summaries {
		if summary.Samples < phase6MetricsMinSamples {
			t.Fatalf("%s sample count too small: %d", scID, summary.Samples)
		}
		if !summary.Passed {
			t.Fatalf("%s should pass: note=%s", scID, summary.EvaluationNote)
		}
	}
}

func TestPhase6MemoryMetricsReportFromJSONL(t *testing.T) {
	inputPath := strings.TrimSpace(os.Getenv("CLAWX_PHASE6_METRICS_JSONL"))
	if inputPath == "" {
		t.Skip("set CLAWX_PHASE6_METRICS_JSONL to collect SC-001~SC-006 metrics from real logs")
	}

	events, err := loadPhase6MetricEventsJSONL(inputPath)
	if err != nil {
		t.Fatalf("load metrics jsonl: %v", err)
	}

	now := time.Now().UTC()
	report := evaluatePhase6MetricGate(events, now)
	markdown := renderPhase6MetricGateMarkdown(report, now, inputPath)

	outputPath := strings.TrimSpace(os.Getenv("CLAWX_PHASE6_METRICS_REPORT"))
	if outputPath == "" {
		outputPath = filepath.Join("docs", "guides", "phase_6", "phase_6_metrics_report.md")
	}
	if err := os.WriteFile(outputPath, []byte(markdown), 0o644); err != nil {
		t.Fatalf("write metrics report: %v", err)
	}

	if !report.AllPassed {
		t.Fatalf("metrics gate failed, see report at %s (failed=%v)", outputPath, report.FailedSCIDs)
	}
}

func evaluatePhase6MetricGate(events []phase6MetricEvent, now time.Time) phase6MetricGateReport {
	windowStart := now.AddDate(0, 0, -phase6MetricsWindowDays)
	summaries := make(map[string]phase6MetricSummary, len(phase6RateThresholds)+1)
	for scID, threshold := range phase6RateThresholds {
		summaries[scID] = phase6MetricSummary{SCID: scID, RateThreshold: threshold}
	}
	summaries["SC-006"] = phase6MetricSummary{SCID: "SC-006", LatencyGateMS: phase6LatencyP95ThresholdM}

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
				summary.P95LatencyMS = phase6Percentile(latencies, 95)
			}
			summary.Passed = summary.Samples >= phase6MetricsMinSamples && summary.P95LatencyMS > 0 && summary.P95LatencyMS < summary.LatencyGateMS
			summary.EvaluationNote = fmt.Sprintf("samples=%d p95=%.2fms threshold=<%.2fms", summary.Samples, summary.P95LatencyMS, summary.LatencyGateMS)
		} else {
			if summary.Samples > 0 {
				summary.Rate = float64(summary.Successes) * 100.0 / float64(summary.Samples)
			}
			summary.Passed = summary.Samples >= phase6MetricsMinSamples && summary.Rate >= summary.RateThreshold
			summary.EvaluationNote = fmt.Sprintf("samples=%d success_rate=%.2f%% threshold=%.2f%%", summary.Samples, summary.Rate, summary.RateThreshold)
		}
		if !summary.Passed {
			failed = append(failed, scID)
		}
		summaries[scID] = summary
	}
	sort.Strings(failed)

	return phase6MetricGateReport{
		Summaries:   summaries,
		AllPassed:   len(failed) == 0,
		FailedSCIDs: failed,
	}
}

func loadPhase6MetricEventsJSONL(path string) ([]phase6MetricEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	events := make([]phase6MetricEvent, 0, 512)
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var event phase6MetricEvent
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

func renderPhase6MetricGateMarkdown(report phase6MetricGateReport, now time.Time, source string) string {
	builder := strings.Builder{}
	builder.WriteString("# Phase 6 Metrics Gate Report\n\n")
	builder.WriteString(fmt.Sprintf("- Generated At (UTC): %s\n", now.Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("- Source: `%s`\n", source))
	builder.WriteString(fmt.Sprintf("- Rolling Window: last %d days\n", phase6MetricsWindowDays))
	builder.WriteString(fmt.Sprintf("- Min Samples Per SC: %d\n\n", phase6MetricsMinSamples))

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

func buildPhase6RateEvents(now time.Time, scID string, samples, successes int) []phase6MetricEvent {
	events := make([]phase6MetricEvent, 0, samples)
	for i := 0; i < samples; i++ {
		success := i < successes
		events = append(events, phase6MetricEvent{
			Timestamp: now.Add(-time.Duration(i%24) * time.Hour),
			SCID:      scID,
			Success:   &success,
		})
	}
	return events
}

func buildPhase6LatencyEvents(now time.Time, scID string, samples int, base, span float64) []phase6MetricEvent {
	events := make([]phase6MetricEvent, 0, samples)
	for i := 0; i < samples; i++ {
		latency := base + float64(i%10)*span/10.0
		events = append(events, phase6MetricEvent{
			Timestamp: now.Add(-time.Duration(i%24) * time.Hour),
			SCID:      scID,
			LatencyMS: latency,
		})
	}
	return events
}

func phase6Percentile(values []float64, p float64) float64 {
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
