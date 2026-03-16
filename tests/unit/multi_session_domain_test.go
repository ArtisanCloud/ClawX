package unit

import (
	"context"
	"testing"

	"clawx/internal/application/command"
	"clawx/internal/application/service"
	"clawx/internal/infrastructure/persistence"
)

func TestMultiSessionDomainWindowIndependentBinding(t *testing.T) {
	repo := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(repo, repo, nil)
	ctx := context.Background()

	conversationID := "discord:-:thread:user-ms-1"

	a, err := manager.CreateSession(ctx, command.SessionCommand{
		Mode:           command.ModeNew,
		ConversationID: conversationID,
		WindowID:       "window-a",
		Backend:        "primary",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("create session A: %v", err)
	}

	b, err := manager.CreateSession(ctx, command.SessionCommand{
		Mode:           command.ModeNew,
		ConversationID: conversationID,
		WindowID:       "window-b",
		Backend:        "primary",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("create session B: %v", err)
	}
	if a.ID == b.ID {
		t.Fatalf("sessions should be distinct across windows, got %q", a.ID)
	}

	continuedA, err := manager.ContinueSession(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: conversationID,
		WindowID:       "window-a",
		Input:          "continue-a",
		Backend:        "primary",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("continue window-a: %v", err)
	}
	if continuedA.ID != a.ID {
		t.Fatalf("window-a should continue session A: got=%q want=%q", continuedA.ID, a.ID)
	}

	continuedB, err := manager.ContinueSession(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: conversationID,
		WindowID:       "window-b",
		Input:          "continue-b",
		Backend:        "primary",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("continue window-b: %v", err)
	}
	if continuedB.ID != b.ID {
		t.Fatalf("window-b should continue session B: got=%q want=%q", continuedB.ID, b.ID)
	}

	bindingA, err := manager.GetWindowBinding(ctx, "window-a")
	if err != nil {
		t.Fatalf("get binding for window-a: %v", err)
	}
	if bindingA.CurrentSessionID != a.ID {
		t.Fatalf("unexpected binding for window-a: got=%q want=%q", bindingA.CurrentSessionID, a.ID)
	}

	bindingB, err := manager.GetWindowBinding(ctx, "window-b")
	if err != nil {
		t.Fatalf("get binding for window-b: %v", err)
	}
	if bindingB.CurrentSessionID != b.ID {
		t.Fatalf("unexpected binding for window-b: got=%q want=%q", bindingB.CurrentSessionID, b.ID)
	}
}

func TestMultiSessionDomainContinueFallsBackToConversationAndCreatesBinding(t *testing.T) {
	repo := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(repo, repo, nil)
	ctx := context.Background()

	conversationID := "discord:-:thread:user-ms-2"
	base, err := manager.CreateSession(ctx, command.SessionCommand{
		Mode:           command.ModeNew,
		ConversationID: conversationID,
		WindowID:       "window-origin",
		Backend:        "primary",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("create base session: %v", err)
	}

	continued, err := manager.ContinueSession(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: conversationID,
		WindowID:       "window-fallback",
		Input:          "continue-fallback",
		Backend:        "primary",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("continue in fallback window: %v", err)
	}
	if continued.ID != base.ID {
		t.Fatalf("fallback should pick latest conversation session: got=%q want=%q", continued.ID, base.ID)
	}

	binding, err := manager.GetWindowBinding(ctx, "window-fallback")
	if err != nil {
		t.Fatalf("get fallback window binding: %v", err)
	}
	if binding.CurrentSessionID != base.ID {
		t.Fatalf("fallback window should bind base session: got=%q want=%q", binding.CurrentSessionID, base.ID)
	}
}
