package integration

import (
	"context"
	"testing"

	"clawx/internal/application/command"
	chatiface "clawx/internal/interfaces/chat"
)

func TestMultiSessionControlFlowListCurrentSwitch(t *testing.T) {
	stack := newTestRuntime(t)
	ctx := context.Background()

	message, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
		Channel:         "telegram",
		UserID:          "user-us2-1",
		Text:            "bootstrap",
		WindowID:        "window-us2",
		IsDirectMessage: true,
		IsAllowed:       true,
	})
	if err != nil {
		t.Fatalf("normalize message: %v", err)
	}
	conversationID := message.ConversationID
	windowID := message.WindowID

	createdA, err := stack.router.HandleControlCommand(ctx, "/new", conversationID, windowID)
	if err != nil {
		t.Fatalf("create session A: %v", err)
	}
	if createdA.CreatedSessionID == "" {
		t.Fatalf("expected created session id for A")
	}

	createdB, err := stack.router.HandleControlCommand(ctx, "/new", conversationID, windowID)
	if err != nil {
		t.Fatalf("create session B: %v", err)
	}
	if createdB.CreatedSessionID == "" || createdB.CreatedSessionID == createdA.CreatedSessionID {
		t.Fatalf("expected second distinct session id, got=%q", createdB.CreatedSessionID)
	}

	currentBeforeSwitch, err := stack.router.HandleControlCommand(ctx, "/current", conversationID, windowID)
	if err != nil {
		t.Fatalf("current before switch: %v", err)
	}
	if currentBeforeSwitch.CurrentSession == nil || currentBeforeSwitch.CurrentSession.ID != createdB.CreatedSessionID {
		t.Fatalf("expected current session to be B before switch")
	}

	listBeforeSwitch, err := stack.router.HandleControlCommand(ctx, "/list", conversationID, windowID)
	if err != nil {
		t.Fatalf("list before switch: %v", err)
	}
	if len(listBeforeSwitch.Sessions) < 2 {
		t.Fatalf("expected at least 2 sessions in list, got=%d", len(listBeforeSwitch.Sessions))
	}
	if listBeforeSwitch.CurrentSession == nil || listBeforeSwitch.CurrentSession.ID != createdB.CreatedSessionID {
		t.Fatalf("expected list current marker to be B before switch")
	}

	switchResult, err := stack.router.HandleControlCommand(ctx, "/switch "+createdA.CreatedSessionID, conversationID, windowID)
	if err != nil {
		t.Fatalf("switch to session A: %v", err)
	}
	if switchResult.SwitchedSessionID != createdA.CreatedSessionID {
		t.Fatalf("unexpected switched session id: got=%q want=%q", switchResult.SwitchedSessionID, createdA.CreatedSessionID)
	}

	currentAfterSwitch, err := stack.router.HandleControlCommand(ctx, "/current", conversationID, windowID)
	if err != nil {
		t.Fatalf("current after switch: %v", err)
	}
	if currentAfterSwitch.CurrentSession == nil || currentAfterSwitch.CurrentSession.ID != createdA.CreatedSessionID {
		t.Fatalf("expected current session to be A after switch")
	}

	listAfterSwitch, err := stack.router.HandleControlCommand(ctx, "/list", conversationID, windowID)
	if err != nil {
		t.Fatalf("list after switch: %v", err)
	}
	if listAfterSwitch.CurrentSession == nil || listAfterSwitch.CurrentSession.ID != createdA.CreatedSessionID {
		t.Fatalf("expected list current marker to be A after switch")
	}

	flowResult, err := stack.router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: conversationID,
		WindowID:       windowID,
		Input:          "continue-after-switch",
		Backend:        "primary",
		CWD:            stack.cfg.DefaultCWD,
	})
	if err != nil {
		t.Fatalf("continue after switch: %v", err)
	}
	if flowResult.Session.ID != createdA.CreatedSessionID {
		t.Fatalf("continue should route to switched session: got=%q want=%q", flowResult.Session.ID, createdA.CreatedSessionID)
	}
}
