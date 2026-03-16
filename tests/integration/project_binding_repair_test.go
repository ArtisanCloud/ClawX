package integration

import (
	"context"
	"strings"
	"testing"
)

func TestProjectBindingRepairCommands(t *testing.T) {
	ctx := context.Background()
	router, _, _ := newProjectControlRouterForIntegration(t)

	operatorConversation := "project-binding-repair-operator-conversation"
	operatorWindow := "project-binding-repair-operator-window"
	operatorRouteKey := "discord:discord-main:channel:guild-42:thread:thread-operator"
	targetRouteKey := "discord:discord-main:channel:guild-42:thread:thread-target"

	if _, err := router.HandleControlCommand(ctx, "/project create bid Bid", operatorConversation, operatorWindow, operatorRouteKey); err != nil {
		t.Fatalf("create bid project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project create nba NBA", operatorConversation, operatorWindow, operatorRouteKey); err != nil {
		t.Fatalf("create nba project: %v", err)
	}

	boundBid, err := router.HandleControlCommand(ctx, "/project bind "+targetRouteKey+" bid", operatorConversation, operatorWindow, operatorRouteKey)
	if err != nil {
		t.Fatalf("bind target route to bid: %v", err)
	}
	if !strings.Contains(boundBid.Message, "已绑定路由") {
		t.Fatalf("unexpected bind response: %q", boundBid.Message)
	}

	currentBid, err := router.HandleControlCommand(ctx, "/project current", operatorConversation, operatorWindow, targetRouteKey)
	if err != nil {
		t.Fatalf("current target route project after bind bid: %v", err)
	}
	if !strings.Contains(currentBid.Message, "当前项目: bid [active]") {
		t.Fatalf("unexpected current after bind bid: %q", currentBid.Message)
	}

	reboundNBA, err := router.HandleControlCommand(ctx, "/project bind "+targetRouteKey+" nba", operatorConversation, operatorWindow, operatorRouteKey)
	if err != nil {
		t.Fatalf("rebind target route to nba: %v", err)
	}
	if !strings.Contains(reboundNBA.Message, "-> nba") {
		t.Fatalf("unexpected rebind response: %q", reboundNBA.Message)
	}

	currentNBA, err := router.HandleControlCommand(ctx, "/project current", operatorConversation, operatorWindow, targetRouteKey)
	if err != nil {
		t.Fatalf("current target route project after rebind nba: %v", err)
	}
	if !strings.Contains(currentNBA.Message, "当前项目: nba [active]") {
		t.Fatalf("unexpected current after rebind nba: %q", currentNBA.Message)
	}

	unbound, err := router.HandleControlCommand(ctx, "/project unbind "+targetRouteKey, operatorConversation, operatorWindow, operatorRouteKey)
	if err != nil {
		t.Fatalf("unbind target route: %v", err)
	}
	if !strings.Contains(unbound.Message, "已解绑路由") {
		t.Fatalf("unexpected unbind response: %q", unbound.Message)
	}

	currentFallback, err := router.HandleControlCommand(ctx, "/project current", operatorConversation, operatorWindow, targetRouteKey)
	if err != nil {
		t.Fatalf("current target route project after unbind: %v", err)
	}
	if !strings.Contains(currentFallback.Message, "当前项目: main [active]") {
		t.Fatalf("unbind should fallback to main project: %q", currentFallback.Message)
	}
}
