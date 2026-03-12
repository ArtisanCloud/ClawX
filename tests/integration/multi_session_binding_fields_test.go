package integration

import (
	"context"
	"testing"
	"time"

	"clawx/internal/application/command"
	chatiface "clawx/internal/interfaces/chat"
)

func TestMultiSessionBindingFieldsPersistenceAndQuery(t *testing.T) {
	stack := newTestRuntime(t)
	ctx := context.Background()

	message, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
		Channel:         "telegram",
		UserID:          "user-binding-us3",
		Text:            "binding bootstrap",
		WindowID:        "window-binding-us3",
		IsDirectMessage: true,
		IsAllowed:       true,
	})
	if err != nil {
		t.Fatalf("normalize message: %v", err)
	}

	decision, err := stack.router.Route(ctx, message)
	if err != nil {
		t.Fatalf("route message: %v", err)
	}

	firstFlow, err := stack.router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: decision.ConversationID,
		WindowID:       decision.WindowID,
		Input:          decision.Message.Text,
		Backend:        "primary",
		CWD:            stack.cfg.DefaultCWD,
	})
	if err != nil {
		t.Fatalf("first flow: %v", err)
	}

	binding, err := stack.sessionManager.GetWindowBinding(ctx, decision.WindowID)
	if err != nil {
		t.Fatalf("get window binding: %v", err)
	}
	if binding.WindowID != decision.WindowID {
		t.Fatalf("unexpected window id: got=%q want=%q", binding.WindowID, decision.WindowID)
	}
	if binding.CurrentSessionID != firstFlow.Session.ID {
		t.Fatalf("unexpected current session id: got=%q want=%q", binding.CurrentSessionID, firstFlow.Session.ID)
	}
	if binding.ConversationID != decision.ConversationID {
		t.Fatalf("unexpected conversation id: got=%q want=%q", binding.ConversationID, decision.ConversationID)
	}
	if binding.UpdatedAt.IsZero() || binding.LastUsedAt.IsZero() {
		t.Fatalf("binding timestamps should not be zero: updated_at=%s last_used_at=%s", binding.UpdatedAt, binding.LastUsedAt)
	}

	bindings, err := stack.sessionManager.ListWindowBindingsByConversation(ctx, decision.ConversationID)
	if err != nil {
		t.Fatalf("list bindings by conversation: %v", err)
	}
	if len(bindings) == 0 {
		t.Fatalf("expected at least one window binding in conversation scope")
	}

	found := false
	for _, candidate := range bindings {
		if candidate.WindowID != decision.WindowID {
			continue
		}
		found = true
		if candidate.CurrentSessionID != firstFlow.Session.ID {
			t.Fatalf("listed current session id mismatch: got=%q want=%q", candidate.CurrentSessionID, firstFlow.Session.ID)
		}
		if candidate.ConversationID != decision.ConversationID {
			t.Fatalf("listed conversation id mismatch: got=%q want=%q", candidate.ConversationID, decision.ConversationID)
		}
		if candidate.UpdatedAt.IsZero() || candidate.LastUsedAt.IsZero() {
			t.Fatalf("listed timestamps should not be zero")
		}
	}
	if !found {
		t.Fatalf("expected binding for window %q in listed results", decision.WindowID)
	}

	previousUpdatedAt := binding.UpdatedAt
	previousLastUsedAt := binding.LastUsedAt

	time.Sleep(2 * time.Millisecond)
	_, err = stack.router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: decision.ConversationID,
		WindowID:       decision.WindowID,
		Input:          "binding refresh",
		Backend:        "primary",
		CWD:            stack.cfg.DefaultCWD,
	})
	if err != nil {
		t.Fatalf("second flow: %v", err)
	}

	updatedBinding, err := stack.sessionManager.GetWindowBinding(ctx, decision.WindowID)
	if err != nil {
		t.Fatalf("get updated binding: %v", err)
	}
	if !updatedBinding.UpdatedAt.After(previousUpdatedAt) {
		t.Fatalf("updated_at should move forward: prev=%s current=%s", previousUpdatedAt, updatedBinding.UpdatedAt)
	}
	if !updatedBinding.LastUsedAt.After(previousLastUsedAt) {
		t.Fatalf("last_used_at should move forward: prev=%s current=%s", previousLastUsedAt, updatedBinding.LastUsedAt)
	}
}
