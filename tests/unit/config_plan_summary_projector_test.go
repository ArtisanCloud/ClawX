package unit

import (
	"testing"
	"time"

	"clawx/internal/application/configplan"
)

func TestConfigPlanSummaryProjectorDeterministicFromPatchHistory(t *testing.T) {
	baseTime := time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)
	patches := []configplan.Patch{
		{
			Operation: configplan.PatchSetWorkspace,
			Value:     "/home/ubuntu/.clawx/workspaces/bid-all",
			By:        "u1",
			Source:    configplan.SourceNL,
			At:        baseTime,
		},
		{
			Operation: configplan.PatchSetTimeout,
			Value:     "900",
			By:        "u1",
			Source:    configplan.SourceNL,
			At:        baseTime.Add(1 * time.Minute),
		},
		{
			Operation: configplan.PatchSetProfile,
			Value:     "claude",
			By:        "u2",
			Source:    configplan.SourceSlash,
			At:        baseTime.Add(2 * time.Minute),
		},
	}

	planA := configplan.Plan{
		Kind:           configplan.KindUpsertAgent,
		ConversationID: "conv-sum-a",
		CreatedBy:      "owner",
		CreatedAt:      baseTime,
		Source:         configplan.SourceSlash,
		SummaryVersion: 7,
		AgentUpsertOpts: &configplan.UpsertAgentOptions{
			ID:             "bid-all",
			ProfileID:      "claude",
			Workspace:      "/home/ubuntu/.clawx/workspaces/bid-all",
			TimeoutSeconds: 900,
			SetAsDefault:   false,
		},
		PatchHistory: patches,
	}
	planB := planA
	planB.ConversationID = "conv-sum-b"

	s1 := configplan.ProjectSummaryFromPlan(planA)
	s2 := configplan.ProjectSummaryFromPlan(planB)

	if s1.Version != s2.Version || s1.PatchCount != s2.PatchCount || s1.LastPatchedBy != s2.LastPatchedBy || !s1.LastPatchedAt.Equal(s2.LastPatchedAt) || s1.LastPatchSource != s2.LastPatchSource {
		t.Fatalf("summary header should be deterministic: s1=%+v s2=%+v", s1, s2)
	}

	required := []configplan.SummaryField{
		configplan.SummaryFieldAgentID,
		configplan.SummaryFieldProfile,
		configplan.SummaryFieldWorkspace,
		configplan.SummaryFieldTimeout,
		configplan.SummaryFieldDefault,
	}
	for _, field := range required {
		f1, ok1 := s1.Fields[field]
		f2, ok2 := s2.Fields[field]
		if !ok1 || !ok2 {
			t.Fatalf("missing required field in summary: %s", field)
		}
		if f1.Value != f2.Value || f1.LastPatchedBy != f2.LastPatchedBy || !f1.LastPatchedAt.Equal(f2.LastPatchedAt) || f1.LastSource != f2.LastSource {
			t.Fatalf("field summary should be deterministic for %s: %+v %+v", field, f1, f2)
		}
	}
}
