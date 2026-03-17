package integration

import (
	"context"
	"strings"
	"testing"

	"clawx/internal/infrastructure/config"
)

func TestMemoryControlResumeSwitchRegression(t *testing.T) {
	ctx := context.Background()
	router, _, _, _ := newMemoryExecutionRouterForIntegrationWithMemoryConfig(t, config.MemoryConfig{
		TokenBudget:    4096,
		OwnerAllowlist: []string{"owner-1"},
	})

	conversationID := "memory-control-resume-switch-conversation"
	windowID := "memory-control-resume-switch-window"
	routeKey := "telegram:default:direct:owner-1"

	if _, err := router.HandleControlCommand(ctx, "/project create memo Memo", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use memo", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("use project: %v", err)
	}

	first, err := router.HandleControlCommand(ctx, "/new", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("new #1: %v", err)
	}
	second, err := router.HandleControlCommand(ctx, "/new", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("new #2: %v", err)
	}
	if !strings.HasPrefix(first.CreatedSessionID, "sess-memo-") || !strings.HasPrefix(second.CreatedSessionID, "sess-memo-") {
		t.Fatalf("session ids should remain project-scoped after memory integration: %s / %s", first.CreatedSessionID, second.CreatedSessionID)
	}
	if first.CreatedSessionID == second.CreatedSessionID {
		t.Fatalf("new should create distinct session ids")
	}

	resumed, err := router.HandleControlCommand(ctx, "/resume "+first.CreatedSessionID, conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if resumed.ResumedSessionID != first.CreatedSessionID {
		t.Fatalf("unexpected resumed id: got=%s want=%s", resumed.ResumedSessionID, first.CreatedSessionID)
	}
	currentAfterResume, err := router.HandleControlCommand(ctx, "/current", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("current after resume: %v", err)
	}
	if currentAfterResume.CurrentSession == nil || currentAfterResume.CurrentSession.ID != first.CreatedSessionID {
		t.Fatalf("resume should move current session to first; got=%+v", currentAfterResume.CurrentSession)
	}

	switched, err := router.HandleControlCommand(ctx, "/switch "+second.CreatedSessionID, conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("switch: %v", err)
	}
	if switched.SwitchedSessionID != second.CreatedSessionID {
		t.Fatalf("unexpected switched id: got=%s want=%s", switched.SwitchedSessionID, second.CreatedSessionID)
	}
	currentAfterSwitch, err := router.HandleControlCommand(ctx, "/current", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("current after switch: %v", err)
	}
	if currentAfterSwitch.CurrentSession == nil || currentAfterSwitch.CurrentSession.ID != second.CreatedSessionID {
		t.Fatalf("switch should move current session to second; got=%+v", currentAfterSwitch.CurrentSession)
	}
}
