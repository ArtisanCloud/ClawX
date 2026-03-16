package integration

import (
	"context"
	"strings"
	"testing"

	"clawx/internal/application/command"
	"clawx/internal/application/service"
	chatiface "clawx/internal/interfaces/chat"
)

func TestProjectRouteIsolation(t *testing.T) {
	ctx := context.Background()
	router, _, _ := newProjectControlRouterForIntegration(t)

	routeA := mustNormalizeMessage(t, chatiface.NormalizeInput{
		Channel:         "discord",
		InstanceID:      "discord-main",
		UserID:          "project-route-user",
		GuildID:         "guild-42",
		ThreadID:        "thread-a",
		Text:            "/project create bid Bid",
		IsDirectMessage: false,
		IsThread:        true,
		IsAllowed:       true,
	})
	routeB := mustNormalizeMessage(t, chatiface.NormalizeInput{
		Channel:         "discord",
		InstanceID:      "discord-main",
		UserID:          "project-route-user",
		GuildID:         "guild-42",
		ThreadID:        "thread-b",
		Text:            "/project create nba NBA",
		IsDirectMessage: false,
		IsThread:        true,
		IsAllowed:       true,
	})

	if _, err := router.HandleControlCommand(ctx, "/project create bid Bid", routeA.ConversationID, routeA.WindowID, routeA.RouteKey); err != nil {
		t.Fatalf("create bid project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project create nba NBA", routeA.ConversationID, routeA.WindowID, routeA.RouteKey); err != nil {
		t.Fatalf("create nba project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use bid", routeA.ConversationID, routeA.WindowID, routeA.RouteKey); err != nil {
		t.Fatalf("bind route A to bid: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use nba", routeB.ConversationID, routeB.WindowID, routeB.RouteKey); err != nil {
		t.Fatalf("bind route B to nba: %v", err)
	}

	routeA.Text = "task for bid"
	routeB.Text = "task for nba"

	decisionA, err := router.Route(ctx, routeA)
	if err != nil {
		t.Fatalf("route A decision: %v", err)
	}
	decisionB, err := router.Route(ctx, routeB)
	if err != nil {
		t.Fatalf("route B decision: %v", err)
	}
	if decisionA.Kind != service.DecisionExecute || decisionB.Kind != service.DecisionExecute {
		t.Fatalf("unexpected decision kinds: A=%s B=%s", decisionA.Kind, decisionB.Kind)
	}
	if decisionA.ProjectID != "bid" || decisionA.ProjectMode != "binding" {
		t.Fatalf("unexpected project for route A: id=%s mode=%s", decisionA.ProjectID, decisionA.ProjectMode)
	}
	if decisionB.ProjectID != "nba" || decisionB.ProjectMode != "binding" {
		t.Fatalf("unexpected project for route B: id=%s mode=%s", decisionB.ProjectID, decisionB.ProjectMode)
	}

	flowA1, err := router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: decisionA.ConversationID,
		WindowID:       decisionA.WindowID,
		ProjectID:      decisionA.ProjectID,
		Input:          decisionA.Message.Text,
		Backend:        "project-control",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("execute route A first request: %v", err)
	}
	flowB1, err := router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: decisionB.ConversationID,
		WindowID:       decisionB.WindowID,
		ProjectID:      decisionB.ProjectID,
		Input:          decisionB.Message.Text,
		Backend:        "project-control",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("execute route B first request: %v", err)
	}
	if !strings.HasPrefix(flowA1.Session.ID, "sess-bid-") {
		t.Fatalf("route A should use bid session key, got=%s", flowA1.Session.ID)
	}
	if !strings.HasPrefix(flowB1.Session.ID, "sess-nba-") {
		t.Fatalf("route B should use nba session key, got=%s", flowB1.Session.ID)
	}

	routeA.Text = "task for bid again"
	routeB.Text = "task for nba again"
	decisionA2, err := router.Route(ctx, routeA)
	if err != nil {
		t.Fatalf("route A second decision: %v", err)
	}
	decisionB2, err := router.Route(ctx, routeB)
	if err != nil {
		t.Fatalf("route B second decision: %v", err)
	}
	flowA2, err := router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: decisionA2.ConversationID,
		WindowID:       decisionA2.WindowID,
		ProjectID:      decisionA2.ProjectID,
		Input:          decisionA2.Message.Text,
		Backend:        "project-control",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("execute route A second request: %v", err)
	}
	flowB2, err := router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: decisionB2.ConversationID,
		WindowID:       decisionB2.WindowID,
		ProjectID:      decisionB2.ProjectID,
		Input:          decisionB2.Message.Text,
		Backend:        "project-control",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("execute route B second request: %v", err)
	}
	if flowA2.Session.ID != flowA1.Session.ID {
		t.Fatalf("route A should keep its own session: first=%s second=%s", flowA1.Session.ID, flowA2.Session.ID)
	}
	if flowB2.Session.ID != flowB1.Session.ID {
		t.Fatalf("route B should keep its own session: first=%s second=%s", flowB1.Session.ID, flowB2.Session.ID)
	}
	if flowA2.Session.ID == flowB2.Session.ID {
		t.Fatalf("two routes should not share session id: %s", flowA2.Session.ID)
	}
}
