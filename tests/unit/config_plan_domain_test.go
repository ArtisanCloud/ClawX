package unit

import (
	"testing"

	"clawx/internal/application/configplan"
)

func TestConfigPlanValidateAndNormalize(t *testing.T) {
	plan := configplan.Plan{
		Kind:           configplan.KindUpsertAgent,
		ConversationID: " conv-1 ",
		AgentUpsertOpts: &configplan.UpsertAgentOptions{
			ID:             " review ",
			ProfileID:      " codex ",
			TimeoutSeconds: 600,
		},
	}
	plan = plan.Normalize()
	if err := plan.Validate(); err != nil {
		t.Fatalf("validate plan: %v", err)
	}
	if plan.ConversationID != "conv-1" {
		t.Fatalf("unexpected conversation id: %q", plan.ConversationID)
	}
	if plan.AgentUpsertOpts.ID != "review" {
		t.Fatalf("unexpected agent id: %q", plan.AgentUpsertOpts.ID)
	}
}
