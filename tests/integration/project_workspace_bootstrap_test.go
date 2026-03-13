package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectWorkspaceBootstrap(t *testing.T) {
	ctx := context.Background()
	router, _, workspaceRoot := newProjectControlRouterForIntegration(t)

	conversationID := "project-workspace-bootstrap-conversation"
	windowID := "project-workspace-bootstrap-window"
	routeKey := "telegram:default:direct:project-bootstrap-user-1"

	current, err := router.HandleControlCommand(ctx, "/project current", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("current default project: %v", err)
	}
	if !strings.Contains(current.Message, "当前项目: main [active]") {
		t.Fatalf("unexpected current response: %q", current.Message)
	}

	mainWorkspace := filepath.Join(workspaceRoot, "main")
	if stat, err := os.Stat(mainWorkspace); err != nil {
		t.Fatalf("main workspace should be created: %v", err)
	} else if !stat.IsDir() {
		t.Fatalf("main workspace should be a directory: %s", mainWorkspace)
	}

	created, err := router.HandleControlCommand(ctx, "/project create nba NBA", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("create nba project: %v", err)
	}
	if !strings.Contains(created.Message, "已创建项目: nba") {
		t.Fatalf("unexpected create response: %q", created.Message)
	}

	nbaWorkspace := filepath.Join(workspaceRoot, "nba")
	if stat, err := os.Stat(nbaWorkspace); err != nil {
		t.Fatalf("nba workspace should be created: %v", err)
	} else if !stat.IsDir() {
		t.Fatalf("nba workspace should be a directory: %s", nbaWorkspace)
	}

	listed, err := router.HandleControlCommand(ctx, "/project list", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if !strings.Contains(listed.Message, "main [active]") {
		t.Fatalf("list should include main active status: %q", listed.Message)
	}
	if !strings.Contains(listed.Message, "nba [active]") {
		t.Fatalf("list should include nba active status: %q", listed.Message)
	}
}
