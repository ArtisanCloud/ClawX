package unit

import (
	"testing"

	"clawx/internal/application/configplan"
)

func TestConfigPlanMemoryStoreSetGetPop(t *testing.T) {
	store := configplan.NewMemoryStore()
	plan := configplan.Plan{
		Kind:           configplan.KindSetDefaultAgent,
		ConversationID: "conv-1",
		DefaultAgentID: "main",
	}
	if err := store.Set("conv-1", plan); err != nil {
		t.Fatalf("set plan: %v", err)
	}
	got, ok := store.Get("conv-1")
	if !ok {
		t.Fatalf("expected plan")
	}
	if got.DefaultAgentID != "main" {
		t.Fatalf("unexpected default agent: %q", got.DefaultAgentID)
	}
	popped, ok := store.Pop("conv-1")
	if !ok {
		t.Fatalf("expected popped plan")
	}
	if popped.DefaultAgentID != "main" {
		t.Fatalf("unexpected popped agent: %q", popped.DefaultAgentID)
	}
	if _, ok := store.Get("conv-1"); ok {
		t.Fatalf("expected plan removed after pop")
	}
}
