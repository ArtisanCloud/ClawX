package integration

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	metricsWindowDays = 7
	metricsMinSamples = 200
)

var successCriteriaThresholds = map[string]float64{
	"SC-001": 95.0,
	"SC-002": 95.0,
	"SC-003": 100.0,
	"SC-004": 95.0,
}

type metricEvent struct {
	Timestamp time.Time `json:"timestamp"`
	SCID      string    `json:"sc_id"`
	Success   bool      `json:"success"`
}

type metricSummary struct {
	SCID      string
	Samples   int
	Successes int
	Rate      float64
	Threshold float64
	Passed    bool
}

func TestMultiSessionMetricsReportGateWithSyntheticDataset(t *testing.T) {
	now := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	events := make([]metricEvent, 0, 900)
	events = append(events, buildEvents(now, "SC-001", 210, 200)...)
	events = append(events, buildEvents(now, "SC-002", 220, 210)...)
	events = append(events, buildEvents(now, "SC-003", 205, 205)...)
	events = append(events, buildEvents(now, "SC-004", 215, 205)...)
	// Out-of-window records should not impact the gate result.
	events = append(events, metricEvent{
		Timestamp: now.AddDate(0, 0, -(metricsWindowDays + 1)),
		SCID:      "SC-001",
		Success:   false,
	})

	report := evaluateMetricGate(events, now)
	if !report.AllPassed {
		t.Fatalf("expected synthetic dataset to pass gate, got failures: %v", report.FailedSCIDs)
	}

	for scID, summary := range report.Summaries {
		if summary.Samples < metricsMinSamples {
			t.Fatalf("%s sample count too small: %d", scID, summary.Samples)
		}
		if !summary.Passed {
			t.Fatalf("%s should pass: rate=%.2f threshold=%.2f", scID, summary.Rate, summary.Threshold)
		}
	}
}

func TestMultiSessionMetricsReportFromJSONL(t *testing.T) {
	inputPath := strings.TrimSpace(os.Getenv("SYNAPSEX_PHASE2_METRICS_JSONL"))
	if inputPath == "" {
		t.Skip("set SYNAPSEX_PHASE2_METRICS_JSONL to collect SC-001~SC-004 metrics from real logs")
	}

	events, err := loadMetricEventsJSONL(inputPath)
	if err != nil {
		t.Fatalf("load metrics jsonl: %v", err)
	}

	now := time.Now().UTC()
	report := evaluateMetricGate(events, now)
	markdown := renderMetricGateMarkdown(report, now, inputPath)

	outputPath := strings.TrimSpace(os.Getenv("SYNAPSEX_PHASE2_METRICS_REPORT"))
	if outputPath == "" {
		outputPath = filepath.Join("docs", "guides", "phase_2", "phase_2_metrics_report.md")
	}
	if err := os.WriteFile(outputPath, []byte(markdown), 0o644); err != nil {
		t.Fatalf("write metrics report: %v", err)
	}

	if !report.AllPassed {
		t.Fatalf("metrics gate failed, see report at %s (failed=%v)", outputPath, report.FailedSCIDs)
	}
}

type metricGateReport struct {
	Summaries   map[string]metricSummary
	AllPassed   bool
	FailedSCIDs []string
}

func evaluateMetricGate(events []metricEvent, now time.Time) metricGateReport {
	windowStart := now.AddDate(0, 0, -metricsWindowDays)
	summaries := make(map[string]metricSummary, len(successCriteriaThresholds))
	for scID, threshold := range successCriteriaThresholds {
		summaries[scID] = metricSummary{SCID: scID, Threshold: threshold}
	}

	for _, event := range events {
		summary, ok := summaries[event.SCID]
		if !ok {
			continue
		}
		if event.Timestamp.Before(windowStart) || event.Timestamp.After(now) {
			continue
		}
		summary.Samples++
		if event.Success {
			summary.Successes++
		}
		summaries[event.SCID] = summary
	}

	failed := make([]string, 0)
	for scID, summary := range summaries {
		if summary.Samples > 0 {
			summary.Rate = float64(summary.Successes) * 100.0 / float64(summary.Samples)
		}
		summary.Passed = summary.Samples >= metricsMinSamples && summary.Rate >= summary.Threshold
		if !summary.Passed {
			failed = append(failed, scID)
		}
		summaries[scID] = summary
	}

	return metricGateReport{
		Summaries:   summaries,
		AllPassed:   len(failed) == 0,
		FailedSCIDs: failed,
	}
}

func loadMetricEventsJSONL(path string) ([]metricEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	events := make([]metricEvent, 0, 512)
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var event metricEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		if event.SCID == "" || event.Timestamp.IsZero() {
			return nil, fmt.Errorf("line %d: missing required fields timestamp/sc_id", lineNumber)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func renderMetricGateMarkdown(report metricGateReport, now time.Time, source string) string {
	builder := strings.Builder{}
	builder.WriteString("# Phase 2 Metrics Gate Report\n\n")
	builder.WriteString(fmt.Sprintf("- Generated At (UTC): %s\n", now.Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("- Source: `%s`\n", source))
	builder.WriteString(fmt.Sprintf("- Rolling Window: last %d days\n", metricsWindowDays))
	builder.WriteString(fmt.Sprintf("- Min Samples Per SC: %d\n\n", metricsMinSamples))

	builder.WriteString("| SC | Samples | Successes | Success Rate | Threshold | Gate |\n")
	builder.WriteString("| --- | ---: | ---: | ---: | ---: | --- |\n")
	for _, scID := range []string{"SC-001", "SC-002", "SC-003", "SC-004"} {
		s := report.Summaries[scID]
		status := "PASS"
		if !s.Passed {
			status = "BLOCK"
		}
		builder.WriteString(fmt.Sprintf("| %s | %d | %d | %.2f%% | %.2f%% | %s |\n", scID, s.Samples, s.Successes, s.Rate, s.Threshold, status))
	}

	builder.WriteString("\n")
	if report.AllPassed {
		builder.WriteString("Gate Result: PASS\n")
	} else {
		builder.WriteString(fmt.Sprintf("Gate Result: BLOCK (failed: %s)\n", strings.Join(report.FailedSCIDs, ", ")))
	}

	return builder.String()
}

func buildEvents(now time.Time, scID string, samples, successes int) []metricEvent {
	events := make([]metricEvent, 0, samples)
	for i := 0; i < samples; i++ {
		success := i < successes
		events = append(events, metricEvent{
			Timestamp: now.Add(-time.Duration(i%24) * time.Hour),
			SCID:      scID,
			Success:   success,
		})
	}
	return events
}
