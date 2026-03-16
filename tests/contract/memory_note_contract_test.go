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

func TestMemoryNoteContract(t *testing.T) {
	router, workspaceRoot := newMemoryCommandContractRouter(t, config.MemoryConfig{
		TokenBudget:    4096,
		OwnerAllowlist: []string{"owner-1"},
	})
	ctx := context.Background()
	conversationID := "memory-note-contract-conversation"
	windowID := "memory-note-contract-window"
	routeKey := "telegram:default:direct:owner-1"

	createAndUseMemoryProject(t, router, conversationID, windowID, routeKey)

	noted, err := router.HandleControlCommand(ctx, "/memory note remember-agent-only", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("memory note: %v", err)
	}
	if !strings.Contains(noted.Message, "scope=agent-private") {
		t.Fatalf("note response should include private scope, got: %q", noted.Message)
	}

	daily := time.Now().UTC().Format("2006-01-02") + ".md"
	notePath := filepath.Join(workspaceRoot, "memo", ".agents", "main", "memory", daily)
	body, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("read note file: %v", err)
	}
	if !strings.Contains(string(body), "remember-agent-only") {
		t.Fatalf("note file should contain text, got: %q", string(body))
	}
}
