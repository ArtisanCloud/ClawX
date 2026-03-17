package contract

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clawx/internal/infrastructure/config"
)

func TestMemoryDigestContract(t *testing.T) {
	router, workspaceRoot := newMemoryCommandContractRouter(t, config.MemoryConfig{
		TokenBudget:    4096,
		OwnerAllowlist: []string{"owner-1"},
	})
	ctx := context.Background()
	conversationID := "memory-digest-contract-conversation"
	windowID := "memory-digest-contract-window"
	routeKey := "telegram:default:direct:owner-1"

	createAndUseMemoryProject(t, router, conversationID, windowID, routeKey)

	if _, err := router.HandleControlCommand(ctx, "/memory note digest-source-line", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("memory note before digest: %v", err)
	}
	digested, err := router.HandleControlCommand(ctx, "/memory digest", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("memory digest: %v", err)
	}
	if !strings.Contains(digested.Message, "status=completed") {
		t.Fatalf("digest response should include completed status, got: %q", digested.Message)
	}

	body, err := os.ReadFile(filepath.Join(workspaceRoot, "memo", "MEMORY.md"))
	if err != nil {
		t.Fatalf("read MEMORY.md: %v", err)
	}
	content := string(body)
	if !strings.Contains(content, "MEMORY DIGEST") {
		t.Fatalf("digest output not found in MEMORY.md, got: %q", content)
	}
}

func TestMemoryDigestContractRejectsACLInSharedSession(t *testing.T) {
	router, _ := newMemoryCommandContractRouter(t, config.MemoryConfig{
		TokenBudget:    4096,
		OwnerAllowlist: []string{"owner-1"},
	})
	ctx := context.Background()
	conversationID := "memory-digest-acl-contract-conversation"
	windowID := "memory-digest-acl-contract-window"
	sharedRouteKey := "telegram:default:channel:group-1"

	createAndUseMemoryProject(t, router, conversationID, windowID, sharedRouteKey)

	_, err := router.HandleControlCommand(ctx, "/memory digest", conversationID, windowID, sharedRouteKey)
	if err == nil {
		t.Fatalf("shared session digest should be rejected")
	}
	if categorizedCode(err) != "rejected_acl" {
		t.Fatalf("expected rejected_acl error, got %v", err)
	}
}
