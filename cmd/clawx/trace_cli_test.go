package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunTraceCacheReport(t *testing.T) {
	dir := t.TempDir()
	traceFile := filepath.Join(dir, "trace.jsonl")
	body := strings.Join([]string{
		`{"event":"llm_io","phase":"response","channel":"discord","agent_id":"main","intent_kind":"execute","prompt_cached_tokens":100,"prompt_tokens":200}`,
		`{"event":"llm_io","phase":"response","channel":"telegram","agent_id":"main","intent_kind":"execute","prompt_cached_tokens":0,"prompt_tokens":100}`,
	}, "\n")
	if err := os.WriteFile(traceFile, []byte(body), 0o644); err != nil {
		t.Fatalf("write trace file: %v", err)
	}

	var output bytes.Buffer
	if err := runTraceCacheReport([]string{"--file", traceFile}, &output); err != nil {
		t.Fatalf("run trace cache report: %v", err)
	}
	text := output.String()
	if !strings.Contains(text, "Prompt Cache Metrics") {
		t.Fatalf("expected report title, got: %s", text)
	}
	if !strings.Contains(text, "by_channel") {
		t.Fatalf("expected channel section, got: %s", text)
	}
}

func TestRunTraceCacheReportJSON(t *testing.T) {
	dir := t.TempDir()
	traceFile := filepath.Join(dir, "trace.jsonl")
	body := `{"event":"llm_io","phase":"response","channel":"discord","agent_id":"main","intent_kind":"execute","prompt_cached_tokens":50,"prompt_tokens":100}`
	if err := os.WriteFile(traceFile, []byte(body), 0o644); err != nil {
		t.Fatalf("write trace file: %v", err)
	}

	var output bytes.Buffer
	if err := runTraceCacheReport([]string{"--file", traceFile, "--json"}, &output); err != nil {
		t.Fatalf("run trace cache report json: %v", err)
	}
	text := output.String()
	if !strings.Contains(text, "\"TotalResponses\": 1") {
		t.Fatalf("expected json summary, got: %s", text)
	}
}

func TestRunTraceCommandUnknown(t *testing.T) {
	err := runTraceCommand([]string{"unknown"})
	if err == nil {
		t.Fatalf("expected error for unknown trace command")
	}
}

func TestRunTraceTokenReport(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token_usage.jsonl")
	body := strings.Join([]string{
		`{"timestamp":"2026-03-23T00:00:00Z","channel":"discord","agent_id":"main","prompt_cached_tokens":100,"prompt_tokens":200,"completion_tokens":50,"total_tokens":250,"estimated_cost_usd":0.001}`,
		`{"timestamp":"2026-03-23T00:01:00Z","channel":"discord","agent_id":"bid-all","prompt_cached_tokens":0,"prompt_tokens":300,"completion_tokens":100,"total_tokens":400,"estimated_cost_usd":0.002}`,
	}, "\n")
	if err := os.WriteFile(tokenFile, []byte(body), 0o644); err != nil {
		t.Fatalf("write token usage file: %v", err)
	}

	var output bytes.Buffer
	if err := runTraceTokenReport([]string{"--file", tokenFile}, &output); err != nil {
		t.Fatalf("run trace token report: %v", err)
	}
	text := output.String()
	if !strings.Contains(text, "Token Usage Summary") {
		t.Fatalf("expected token report title, got: %s", text)
	}
	if !strings.Contains(text, "by_agent") || !strings.Contains(text, "by_channel") {
		t.Fatalf("expected grouped sections, got: %s", text)
	}
}

func TestRunTraceTokenReportJSON(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token_usage.jsonl")
	body := `{"timestamp":"2026-03-23T00:00:00Z","channel":"discord","agent_id":"main","prompt_cached_tokens":100,"prompt_tokens":200,"completion_tokens":50,"total_tokens":250,"estimated_cost_usd":0.001}`
	if err := os.WriteFile(tokenFile, []byte(body), 0o644); err != nil {
		t.Fatalf("write token usage file: %v", err)
	}

	var output bytes.Buffer
	if err := runTraceTokenReport([]string{"--file", tokenFile, "--json"}, &output); err != nil {
		t.Fatalf("run trace token report json: %v", err)
	}
	if !strings.Contains(output.String(), "\"Requests\": 1") {
		t.Fatalf("expected json summary, got: %s", output.String())
	}
}
