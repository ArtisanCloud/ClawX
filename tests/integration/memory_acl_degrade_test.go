package integration

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"clawx/internal/application/command"
	"clawx/internal/infrastructure/config"
)

func TestMemoryACLConflictFallsBackToDegraded(t *testing.T) {
	ctx := context.Background()
	router, _, backendSpy, workspaceRoot := newMemoryExecutionRouterForIntegrationWithMemoryConfig(t, config.MemoryConfig{
		TokenBudget:    4096,
		OwnerAllowlist: []string{"owner-1"},
	})

	conversationID := "memory-acl-degrade-conversation"
	routeKey := "telegram:default:direct:owner-1"

	if _, err := router.HandleControlCommand(ctx, "/project create memo Memo", conversationID, "window-ctl", routeKey); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use memo", conversationID, "window-ctl", routeKey); err != nil {
		t.Fatalf("use project: %v", err)
	}
	writeMemoryFileForTest(t, filepath.Join(workspaceRoot, "memo", "MEMORY.md"), "# MEMORY\nMAIN_PRIVATE_SECRET\n")

	if _, err := router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:            command.ModeNew,
		ConversationID:  conversationID,
		WindowID:        "window-degrade",
		ProjectID:       "memo",
		RouteKey:        routeKey,
		UserID:          "owner-2", // conflict with route peer owner-1
		IsDirectMessage: true,
		Input:           "degrade-task",
		Backend:         "agent-d",
		CWD:             ".",
	}); err != nil {
		t.Fatalf("execute degrade flow: %v", err)
	}

	req := backendSpy.RequestAt(t, 0)
	if req.MemoryACLMode != "degraded" {
		t.Fatalf("expected degraded acl mode, got %q", req.MemoryACLMode)
	}
	if !strings.Contains(req.MemoryScope, "chat_mode=shared") {
		t.Fatalf("degraded policy should fallback to shared mode scope, got %q", req.MemoryScope)
	}
	if strings.Contains(req.MemoryContext, "MAIN_PRIVATE_SECRET") {
		t.Fatalf("degraded policy must not load main private memory")
	}
}
