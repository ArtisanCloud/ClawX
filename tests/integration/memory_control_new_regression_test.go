package integration

import (
	"context"
	"strings"
	"testing"

	"clawx/internal/infrastructure/config"
)

func TestMemoryControlNewRegression(t *testing.T) {
	ctx := context.Background()
	router, _, _, _ := newMemoryExecutionRouterForIntegrationWithMemoryConfig(t, config.MemoryConfig{
		TokenBudget:    4096,
		OwnerAllowlist: []string{"owner-1"},
	})

	conversationID := "memory-control-new-regression-conversation"
	windowID := "memory-control-new-regression-window"
	routeKey := "telegram:default:direct:owner-1"

	if _, err := router.HandleControlCommand(ctx, "/project create bid Bid", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create bid project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project create nba NBA", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create nba project: %v", err)
	}

	if _, err := router.HandleControlCommand(ctx, "/project use bid", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("use bid: %v", err)
	}
	bidSession, err := router.HandleControlCommand(ctx, "/new", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("new under bid: %v", err)
	}
	if !strings.HasPrefix(bidSession.CreatedSessionID, "sess-bid-") {
		t.Fatalf("new session should remain project-scoped to bid: %s", bidSession.CreatedSessionID)
	}
	currentBidProject, err := router.HandleControlCommand(ctx, "/project current", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("project current bid: %v", err)
	}
	if !strings.Contains(currentBidProject.Message, "当前项目: bid") {
		t.Fatalf("project should stay bid after /new: %q", currentBidProject.Message)
	}

	if _, err := router.HandleControlCommand(ctx, "/project use nba", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("use nba: %v", err)
	}
	nbaSession, err := router.HandleControlCommand(ctx, "/new", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("new under nba: %v", err)
	}
	if !strings.HasPrefix(nbaSession.CreatedSessionID, "sess-nba-") {
		t.Fatalf("new session should remain project-scoped to nba: %s", nbaSession.CreatedSessionID)
	}
	if bidSession.CreatedSessionID == nbaSession.CreatedSessionID {
		t.Fatalf("sessions should remain distinct across projects")
	}
	currentNBAProject, err := router.HandleControlCommand(ctx, "/project current", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("project current nba: %v", err)
	}
	if !strings.Contains(currentNBAProject.Message, "当前项目: nba") {
		t.Fatalf("project should stay nba after /new: %q", currentNBAProject.Message)
	}
}
