package service

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type ChannelRouteSummary struct {
	Channel  string
	Instance string
	Samples  int
	P95      time.Duration
	Average  time.Duration
	Min      time.Duration
	Max      time.Duration
}

type ChannelRouteMetrics struct {
	mu         sync.Mutex
	maxSamples int
	series     map[string][]time.Duration
}

func NewChannelRouteMetrics(maxSamples int) *ChannelRouteMetrics {
	if maxSamples <= 0 {
		maxSamples = 1024
	}
	return &ChannelRouteMetrics{
		maxSamples: maxSamples,
		series:     make(map[string][]time.Duration),
	}
}

func (m *ChannelRouteMetrics) Observe(channel, instance string, duration time.Duration) {
	channel = normalizeMetricLabel(channel, "unknown")
	instance = normalizeMetricLabel(instance, "default")
	if duration < 0 {
		duration = 0
	}

	key := channel + "|" + instance
	m.mu.Lock()
	defer m.mu.Unlock()

	values := append(m.series[key], duration)
	if len(values) > m.maxSamples {
		excess := len(values) - m.maxSamples
		values = append([]time.Duration(nil), values[excess:]...)
	}
	m.series[key] = values
}

func (m *ChannelRouteMetrics) Summaries() []ChannelRouteSummary {
	m.mu.Lock()
	copied := make(map[string][]time.Duration, len(m.series))
	for key, values := range m.series {
		copied[key] = append([]time.Duration(nil), values...)
	}
	m.mu.Unlock()

	summaries := make([]ChannelRouteSummary, 0, len(copied))
	for key, values := range copied {
		parts := strings.SplitN(key, "|", 2)
		if len(parts) != 2 || len(values) == 0 {
			continue
		}
		summary := ChannelRouteSummary{
			Channel:  parts[0],
			Instance: parts[1],
			Samples:  len(values),
		}
		sort.Slice(values, func(i, j int) bool {
			return values[i] < values[j]
		})
		summary.Min = values[0]
		summary.Max = values[len(values)-1]
		summary.P95 = percentileDuration(values, 0.95)

		var total time.Duration
		for _, value := range values {
			total += value
		}
		summary.Average = time.Duration(int64(total) / int64(len(values)))
		summaries = append(summaries, summary)
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Channel == summaries[j].Channel {
			return summaries[i].Instance < summaries[j].Instance
		}
		return summaries[i].Channel < summaries[j].Channel
	})
	return summaries
}

func (m *ChannelRouteMetrics) RenderP95Report(generatedAt time.Time) string {
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	summaries := m.Summaries()
	builder := strings.Builder{}
	builder.WriteString("# Channel Route Latency Report\n\n")
	builder.WriteString(fmt.Sprintf("- Generated At (UTC): %s\n", generatedAt.UTC().Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("- Series Count: %d\n\n", len(summaries)))
	builder.WriteString("| Channel | Instance | Samples | P95 | Avg | Min | Max |\n")
	builder.WriteString("| --- | --- | ---: | ---: | ---: | ---: | ---: |\n")
	for _, summary := range summaries {
		builder.WriteString(fmt.Sprintf(
			"| %s | %s | %d | %dms | %dms | %dms | %dms |\n",
			summary.Channel,
			summary.Instance,
			summary.Samples,
			summary.P95.Milliseconds(),
			summary.Average.Milliseconds(),
			summary.Min.Milliseconds(),
			summary.Max.Milliseconds(),
		))
	}
	return builder.String()
}

func percentileDuration(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	index := int(math.Ceil(float64(len(sorted))*p)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func normalizeMetricLabel(raw, fallback string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	return value
}
