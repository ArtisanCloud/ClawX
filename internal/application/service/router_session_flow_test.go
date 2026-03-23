package service

import (
	"strings"
	"testing"

	"clawx/internal/application/command"
	"clawx/internal/domain/session"
)

func TestBuildPromptCacheKey(t *testing.T) {
	cmd := command.SessionCommand{
		ProjectID: "bid-all",
		RouteKey:  "discord:default:conv-1",
		Backend:   "codex",
	}
	record := session.Record{
		AgentID: "bid-all",
	}
	key := buildPromptCacheKey(cmd, record)
	if !strings.Contains(key, "clawx:nl:execute:bid-all:bid-all:") {
		t.Fatalf("unexpected prompt cache key: %q", key)
	}
	if strings.Contains(key, " ") || strings.Contains(key, "/") {
		t.Fatalf("expected sanitized key: %q", key)
	}
}

func TestResolvePromptCacheRetentionDefault(t *testing.T) {
	t.Setenv("CLAWX_PROMPT_CACHE_RETENTION", "")
	if got := resolvePromptCacheRetention(); got != "in_memory" {
		t.Fatalf("unexpected default retention: %q", got)
	}
}
