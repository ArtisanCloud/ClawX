package logging

import (
	"strings"
	"testing"
)

func TestAggregatePromptCacheMetrics(t *testing.T) {
	input := strings.Join([]string{
		`{"event":"llm_io","phase":"request","channel":"discord","agent_id":"main","intent_kind":"execute","prompt_cache_key":"k1"}`,
		`{"event":"llm_io","phase":"response","channel":"discord","agent_id":"main","intent_kind":"execute","prompt_cached_tokens":100,"prompt_tokens":200}`,
		`{"event":"llm_io","phase":"response","channel":"discord","agent_id":"bid-all","intent_kind":"control","prompt_cached_tokens":50,"prompt_tokens":100}`,
		`{"event":"llm_io","phase":"response","channel":"telegram","agent_id":"bid-all","intent_kind":"execute","prompt_cached_tokens":0,"prompt_tokens":80}`,
		`{"event":"control_apply_trace","phase":"apply","agent_id":"bid-all"}`,
	}, "\n")

	metrics, err := AggregatePromptCacheMetrics(strings.NewReader(input))
	if err != nil {
		t.Fatalf("aggregate metrics: %v", err)
	}
	if metrics.TotalResponses != 3 {
		t.Fatalf("unexpected total responses: %d", metrics.TotalResponses)
	}
	if metrics.TotalTokens != 380 {
		t.Fatalf("unexpected total tokens: %d", metrics.TotalTokens)
	}
	if metrics.CachedTokens != 150 {
		t.Fatalf("unexpected cached tokens: %d", metrics.CachedTokens)
	}
	if metrics.HitRate <= 0.39 || metrics.HitRate >= 0.40 {
		t.Fatalf("unexpected hit rate: %f", metrics.HitRate)
	}
	if metrics.ByChannel["discord"].Responses != 2 {
		t.Fatalf("unexpected discord responses: %+v", metrics.ByChannel["discord"])
	}
	if metrics.ByAgent["bid-all"].TotalTokens != 180 {
		t.Fatalf("unexpected bid-all bucket: %+v", metrics.ByAgent["bid-all"])
	}
	if metrics.ByStage["execute"].Responses != 2 {
		t.Fatalf("unexpected execute stage bucket: %+v", metrics.ByStage["execute"])
	}
}

func TestAggregatePromptCacheMetricsInvalidLine(t *testing.T) {
	_, err := AggregatePromptCacheMetrics(strings.NewReader("{not-json}"))
	if err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestRenderPromptCacheMetricsReport(t *testing.T) {
	metrics := PromptCacheMetrics{
		TotalResponses: 2,
		TotalTokens:    1000,
		CachedTokens:   300,
		HitRate:        0.3,
		ByChannel: map[string]PromptCacheBucket{
			"discord": {Responses: 2, TotalTokens: 1000, CachedTokens: 300, HitRate: 0.3},
		},
		ByAgent: map[string]PromptCacheBucket{
			"main": {Responses: 2, TotalTokens: 1000, CachedTokens: 300, HitRate: 0.3},
		},
		ByStage: map[string]PromptCacheBucket{
			"execute": {Responses: 2, TotalTokens: 1000, CachedTokens: 300, HitRate: 0.3},
		},
	}
	report := RenderPromptCacheMetricsReport(metrics)
	if !strings.Contains(report, "hit_rate=30.00%") {
		t.Fatalf("expected hit rate in report, got: %s", report)
	}
	if !strings.Contains(report, "by_channel") || !strings.Contains(report, "by_agent") || !strings.Contains(report, "by_stage") {
		t.Fatalf("expected grouped sections, got: %s", report)
	}
}
