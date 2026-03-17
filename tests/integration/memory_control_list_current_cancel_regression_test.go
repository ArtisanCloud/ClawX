package integration

import (
	"context"
	"strings"
	"testing"

	"clawx/internal/infrastructure/config"
)

func TestMemoryControlListCurrentCancelRegression(t *testing.T) {
	ctx := context.Background()
	router, _, _, _ := newMemoryExecutionRouterForIntegrationWithMemoryConfig(t, config.MemoryConfig{
		TokenBudget:    4096,
		OwnerAllowlist: []string{"owner-1"},
	})

	conversationID := "memory-control-list-current-cancel-conversation"
	windowID := "memory-control-list-current-cancel-window"
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

	listed, err := router.HandleControlCommand(ctx, "/list", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed.Sessions) < 2 {
		t.Fatalf("list should include at least 2 sessions, got %d", len(listed.Sessions))
	}
	for _, summary := range listed.Sessions {
		if !strings.HasPrefix(summary.ID, "sess-memo-") {
			t.Fatalf("list contains non-project session after memory integration: %s", summary.ID)
		}
	}

	current, err := router.HandleControlCommand(ctx, "/current", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if current.CurrentSession == nil || current.CurrentSession.ID != second.CreatedSessionID {
		t.Fatalf("current should point to latest session: got=%+v want=%s", current.CurrentSession, second.CreatedSessionID)
	}

	cancelled, err := router.HandleControlCommand(ctx, "/cancel", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.CancelledSessionID != second.CreatedSessionID {
		t.Fatalf("cancel target mismatch: got=%s want=%s", cancelled.CancelledSessionID, second.CreatedSessionID)
	}
	if !cancelled.CancelNoop {
		t.Fatalf("idle session cancel should remain noop")
	}

	if first.CreatedSessionID == second.CreatedSessionID {
		t.Fatalf("new sessions should remain unique")
	}
}
