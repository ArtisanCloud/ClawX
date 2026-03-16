package contract

import (
	"context"
	"strings"
	"testing"

	"clawx/internal/infrastructure/config"
)

func TestMemoryAuditContract(t *testing.T) {
	router, _ := newMemoryCommandContractRouter(t, config.MemoryConfig{
		TokenBudget:    4096,
		OwnerAllowlist: []string{"owner-1"},
	})
	ctx := context.Background()
	conversationID := "memory-audit-contract-conversation"
	windowID := "memory-audit-contract-window"
	routeKey := "telegram:default:direct:owner-1"

	createAndUseMemoryProject(t, router, conversationID, windowID, routeKey)

	if _, err := router.HandleControlCommand(ctx, "/memory note sensitive-audit-source", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("memory note: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/memory digest", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("memory digest: %v", err)
	}

	audited, err := router.HandleControlCommand(ctx, "/memory audit", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("memory audit: %v", err)
	}
	if !strings.Contains(audited.Message, "template_version=") {
		t.Fatalf("audit response missing template version: %q", audited.Message)
	}
	if !strings.Contains(audited.Message, "acl_denied=") {
		t.Fatalf("audit response missing acl stats: %q", audited.Message)
	}
	if !strings.Contains(audited.Message, "budget_skipped=") {
		t.Fatalf("audit response missing budget stats: %q", audited.Message)
	}
	if strings.Contains(audited.Message, "sensitive-audit-source") {
		t.Fatalf("audit response must not include memory raw content: %q", audited.Message)
	}
}
