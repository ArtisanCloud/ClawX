package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clawx/internal/application/command"
)

func TestMemoryCrossProjectIsolationRejectsExternalPath(t *testing.T) {
	ctx := context.Background()
	router, _, backendSpy, workspaceRoot := newMemoryExecutionRouterForIntegration(t)

	conversationID := "memory-cross-project-conversation"
	routeKey := "telegram:default:direct:memory-cross-project-user"

	if _, err := router.HandleControlCommand(ctx, "/project create alpha Alpha", conversationID, "window-ctl", routeKey); err != nil {
		t.Fatalf("create alpha: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project create beta Beta", conversationID, "window-ctl", routeKey); err != nil {
		t.Fatalf("create beta: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use alpha", conversationID, "window-ctl", routeKey); err != nil {
		t.Fatalf("use alpha: %v", err)
	}

	betaPrivate := filepath.Join(workspaceRoot, "beta", ".agents", "agent-a", "MEMORY.md")
	writeMemoryFileForTest(t, betaPrivate, "# MEMORY\nBETA_SECRET\n")

	alphaPrivate := filepath.Join(workspaceRoot, "alpha", ".agents", "agent-a", "MEMORY.md")
	if err := os.MkdirAll(filepath.Dir(alphaPrivate), 0o755); err != nil {
		t.Fatalf("mkdir alpha private: %v", err)
	}
	_ = os.Remove(alphaPrivate)
	if err := os.Symlink(betaPrivate, alphaPrivate); err != nil {
		t.Fatalf("create cross-project symlink: %v", err)
	}

	if _, err := router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeNew,
		ConversationID: conversationID,
		WindowID:       "window-alpha-a",
		ProjectID:      "alpha",
		Input:          "task-alpha",
		Backend:        "agent-a",
		CWD:            ".",
	}); err != nil {
		t.Fatalf("execute alpha flow: %v", err)
	}

	req := backendSpy.RequestAt(t, 0)
	if strings.Contains(req.MemoryContext, "BETA_SECRET") {
		t.Fatalf("memory context must not include cross-project secret")
	}
	if !strings.Contains(req.MemoryContext, "# SOUL") {
		t.Fatalf("memory context should still include alpha shared template")
	}
}
