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
	phase4MetricsWindowDays = 7
	phase4MetricsMinSamples = 200
)

var phase4RateThresholds = map[string]float64{
	"SC-001": 99.0,
	"SC-002": 99.0,
	"SC-003": 100.0,
	"SC-004": 100.0,
	"SC-005": 100.0,
}

const phase4LatencyP95ThresholdMs = 120.0

type phase4MetricEvent struct {
	Timestamp time.Time `json:"timestamp"`
	SCID      string    `json:"sc_id"`
	Success   *bool     `json:"success,omitempty"`
	LatencyMs float64   `json:"latency_ms,omitempty"`
}

type phase4MetricSummary struct {
	SCID           string
	Samples        int
	Successes      int
	Rate           float64
	RateThreshold  float64
	P95LatencyMs   float64
	LatencyGateMs  float64
	Passed         bool
	EvaluationNote string
}

type phase4MetricGateReport struct {
	Summaries   map[string]phase4MetricSummary
	AllPassed   bool
	FailedSCIDs []string
}

func TestPhase4ChannelsMetricsReportGateWithSyntheticDataset(t *testing.T) {
	now := time.Date(2026, 3, 12, 12, 0, 0, 0, time.UTC)
	events := make([]phase4MetricEvent, 0, 1400)
	events = append(events, buildRateEvents(now, "SC-001", 210, 208)...)
	events = append(events, buildRateEvents(now, "SC-002", 230, 228)...)
	events = append(events, buildRateEvents(now, "SC-003", 205, 205)...)
	events = append(events, buildRateEvents(now, "SC-004", 220, 220)...)
	events = append(events, buildRateEvents(now, "SC-005", 215, 215)...)
	events = append(events, buildLatencyEvents(now, "SC-006", 240, 85, 18)...)
	oldSuccess := true
	events = append(events, phase4MetricEvent{
		Timestamp: now.AddDate(0, 0, -(phase4MetricsWindowDays + 1)),
		SCID:      "SC-001",
		Success:   &oldSuccess,
	})

	report := evaluatePhase4MetricGate(events, now)
	if !report.AllPassed {
		t.Fatalf("expected synthetic dataset to pass gate, got failures: %v", report.FailedSCIDs)
	}

	for scID, summary := range report.Summaries {
		if summary.Samples < phase4MetricsMinSamples {
			t.Fatalf("%s sample count too small: %d", scID, summary.Samples)
		}
		if !summary.Passed {
			t.Fatalf("%s should pass: note=%s", scID, summary.EvaluationNote)
		}
	}
}

func TestPhase4ChannelsMetricsReportFromJSONL(t *testing.T) {
	inputPath := strings.TrimSpace(os.Getenv("SYNAPSEX_PHASE4_METRICS_JSONL"))
	if inputPath == "" {
		t.Skip("set SYNAPSEX_PHASE4_METRICS_JSONL to collect SC-001~SC-006 metrics from real logs")
	}

	events, err := loadPhase4MetricEventsJSONL(inputPath)
	if err != nil {
		t.Fatalf("load metrics jsonl: %v", err)
	}

	now := time.Now().UTC()
	report := evaluatePhase4MetricGate(events, now)
	markdown := renderPhase4MetricGateMarkdown(report, now, inputPath)

	outputPath := strings.TrimSpace(os.Getenv("SYNAPSEX_PHASE4_METRICS_REPORT"))
	if outputPath == "" {
		outputPath = filepath.Join("docs", "guides", "phase_4", "phase_4_metrics_report.md")
	}
	if err := os.WriteFile(outputPath, []byte(markdown), 0o644); err != nil {
		t.Fatalf("write metrics report: %v", err)
	}

	if !report.AllPassed {
		t.Fatalf("metrics gate failed, see report at %s (failed=%v)", outputPath, report.FailedSCIDs)
	}
}

func evaluatePhase4MetricGate(events []phase4MetricEvent, now time.Time) phase4MetricGateReport {
	windowStart := now.AddDate(0, 0, -phase4MetricsWindowDays)
	summaries := make(map[string]phase4MetricSummary, len(phase4RateThresholds)+1)
	for scID, threshold := range phase4RateThresholds {
		summaries[scID] = phase4MetricSummary{SCID: scID, RateThreshold: threshold}
	}
	summaries["SC-006"] = phase4MetricSummary{SCID: "SC-006", LatencyGateMs: phase4LatencyP95ThresholdMs}

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
			if event.LatencyMs > 0 {
				latencyBySC[event.SCID] = append(latencyBySC[event.SCID], event.LatencyMs)
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
				summary.P95LatencyMs = percentile(latencies, 95)
			}
			summary.Passed = summary.Samples >= phase4MetricsMinSamples && summary.P95LatencyMs > 0 && summary.P95LatencyMs < summary.LatencyGateMs
			summary.EvaluationNote = fmt.Sprintf("samples=%d p95=%.2fms threshold=<%.2fms", summary.Samples, summary.P95LatencyMs, summary.LatencyGateMs)
		} else {
			if summary.Samples > 0 {
				summary.Rate = float64(summary.Successes) * 100.0 / float64(summary.Samples)
			}
			summary.Passed = summary.Samples >= phase4MetricsMinSamples && summary.Rate >= summary.RateThreshold
			summary.EvaluationNote = fmt.Sprintf("samples=%d success_rate=%.2f%% threshold=%.2f%%", summary.Samples, summary.Rate, summary.RateThreshold)
		}
		if !summary.Passed {
			failed = append(failed, scID)
		}
		summaries[scID] = summary
	}
	sort.Strings(failed)

	return phase4MetricGateReport{
		Summaries:   summaries,
		AllPassed:   len(failed) == 0,
		FailedSCIDs: failed,
	}
}

func loadPhase4MetricEventsJSONL(path string) ([]phase4MetricEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	events := make([]phase4MetricEvent, 0, 512)
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var event phase4MetricEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		if event.SCID == "" || event.Timestamp.IsZero() {
			return nil, fmt.Errorf("line %d: missing required fields timestamp/sc_id", lineNumber)
		}
		if event.SCID == "SC-006" {
			if event.LatencyMs <= 0 {
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

func renderPhase4MetricGateMarkdown(report phase4MetricGateReport, now time.Time, source string) string {
	builder := strings.Builder{}
	builder.WriteString("# Phase 4 Metrics Gate Report\n\n")
	builder.WriteString(fmt.Sprintf("- Generated At (UTC): %s\n", now.Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("- Source: `%s`\n", source))
	builder.WriteString(fmt.Sprintf("- Rolling Window: last %d days\n", phase4MetricsWindowDays))
	builder.WriteString(fmt.Sprintf("- Min Samples Per SC: %d\n\n", phase4MetricsMinSamples))

	builder.WriteString("| SC | Samples | Successes | Success Rate | p95 Latency | Threshold | Gate |\n")
	builder.WriteString("| --- | ---: | ---: | ---: | ---: | --- | --- |\n")
	for _, scID := range []string{"SC-001", "SC-002", "SC-003", "SC-004", "SC-005", "SC-006"} {
		s := report.Summaries[scID]
		status := "PASS"
		if !s.Passed {
			status = "BLOCK"
		}
		threshold := fmt.Sprintf("%.2f%%", s.RateThreshold)
		rate := fmt.Sprintf("%.2f%%", s.Rate)
		successes := fmt.Sprintf("%d", s.Successes)
		latency := "-"
		if scID == "SC-006" {
			threshold = fmt.Sprintf("p95 < %.2fms", s.LatencyGateMs)
			rate = "-"
			successes = "-"
			latency = fmt.Sprintf("%.2fms", s.P95LatencyMs)
		}
		builder.WriteString(fmt.Sprintf("| %s | %d | %s | %s | %s | %s | %s |\n", scID, s.Samples, successes, rate, latency, threshold, status))
	}

	builder.WriteString("\n")
	if report.AllPassed {
		builder.WriteString("Gate Result: PASS\n")
	} else {
		builder.WriteString(fmt.Sprintf("Gate Result: BLOCK (failed: %s)\n", strings.Join(report.FailedSCIDs, ", ")))
	}

	return builder.String()
}

func buildRateEvents(now time.Time, scID string, samples, successes int) []phase4MetricEvent {
	events := make([]phase4MetricEvent, 0, samples)
	for i := 0; i < samples; i++ {
		success := i < successes
		events = append(events, phase4MetricEvent{
			Timestamp: now.Add(-time.Duration(i%24) * time.Hour),
			SCID:      scID,
			Success:   &success,
		})
	}
	return events
}

func buildLatencyEvents(now time.Time, scID string, samples int, base, span float64) []phase4MetricEvent {
	events := make([]phase4MetricEvent, 0, samples)
	for i := 0; i < samples; i++ {
		latency := base + float64(i%10)*span/10.0
		events = append(events, phase4MetricEvent{
			Timestamp: now.Add(-time.Duration(i%24) * time.Hour),
			SCID:      scID,
			LatencyMs: latency,
		})
	}
	return events
}

func percentile(values []float64, p float64) float64 {
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
