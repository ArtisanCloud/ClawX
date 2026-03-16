package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clawx/internal/application/command"
)

func TestMemoryAgentPrivateBootstrapWhenMissing(t *testing.T) {
	ctx := context.Background()
	router, _, backendSpy, workspaceRoot := newMemoryExecutionRouterForIntegration(t)

	conversationID := "memory-agent-bootstrap-conversation"
	routeKey := "telegram:default:direct:memory-agent-bootstrap-user"
	agentID := "agent-z"

	if _, err := router.HandleControlCommand(ctx, "/project create memo Memo", conversationID, "window-ctl", routeKey); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use memo", conversationID, "window-ctl", routeKey); err != nil {
		t.Fatalf("use project: %v", err)
	}

	agentRoot := filepath.Join(workspaceRoot, "memo", ".agents", agentID)
	if err := os.RemoveAll(agentRoot); err != nil {
		t.Fatalf("cleanup agent root: %v", err)
	}

	if _, err := router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeNew,
		ConversationID: conversationID,
		WindowID:       "window-agent-z",
		ProjectID:      "memo",
		Input:          "bootstrap-agent-private",
		Backend:        agentID,
		CWD:            ".",
	}); err != nil {
		t.Fatalf("execute agent-z flow: %v", err)
	}

	for _, name := range []string{"IDENTITY.md", "TOOLS.md", "MEMORY.md"} {
		if _, err := os.Stat(filepath.Join(agentRoot, name)); err != nil {
			t.Fatalf("expected agent private file %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(agentRoot, "memory")); err != nil {
		t.Fatalf("expected agent private memory dir: %v", err)
	}

	req := backendSpy.RequestAt(t, 0)
	if !strings.Contains(req.MemoryContext, "agent-z") {
		t.Fatalf("memory context should include auto-bootstrapped agent identity")
	}
}
