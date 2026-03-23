package logging

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

type PromptCacheMetrics struct {
	TotalResponses int
	TotalTokens    int
	CachedTokens   int
	HitRate        float64
	ByChannel      map[string]PromptCacheBucket
	ByAgent        map[string]PromptCacheBucket
	ByStage        map[string]PromptCacheBucket
}

type PromptCacheBucket struct {
	Responses    int
	TotalTokens  int
	CachedTokens int
	HitRate      float64
}

func AggregatePromptCacheMetrics(reader io.Reader) (PromptCacheMetrics, error) {
	metrics := PromptCacheMetrics{
		ByChannel: make(map[string]PromptCacheBucket),
		ByAgent:   make(map[string]PromptCacheBucket),
		ByStage:   make(map[string]PromptCacheBucket),
	}
	scanner := bufio.NewScanner(reader)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return PromptCacheMetrics{}, fmt.Errorf("parse trace line %d: %w", lineNo, err)
		}
		if strings.TrimSpace(asString(record["event"])) != "llm_io" {
			continue
		}
		if strings.TrimSpace(asString(record["phase"])) != "response" {
			continue
		}
		promptTokens := asInt(record["prompt_tokens"])
		cachedTokens := asInt(record["prompt_cached_tokens"])
		if promptTokens <= 0 {
			continue
		}
		if cachedTokens < 0 {
			cachedTokens = 0
		}
		if cachedTokens > promptTokens {
			cachedTokens = promptTokens
		}
		channel := nonEmptyOrDefault(asString(record["channel"]), "unknown")
		agentID := nonEmptyOrDefault(asString(record["agent_id"]), "unknown")
		stage := nonEmptyOrDefault(asString(record["intent_kind"]), "unknown")

		metrics.TotalResponses++
		metrics.TotalTokens += promptTokens
		metrics.CachedTokens += cachedTokens
		metrics.ByChannel[channel] = mergePromptCacheBucket(metrics.ByChannel[channel], promptTokens, cachedTokens)
		metrics.ByAgent[agentID] = mergePromptCacheBucket(metrics.ByAgent[agentID], promptTokens, cachedTokens)
		metrics.ByStage[stage] = mergePromptCacheBucket(metrics.ByStage[stage], promptTokens, cachedTokens)
	}
	if err := scanner.Err(); err != nil {
		return PromptCacheMetrics{}, err
	}
	metrics.HitRate = hitRate(metrics.CachedTokens, metrics.TotalTokens)
	metrics.ByChannel = finalizePromptCacheBuckets(metrics.ByChannel)
	metrics.ByAgent = finalizePromptCacheBuckets(metrics.ByAgent)
	metrics.ByStage = finalizePromptCacheBuckets(metrics.ByStage)
	return metrics, nil
}

func RenderPromptCacheMetricsReport(metrics PromptCacheMetrics) string {
	var b strings.Builder
	b.WriteString("Prompt Cache Metrics\n")
	b.WriteString(fmt.Sprintf("total_responses=%d total_tokens=%d cached_tokens=%d hit_rate=%.2f%%\n",
		metrics.TotalResponses,
		metrics.TotalTokens,
		metrics.CachedTokens,
		metrics.HitRate*100,
	))
	b.WriteString(renderPromptCacheSection("by_channel", metrics.ByChannel))
	b.WriteString(renderPromptCacheSection("by_agent", metrics.ByAgent))
	b.WriteString(renderPromptCacheSection("by_stage", metrics.ByStage))
	return strings.TrimSpace(b.String())
}

func renderPromptCacheSection(name string, buckets map[string]PromptCacheBucket) string {
	if len(buckets) == 0 {
		return fmt.Sprintf("%s: (empty)\n", name)
	}
	keys := make([]string, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(name)
	b.WriteString(":\n")
	for _, key := range keys {
		item := buckets[key]
		b.WriteString(fmt.Sprintf("- %s responses=%d tokens=%d cached=%d hit_rate=%.2f%%\n",
			key,
			item.Responses,
			item.TotalTokens,
			item.CachedTokens,
			item.HitRate*100,
		))
	}
	return b.String()
}

func mergePromptCacheBucket(bucket PromptCacheBucket, promptTokens, cachedTokens int) PromptCacheBucket {
	bucket.Responses++
	bucket.TotalTokens += promptTokens
	bucket.CachedTokens += cachedTokens
	bucket.HitRate = hitRate(bucket.CachedTokens, bucket.TotalTokens)
	return bucket
}

func finalizePromptCacheBuckets(in map[string]PromptCacheBucket) map[string]PromptCacheBucket {
	out := make(map[string]PromptCacheBucket, len(in))
	for key, bucket := range in {
		bucket.HitRate = hitRate(bucket.CachedTokens, bucket.TotalTokens)
		out[key] = bucket
	}
	return out
}

func hitRate(cached, total int) float64 {
	if total <= 0 || cached <= 0 {
		return 0
	}
	return float64(cached) / float64(total)
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func asInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed)
		}
	}
	return 0
}

func nonEmptyOrDefault(raw, fallback string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	return value
}
