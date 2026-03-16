package integration

import (
	"strings"
	"testing"
	"time"

	"clawx/internal/application/service"
)

func TestChannelRouteLatencyMetricsP95Report(t *testing.T) {
	metrics := service.NewChannelRouteMetrics(256)

	telegramDurations := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		60 * time.Millisecond,
		70 * time.Millisecond,
		80 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}
	for _, duration := range telegramDurations {
		metrics.Observe("telegram", "tg-main", duration)
	}

	for i := 0; i < 10; i++ {
		metrics.Observe("feishu", "feishu-main", 25*time.Millisecond)
	}

	summaries := metrics.Summaries()
	if len(summaries) != 2 {
		t.Fatalf("unexpected summary count: got=%d want=2", len(summaries))
	}

	var telegramSummary service.ChannelRouteSummary
	var feishuSummary service.ChannelRouteSummary
	for _, summary := range summaries {
		switch summary.Channel {
		case "telegram":
			telegramSummary = summary
		case "feishu":
			feishuSummary = summary
		}
	}

	if telegramSummary.Samples != len(telegramDurations) {
		t.Fatalf("telegram sample mismatch: got=%d want=%d", telegramSummary.Samples, len(telegramDurations))
	}
	if telegramSummary.P95 != 100*time.Millisecond {
		t.Fatalf("telegram p95 mismatch: got=%s want=%s", telegramSummary.P95, 100*time.Millisecond)
	}
	if feishuSummary.P95 != 25*time.Millisecond {
		t.Fatalf("feishu p95 mismatch: got=%s want=%s", feishuSummary.P95, 25*time.Millisecond)
	}

	report := metrics.RenderP95Report(time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(report, "| telegram | tg-main | 10 | 100ms") {
		t.Fatalf("report missing telegram p95 row: %s", report)
	}
	if !strings.Contains(report, "| feishu | feishu-main | 10 | 25ms") {
		t.Fatalf("report missing feishu p95 row: %s", report)
	}
}
