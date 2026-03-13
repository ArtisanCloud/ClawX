package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	projectdomain "clawx/internal/domain/project"
)

func TestProjectRegistryBrokenStateDetection(t *testing.T) {
	ctx := context.Background()
	router, projectService, workspaceRoot := newProjectControlRouterForIntegration(t)

	conversationID := "project-registry-broken-conversation"
	windowID := "project-registry-broken-window"
	routeKey := "discord:discord-main:channel:guild-42:thread:thread-broken"

	if _, err := router.HandleControlCommand(ctx, "/project create bid Bid", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create bid project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use bid", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("use bid project: %v", err)
	}

	bidWorkspace := filepath.Join(workspaceRoot, "bid")
	if err := os.RemoveAll(bidWorkspace); err != nil {
		t.Fatalf("remove bid workspace: %v", err)
	}

	audit, err := router.HandleControlCommand(ctx, "/project audit", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("audit project state: %v", err)
	}
	if !strings.Contains(audit.Message, "broken=1") {
		t.Fatalf("audit should report broken project, got: %q", audit.Message)
	}
	if !strings.Contains(audit.Message, "project_broken") {
		t.Fatalf("audit should report broken binding issue, got: %q", audit.Message)
	}

	record, err := projectService.GetProject(ctx, "bid")
	if err != nil {
		t.Fatalf("get bid project: %v", err)
	}
	if record.Status != projectdomain.StatusBroken {
		t.Fatalf("project status should be broken after missing workspace: %s", record.Status)
	}

	_, err = router.HandleControlCommand(ctx, "/project current", conversationID, windowID, routeKey)
	if err == nil {
		t.Fatalf("current project should fail for broken binding")
	}
	if !errors.Is(err, projectdomain.ErrInvalidProject) {
		t.Fatalf("unexpected error for broken project current: %v", err)
	}
}
