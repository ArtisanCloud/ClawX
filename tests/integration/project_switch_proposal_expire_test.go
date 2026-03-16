package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	projectdomain "clawx/internal/domain/project"
	chatiface "clawx/internal/interfaces/chat"
)

func TestProjectSwitchProposalExpire(t *testing.T) {
	ctx := context.Background()
	router, projectService := newProjectIntentRouter(t, time.Millisecond)

	conversationID := "project-switch-expire-conversation"
	windowID := "project-switch-expire-window"
	routeKey := "discord:discord-main:channel:guild-42:thread:thread-expire"

	if _, err := router.HandleControlCommand(ctx, "/project create bid Bid", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create bid project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project create nba NBA", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create nba project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use bid", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("bind route to bid: %v", err)
	}

	message := mustNormalizeMessage(t, chatiface.NormalizeInput{
		Channel:         "discord",
		InstanceID:      "discord-main",
		UserID:          "project-switch-expire-user",
		GuildID:         "guild-42",
		ThreadID:        "thread-expire",
		Text:            "请处理 project:nba 的任务",
		IsDirectMessage: false,
		IsThread:        true,
		IsAllowed:       true,
	})
	decision, err := router.Route(ctx, message)
	if err != nil {
		t.Fatalf("route intent message: %v", err)
	}
	if decision.Kind != "control" {
		t.Fatalf("expected suggestion control decision, got=%s", decision.Kind)
	}

	suggestion, err := router.HandleControlCommand(ctx, decision.Command, decision.ConversationID, decision.WindowID, decision.RouteKey)
	if err != nil {
		t.Fatalf("handle project suggest command: %v", err)
	}
	if !strings.Contains(suggestion.Message, "/project confirm ") {
		t.Fatalf("unexpected suggestion message: %q", suggestion.Message)
	}
	proposalID := extractProposalID(t, suggestion.Message)

	time.Sleep(5 * time.Millisecond)
	expired, err := projectService.ExpireProposals(ctx)
	if err != nil {
		t.Fatalf("expire proposals: %v", err)
	}
	if expired == 0 {
		t.Fatalf("expected at least one expired proposal")
	}

	_, err = router.HandleControlCommand(ctx, "/project confirm "+proposalID, conversationID, windowID, routeKey)
	if err == nil {
		t.Fatalf("confirm should fail for expired proposal")
	}
	if !errors.Is(err, projectdomain.ErrProposalExpired) && !errors.Is(err, projectdomain.ErrProposalInvalid) {
		t.Fatalf("unexpected confirm error: %v", err)
	}
}
