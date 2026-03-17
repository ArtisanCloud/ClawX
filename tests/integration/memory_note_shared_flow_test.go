package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clawx/internal/infrastructure/config"
)

func TestMemoryNoteSharedFlow(t *testing.T) {
	ctx := context.Background()
	router, _, _, workspaceRoot := newMemoryExecutionRouterForIntegrationWithMemoryConfig(t, config.MemoryConfig{
		TokenBudget:    4096,
		OwnerAllowlist: []string{"owner-1"},
	})

	conversationID := "memory-note-shared-flow-conversation"
	windowID := "memory-note-shared-flow-window"
	directRouteKey := "telegram:default:direct:owner-1"
	sharedRouteKey := "telegram:default:channel:group-9"

	if _, err := router.HandleControlCommand(ctx, "/project create memo Memo", conversationID, windowID, directRouteKey); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use memo", conversationID, windowID, directRouteKey); err != nil {
		t.Fatalf("use project: %v", err)
	}

	allowed, err := router.HandleControlCommand(ctx, "/memory note --shared project-visible-note", conversationID, windowID, directRouteKey)
	if err != nil {
		t.Fatalf("shared note in main owner session: %v", err)
	}
	if !strings.Contains(allowed.Message, "scope=project-shared") {
		t.Fatalf("unexpected shared note response: %q", allowed.Message)
	}

	daily := time.Now().UTC().Format("2006-01-02") + ".md"
	sharedPath := filepath.Join(workspaceRoot, "memo", "memory", daily)
	body, err := os.ReadFile(sharedPath)
	if err != nil {
		t.Fatalf("read shared note path: %v", err)
	}
	if !strings.Contains(string(body), "project-visible-note") {
		t.Fatalf("shared note file missing content: %q", string(body))
	}

	_, err = router.HandleControlCommand(ctx, "/memory note --shared should-fail", conversationID, windowID, sharedRouteKey)
	if err == nil {
		t.Fatalf("shared note should fail in shared session")
	}
	if !strings.Contains(err.Error(), "rejected_acl") {
		t.Fatalf("expected rejected_acl error, got %v", err)
	}
}
