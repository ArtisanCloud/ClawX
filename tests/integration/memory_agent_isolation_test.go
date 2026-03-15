package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clawx/internal/application/command"
)

func TestMemoryAgentIsolationSameProject(t *testing.T) {
	ctx := context.Background()
	router, _, backendSpy, workspaceRoot := newMemoryExecutionRouterForIntegration(t)

	conversationID := "memory-agent-isolation-conversation"
	routeKey := "telegram:default:direct:memory-agent-isolation-user"

	if _, err := router.HandleControlCommand(ctx, "/project create memo Memo", conversationID, "window-ctl", routeKey); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use memo", conversationID, "window-ctl", routeKey); err != nil {
		t.Fatalf("use project: %v", err)
	}

	projectRoot := filepath.Join(workspaceRoot, "memo")
	writeMemoryFileForTest(t, filepath.Join(projectRoot, ".agents", "agent-a", "MEMORY.md"), "# MEMORY\nAGENT_A_SECRET\n")
	writeMemoryFileForTest(t, filepath.Join(projectRoot, ".agents", "agent-b", "MEMORY.md"), "# MEMORY\nAGENT_B_SECRET\n")

	if _, err := router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeNew,
		ConversationID: conversationID,
		WindowID:       "window-agent-a",
		ProjectID:      "memo",
		Input:          "task-a",
		Backend:        "agent-a",
		CWD:            ".",
	}); err != nil {
		t.Fatalf("execute agent-a flow: %v", err)
	}
	if _, err := router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeNew,
		ConversationID: conversationID,
		WindowID:       "window-agent-b",
		ProjectID:      "memo",
		Input:          "task-b",
		Backend:        "agent-b",
		CWD:            ".",
	}); err != nil {
		t.Fatalf("execute agent-b flow: %v", err)
	}

	if backendSpy.RequestCount() != 2 {
		t.Fatalf("expected 2 backend requests, got %d", backendSpy.RequestCount())
	}
	reqA := backendSpy.RequestAt(t, 0)
	reqB := backendSpy.RequestAt(t, 1)
	if !strings.Contains(reqA.MemoryContext, "AGENT_A_SECRET") {
		t.Fatalf("agent-a context should include AGENT_A_SECRET")
	}
	if strings.Contains(reqA.MemoryContext, "AGENT_B_SECRET") {
		t.Fatalf("agent-a context must not include AGENT_B_SECRET")
	}
	if !strings.Contains(reqB.MemoryContext, "AGENT_B_SECRET") {
		t.Fatalf("agent-b context should include AGENT_B_SECRET")
	}
	if strings.Contains(reqB.MemoryContext, "AGENT_A_SECRET") {
		t.Fatalf("agent-b context must not include AGENT_A_SECRET")
	}

	privateIndex := strings.Index(reqA.MemoryContext, "AGENT_A_SECRET")
	sharedIndex := strings.Index(reqA.MemoryContext, "# SOUL")
	if privateIndex < 0 || sharedIndex < 0 || privateIndex > sharedIndex {
		t.Fatalf("expected agent-private memory to load before shared layer")
	}
}

func writeMemoryFileForTest(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir memory file directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write memory file: %v", err)
	}
}
