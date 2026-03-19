package unit

import (
	"testing"
	"time"

	"clawx/internal/application/configplan"
)

func TestConfigPlanSummaryRebuildOnInvalidSummary(t *testing.T) {
	baseTime := time.Date(2026, 3, 19, 11, 0, 0, 0, time.UTC)
	plan := configplan.Plan{
		Kind:           configplan.KindUpsertAgent,
		ConversationID: "conv-rebuild",
		CreatedBy:      "owner",
		CreatedAt:      baseTime,
		Source:         configplan.SourceSlash,
		SummaryVersion: 3,
		AgentUpsertOpts: &configplan.UpsertAgentOptions{
			ID:             "bid-all",
			ProfileID:      "codex",
			Workspace:      "/home/ubuntu/.clawx/workspaces/bid-all",
			TimeoutSeconds: 900,
			SetAsDefault:   true,
		},
		PatchHistory: []configplan.Patch{
			{
				Operation: configplan.PatchSetTimeout,
				Value:     "900",
				By:        "u1",
				Source:    configplan.SourceNL,
				At:        baseTime.Add(1 * time.Minute),
			},
		},
		CompressedSummary: &configplan.ControlPlaneSummary{
			Version:    1,
			PatchCount: 1,
			Fields: map[configplan.SummaryField]configplan.SummaryFieldState{
				configplan.SummaryFieldAgentID: {
					Value: "bid-all",
				},
			},
		},
	}

	if !configplan.ShouldRebuildSummary(plan) {
		t.Fatalf("expected invalid summary to require rebuild")
	}

	rebuilt := configplan.RebuildSummary(plan, "unit_test_rebuild")
	if rebuilt.CompressedSummary == nil {
		t.Fatalf("expected rebuilt summary")
	}
	if rebuilt.CompressedSummary.RebuildReason != "unit_test_rebuild" {
		t.Fatalf("unexpected rebuild reason: %s", rebuilt.CompressedSummary.RebuildReason)
	}
	if rebuilt.SummaryVersion != rebuilt.CompressedSummary.Version {
		t.Fatalf("summary version mismatch: plan=%d summary=%d", rebuilt.SummaryVersion, rebuilt.CompressedSummary.Version)
	}
	if configplan.ShouldRebuildSummary(rebuilt) {
		t.Fatalf("rebuilt summary should be valid")
	}

	timeoutField, ok := rebuilt.CompressedSummary.Fields[configplan.SummaryFieldTimeout]
	if !ok {
		t.Fatalf("expected timeout field in rebuilt summary")
	}
	if timeoutField.Value != "900" {
		t.Fatalf("expected timeout value from final snapshot, got %q", timeoutField.Value)
	}
}
