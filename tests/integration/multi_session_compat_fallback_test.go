package integration

import (
	"context"
	"strings"
	"testing"

	"synapsex/internal/application/command"
	chatiface "synapsex/internal/interfaces/chat"
)

func TestMultiSessionCompatFallbackWithoutExplicitWindowID(t *testing.T) {
	stack := newTestRuntime(t)
	ctx := context.Background()

	firstMessage, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
		Channel:         "telegram",
		UserID:          "user-compat-us3",
		Text:            "compat first",
		IsDirectMessage: true,
		IsAllowed:       true,
	})
	if err != nil {
		t.Fatalf("normalize first message: %v", err)
	}
	if !strings.HasPrefix(firstMessage.WindowID, "compat:") {
		t.Fatalf("expected compat window id, got %q", firstMessage.WindowID)
	}

	firstDecision, err := stack.router.Route(ctx, firstMessage)
	if err != nil {
		t.Fatalf("route first message: %v", err)
	}
	firstFlow, err := stack.router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: firstDecision.ConversationID,
		WindowID:       firstDecision.WindowID,
		Input:          firstDecision.Message.Text,
		Backend:        "primary",
		CWD:            stack.cfg.DefaultCWD,
	})
	if err != nil {
		t.Fatalf("handle first flow: %v", err)
	}

	secondMessage, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
		Channel:         "telegram",
		UserID:          "user-compat-us3",
		Text:            "compat second",
		IsDirectMessage: true,
		IsAllowed:       true,
	})
	if err != nil {
		t.Fatalf("normalize second message: %v", err)
	}
	if secondMessage.WindowID != firstMessage.WindowID {
		t.Fatalf("compat window id should stay stable in same conversation: first=%q second=%q", firstMessage.WindowID, secondMessage.WindowID)
	}

	secondDecision, err := stack.router.Route(ctx, secondMessage)
	if err != nil {
		t.Fatalf("route second message: %v", err)
	}
	secondFlow, err := stack.router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: secondDecision.ConversationID,
		WindowID:       secondDecision.WindowID,
		Input:          secondDecision.Message.Text,
		Backend:        "primary",
		CWD:            stack.cfg.DefaultCWD,
	})
	if err != nil {
		t.Fatalf("handle second flow: %v", err)
	}

	if secondFlow.Session.ID != firstFlow.Session.ID {
		t.Fatalf("compat fallback should continue same session: first=%q second=%q", firstFlow.Session.ID, secondFlow.Session.ID)
	}

	current, err := stack.router.HandleControlCommand(ctx, "/current", secondDecision.ConversationID)
	if err != nil {
		t.Fatalf("handle /current without explicit window id: %v", err)
	}
	if current.CurrentSession == nil || current.CurrentSession.ID != secondFlow.Session.ID {
		t.Fatalf("compat current session mismatch: got=%v want=%q", current.CurrentSession, secondFlow.Session.ID)
	}
}
