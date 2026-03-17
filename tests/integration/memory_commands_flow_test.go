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

func TestMemoryCommandsFlow(t *testing.T) {
	ctx := context.Background()
	router, _, _, workspaceRoot := newMemoryExecutionRouterForIntegrationWithMemoryConfig(t, config.MemoryConfig{
		TokenBudget:    4096,
		OwnerAllowlist: []string{"owner-1"},
	})

	conversationID := "memory-command-flow-conversation"
	windowID := "memory-command-flow-window"
	routeKey := "telegram:default:direct:owner-1"

	if _, err := router.HandleControlCommand(ctx, "/project create memo Memo", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use memo", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("use project: %v", err)
	}

	noted, err := router.HandleControlCommand(ctx, "/memory note tune-image-threshold", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("memory note: %v", err)
	}
	if !strings.Contains(noted.Message, "scope=agent-private") {
		t.Fatalf("unexpected note response: %q", noted.Message)
	}

	digested, err := router.HandleControlCommand(ctx, "/memory digest", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("memory digest: %v", err)
	}
	if !strings.Contains(digested.Message, "status=completed") {
		t.Fatalf("unexpected digest response: %q", digested.Message)
	}

	audited, err := router.HandleControlCommand(ctx, "/memory audit", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("memory audit: %v", err)
	}
	if !strings.Contains(audited.Message, "acl_denied=") || !strings.Contains(audited.Message, "budget_skipped=") {
		t.Fatalf("unexpected audit response: %q", audited.Message)
	}

	daily := time.Now().UTC().Format("2006-01-02") + ".md"
	privatePath := filepath.Join(workspaceRoot, "memo", ".agents", "main", "memory", daily)
	privateBody, err := os.ReadFile(privatePath)
	if err != nil {
		t.Fatalf("read private note file: %v", err)
	}
	if !strings.Contains(string(privateBody), "tune-image-threshold") {
		t.Fatalf("private note file missing content: %q", string(privateBody))
	}

	memoryBody, err := os.ReadFile(filepath.Join(workspaceRoot, "memo", "MEMORY.md"))
	if err != nil {
		t.Fatalf("read MEMORY.md: %v", err)
	}
	if !strings.Contains(string(memoryBody), "MEMORY DIGEST") {
		t.Fatalf("MEMORY.md should contain digest section, got: %q", string(memoryBody))
	}
}
