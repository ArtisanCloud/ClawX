package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"clawx/internal/application/service"
	"clawx/internal/infrastructure/config"
	stdlogging "clawx/internal/infrastructure/logging"
)

type tokenUsageRecord struct {
	Timestamp          string  `json:"timestamp"`
	Channel            string  `json:"channel"`
	Instance           string  `json:"instance"`
	AgentID            string  `json:"agent_id"`
	ProjectID          string  `json:"project_id"`
	ConversationID     string  `json:"conversation_id"`
	SessionID          string  `json:"session_id"`
	BackendSessionID   string  `json:"backend_session_id"`
	Model              string  `json:"model,omitempty"`
	PromptCachedTokens int     `json:"prompt_cached_tokens"`
	PromptTokens       int     `json:"prompt_tokens"`
	CompletionTokens   int     `json:"completion_tokens"`
	TotalTokens        int     `json:"total_tokens"`
	State              string  `json:"state"`
	DurationMS         int64   `json:"duration_ms"`
	EstimatedCostUSD   float64 `json:"estimated_cost_usd"`
}

var tokenUsageMu sync.Mutex
var tokenUsageWriter *stdlogging.RotatingWriter

func recordTokenUsage(channel, instanceID string, runtime agentRuntime, decision service.Decision, flowResult service.SessionFlowResult) {
	record := tokenUsageRecord{
		Timestamp:          time.Now().UTC().Format(time.RFC3339Nano),
		Channel:            strings.TrimSpace(channel),
		Instance:           strings.TrimSpace(instanceID),
		AgentID:            strings.TrimSpace(runtime.agentID),
		ProjectID:          strings.TrimSpace(decision.ProjectID),
		ConversationID:     strings.TrimSpace(decision.ConversationID),
		SessionID:          strings.TrimSpace(flowResult.Session.ID),
		BackendSessionID:   strings.TrimSpace(flowResult.Execution.BackendSessionID),
		Model:              strings.TrimSpace(runtime.profileModel),
		PromptCachedTokens: flowResult.Execution.PromptCachedTokens,
		PromptTokens:       flowResult.Execution.PromptTokens,
		CompletionTokens:   flowResult.Execution.CompletionTokens,
		TotalTokens:        flowResult.Execution.TotalTokens,
		State:              strings.TrimSpace(string(flowResult.Execution.State)),
		DurationMS:         flowResult.Execution.CompletedAt.Sub(flowResult.Execution.StartedAt).Milliseconds(),
	}
	if record.TotalTokens == 0 && (record.PromptTokens > 0 || record.CompletionTokens > 0) {
		record.TotalTokens = record.PromptTokens + record.CompletionTokens
	}
	record.EstimatedCostUSD = estimateTokenCostUSD(record.PromptTokens, record.CompletionTokens)
	_ = appendTokenUsageRecord(record)
}

func appendTokenUsageRecord(record tokenUsageRecord) error {
	tokenUsageMu.Lock()
	defer tokenUsageMu.Unlock()
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	writer, err := ensureTokenUsageWriterLocked()
	if err != nil {
		return err
	}
	_, err = writer.Write(body)
	return err
}

func ensureTokenUsageWriterLocked() (*stdlogging.RotatingWriter, error) {
	if tokenUsageWriter != nil {
		return tokenUsageWriter, nil
	}
	maxMB := parsePositiveIntEnv("CLAWX_TOKEN_USAGE_LOG_MAX_MB", 20)
	maxBackups := parsePositiveIntEnv("CLAWX_TOKEN_USAGE_LOG_MAX_BACKUPS", 5)
	logPath := strings.TrimSpace(os.Getenv("CLAWX_TOKEN_USAGE_LOG_FILE"))
	if logPath == "" {
		logPath = filepath.Join(config.StateDir(), "logs", "token_usage.jsonl")
	}
	writer, err := stdlogging.NewRotatingWriter(logPath, int64(maxMB)*1024*1024, maxBackups)
	if err != nil {
		return nil, err
	}
	tokenUsageWriter = writer
	return tokenUsageWriter, nil
}

func estimateTokenCostUSD(promptTokens, completionTokens int) float64 {
	promptRate := parseTokenRateEnv("CLAWX_TOKEN_COST_PROMPT_PER_1M")
	completionRate := parseTokenRateEnv("CLAWX_TOKEN_COST_COMPLETION_PER_1M")
	if promptRate <= 0 && completionRate <= 0 {
		return 0
	}
	return (float64(promptTokens)*promptRate + float64(completionTokens)*completionRate) / 1_000_000
}

func parseTokenRateEnv(key string) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

type tokenUsageSummary struct {
	Requests         int
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	PromptCached     int
	EstimatedCostUSD float64
	ByAgent          map[string]tokenUsageBucket
	ByChannel        map[string]tokenUsageBucket
}

type tokenUsageBucket struct {
	Requests         int     `json:"requests"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	PromptCached     int     `json:"prompt_cached_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

func summarizeTokenUsageFile(path string) (tokenUsageSummary, error) {
	summary := tokenUsageSummary{
		ByAgent:   make(map[string]tokenUsageBucket),
		ByChannel: make(map[string]tokenUsageBucket),
	}
	f, err := os.Open(path)
	if err != nil {
		return summary, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record tokenUsageRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		summary.Requests++
		summary.PromptTokens += record.PromptTokens
		summary.CompletionTokens += record.CompletionTokens
		summary.TotalTokens += record.TotalTokens
		summary.PromptCached += record.PromptCachedTokens
		summary.EstimatedCostUSD += record.EstimatedCostUSD
		agent := nonEmpty(record.AgentID, "unknown")
		channel := nonEmpty(record.Channel, "unknown")
		summary.ByAgent[agent] = mergeTokenUsageBucket(summary.ByAgent[agent], record)
		summary.ByChannel[channel] = mergeTokenUsageBucket(summary.ByChannel[channel], record)
	}
	if err := scanner.Err(); err != nil {
		return summary, err
	}
	return summary, nil
}

func renderTokenUsageSummary(summary tokenUsageSummary) string {
	var b strings.Builder
	b.WriteString("Token Usage Summary\n")
	b.WriteString(fmt.Sprintf("requests=%d prompt_tokens=%d completion_tokens=%d total_tokens=%d prompt_cached_tokens=%d estimated_cost_usd=%.6f\n",
		summary.Requests, summary.PromptTokens, summary.CompletionTokens, summary.TotalTokens, summary.PromptCached, summary.EstimatedCostUSD))
	b.WriteString(renderTokenUsageBuckets("by_agent", summary.ByAgent))
	b.WriteString(renderTokenUsageBuckets("by_channel", summary.ByChannel))
	return strings.TrimSpace(b.String())
}

func renderTokenUsageBuckets(title string, items map[string]tokenUsageBucket) string {
	if len(items) == 0 {
		return fmt.Sprintf("%s: (empty)\n", title)
	}
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(title)
	b.WriteString(":\n")
	for _, key := range keys {
		item := items[key]
		b.WriteString(fmt.Sprintf("- %s requests=%d prompt=%d completion=%d total=%d cached=%d cost_usd=%.6f\n",
			key, item.Requests, item.PromptTokens, item.CompletionTokens, item.TotalTokens, item.PromptCached, item.EstimatedCostUSD))
	}
	return b.String()
}

func mergeTokenUsageBucket(bucket tokenUsageBucket, record tokenUsageRecord) tokenUsageBucket {
	bucket.Requests++
	bucket.PromptTokens += record.PromptTokens
	bucket.CompletionTokens += record.CompletionTokens
	bucket.TotalTokens += record.TotalTokens
	bucket.PromptCached += record.PromptCachedTokens
	bucket.EstimatedCostUSD += record.EstimatedCostUSD
	return bucket
}

func nonEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
