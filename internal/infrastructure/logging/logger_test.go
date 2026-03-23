package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoggerPromptTokenFieldsNotRedacted(t *testing.T) {
	var out bytes.Buffer
	logger := New(&out)
	logger.Info("trace", map[string]any{
		"prompt_tokens":        1024,
		"prompt_cached_tokens": 512,
		"completion_tokens":    256,
		"total_tokens":         1280,
		"api_token":            "secret-value",
	})
	body := out.String()
	if !strings.Contains(body, `"prompt_tokens":1024`) {
		t.Fatalf("expected prompt_tokens visible, got: %s", body)
	}
	if !strings.Contains(body, `"prompt_cached_tokens":512`) {
		t.Fatalf("expected prompt_cached_tokens visible, got: %s", body)
	}
	if !strings.Contains(body, `"completion_tokens":256`) {
		t.Fatalf("expected completion_tokens visible, got: %s", body)
	}
	if !strings.Contains(body, `"total_tokens":1280`) {
		t.Fatalf("expected total_tokens visible, got: %s", body)
	}
	if !strings.Contains(body, `"api_token":"[REDACTED]"`) {
		t.Fatalf("expected api_token redacted, got: %s", body)
	}
}
