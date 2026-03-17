package integration

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"clawx/internal/application/command"
	"clawx/internal/infrastructure/config"
)

func TestMemoryAuditFieldsSearch(t *testing.T) {
	ctx := context.Background()
	router, _, _, _ := newMemoryExecutionRouterForIntegrationWithMemoryConfig(t, config.MemoryConfig{
		TokenBudget:    4096,
		OwnerAllowlist: []string{"owner-1"},
	})

	conversationID := "memory-audit-fields-conversation"
	windowID := "memory-audit-fields-window"
	routeKey := "telegram:default:direct:owner-1"

	if _, err := router.HandleControlCommand(ctx, "/project create memo Memo", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use memo", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("use project: %v", err)
	}

	var captured bytes.Buffer
	originWriter := log.Writer()
	originFlags := log.Flags()
	log.SetOutput(&captured)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(originWriter)
		log.SetFlags(originFlags)
	}()

	if _, err := router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:            command.ModeNew,
		ConversationID:  conversationID,
		WindowID:        windowID,
		ProjectID:       "memo",
		RouteKey:        routeKey,
		UserID:          "owner-1",
		IsDirectMessage: true,
		Input:           "audit-fields-task",
		Backend:         "main",
		CWD:             ".",
	}); err != nil {
		t.Fatalf("session flow: %v", err)
	}

	logs := captured.String()
	if !strings.Contains(logs, "memory_load_audit:") {
		t.Fatalf("memory audit marker missing in logs: %q", logs)
	}
	if !strings.Contains(logs, "memory_loaded_files=") {
		t.Fatalf("memory_loaded_files field missing in logs: %q", logs)
	}
	if !strings.Contains(logs, "memory_denied_files=") {
		t.Fatalf("memory_denied_files field missing in logs: %q", logs)
	}
	if !strings.Contains(logs, "error_summary=") {
		t.Fatalf("error_summary field missing in logs: %q", logs)
	}
}
