package integration

import (
	"context"
	"strings"
	"testing"
)

func TestProjectNewSemantics(t *testing.T) {
	ctx := context.Background()
	router, projectService, _ := newProjectControlRouterForIntegration(t)

	conversationID := "project-new-semantics-conversation"
	windowID := "project-new-semantics-window"
	routeKey := "telegram:default:direct:project-new-user-1"

	if _, err := router.HandleControlCommand(ctx, "/project create bid Bid", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create bid project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use bid", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("bind route to bid: %v", err)
	}

	beforeProjectID, beforeMode, err := projectService.ResolveProject(ctx, routeKey)
	if err != nil {
		t.Fatalf("resolve project before /new: %v", err)
	}
	if beforeProjectID != "bid" || beforeMode != "binding" {
		t.Fatalf("unexpected project before /new: id=%s mode=%s", beforeProjectID, beforeMode)
	}

	created, err := router.HandleControlCommand(ctx, "/new", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("/new should create session: %v", err)
	}
	if created.CreatedSessionID == "" {
		t.Fatalf("/new should return created session id")
	}
	if !strings.HasPrefix(created.CreatedSessionID, "sess-bid-") {
		t.Fatalf("/new session should stay in bid project scope, got=%s", created.CreatedSessionID)
	}

	afterProjectID, afterMode, err := projectService.ResolveProject(ctx, routeKey)
	if err != nil {
		t.Fatalf("resolve project after /new: %v", err)
	}
	if afterProjectID != "bid" || afterMode != "binding" {
		t.Fatalf("/new should not switch project: id=%s mode=%s", afterProjectID, afterMode)
	}

	current, err := router.HandleControlCommand(ctx, "/project current", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("query current project: %v", err)
	}
	if !strings.Contains(current.Message, "当前项目: bid [active]") {
		t.Fatalf("unexpected current project response after /new: %q", current.Message)
	}
}
