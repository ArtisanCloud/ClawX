package contract

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clawx/internal/infrastructure/config"
)

func TestMemoryNoteSharedContract(t *testing.T) {
	router, workspaceRoot := newMemoryCommandContractRouter(t, config.MemoryConfig{
		TokenBudget:    4096,
		OwnerAllowlist: []string{"owner-1"},
	})
	ctx := context.Background()
	conversationID := "memory-note-shared-contract-conversation"
	windowID := "memory-note-shared-contract-window"
	directRouteKey := "telegram:default:direct:owner-1"

	createAndUseMemoryProject(t, router, conversationID, windowID, directRouteKey)

	sharedResult, err := router.HandleControlCommand(ctx, "/memory note --shared sync-team-note", conversationID, windowID, directRouteKey)
	if err != nil {
		t.Fatalf("shared note should be allowed in main owner session: %v", err)
	}
	if !strings.Contains(sharedResult.Message, "scope=project-shared") {
		t.Fatalf("shared note response should include project-shared scope, got: %q", sharedResult.Message)
	}

	daily := time.Now().UTC().Format("2006-01-02") + ".md"
	sharedPath := filepath.Join(workspaceRoot, "memo", "memory", daily)
	body, err := os.ReadFile(sharedPath)
	if err != nil {
		t.Fatalf("read shared note file: %v", err)
	}
	if !strings.Contains(string(body), "sync-team-note") {
		t.Fatalf("shared note file should include note content, got: %q", string(body))
	}
}

func TestMemoryNoteSharedContractRejectsNonMainSession(t *testing.T) {
	router, _ := newMemoryCommandContractRouter(t, config.MemoryConfig{
		TokenBudget:    4096,
		OwnerAllowlist: []string{"owner-1"},
	})
	ctx := context.Background()
	conversationID := "memory-note-shared-reject-contract-conversation"
	windowID := "memory-note-shared-reject-contract-window"
	sharedRouteKey := "telegram:default:channel:group-2"

	createAndUseMemoryProject(t, router, conversationID, windowID, sharedRouteKey)

	_, err := router.HandleControlCommand(ctx, "/memory note --shared should-reject", conversationID, windowID, sharedRouteKey)
	if err == nil {
		t.Fatalf("shared note should be rejected in shared session")
	}
	if categorizedCode(err) != "rejected_acl" {
		t.Fatalf("expected rejected_acl error, got %v", err)
	}
}
