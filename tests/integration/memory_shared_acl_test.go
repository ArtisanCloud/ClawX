package integration

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"clawx/internal/application/command"
	"clawx/internal/infrastructure/config"
)

func TestMemorySharedSessionSkipsMainPrivate(t *testing.T) {
	ctx := context.Background()
	router, _, backendSpy, workspaceRoot := newMemoryExecutionRouterForIntegrationWithMemoryConfig(t, config.MemoryConfig{
		TokenBudget:    4096,
		OwnerAllowlist: []string{"owner-1"},
	})

	conversationID := "memory-shared-acl-conversation"
	routeKey := "telegram:default:channel:group-99"

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
		WindowID:        "window-shared",
		ProjectID:       "memo",
		RouteKey:        routeKey,
		UserID:          "owner-1",
		IsDirectMessage: false,
		Input:           "shared-task",
		Backend:         "agent-s",
		CWD:             ".",
	}); err != nil {
		t.Fatalf("execute shared flow: %v", err)
	}

	req := backendSpy.RequestAt(t, 0)
	if strings.Contains(req.MemoryContext, "MAIN_PRIVATE_SECRET") {
		t.Fatalf("shared session must not load main private memory")
	}
	if !strings.Contains(req.MemoryScope, "chat_mode=shared") {
		t.Fatalf("expected memory scope chat_mode=shared, got %q", req.MemoryScope)
	}
	if req.MemoryACLMode != "strict" {
		t.Fatalf("expected strict acl mode, got %q", req.MemoryACLMode)
	}
}
