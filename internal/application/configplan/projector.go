package configplan

import (
	"strconv"
	"strings"
	"time"
)

func ProjectSummaryFromPlan(plan Plan) ControlPlaneSummary {
	normalized := plan.Normalize()
	summary := ControlPlaneSummary{
		Version:       normalized.SummaryVersion,
		PatchCount:    len(normalized.PatchHistory),
		Fields:        make(map[SummaryField]SummaryFieldState),
		RebuiltAt:     time.Now().UTC(),
		RebuildReason: "projected_from_patch_history",
	}
	if summary.Version <= 0 {
		summary.Version = 1
	}

	applySnapshotToSummary(&summary, normalized)
	for _, patch := range normalized.PatchHistory {
		applyPatchMetadataToSummary(&summary, patch)
	}
	return summary
}

func applyPatchMetadataToSummary(summary *ControlPlaneSummary, patch Patch) {
	if summary == nil {
		return
	}
	field := patchOperationToSummaryField(patch.Operation)
	if field == "" {
		return
	}
	state := summary.Fields[field]
	state.LastPatchedBy = strings.TrimSpace(patch.By)
	state.LastPatchedAt = patch.At
	state.LastSource = patch.Source
	summary.Fields[field] = state

	summary.LastPatchedBy = state.LastPatchedBy
	summary.LastPatchedAt = state.LastPatchedAt
	summary.LastPatchSource = state.LastSource
}

func applySnapshotToSummary(summary *ControlPlaneSummary, plan Plan) {
	if summary == nil {
		return
	}
	switch plan.Kind {
	case KindUpsertAgent:
		if plan.AgentUpsertOpts == nil {
			return
		}
		setSummaryFieldValue(summary, SummaryFieldAgentID, plan.AgentUpsertOpts.ID)
		setSummaryFieldValue(summary, SummaryFieldProfile, plan.AgentUpsertOpts.ProfileID)
		setSummaryFieldValue(summary, SummaryFieldWorkspace, plan.AgentUpsertOpts.Workspace)
		setSummaryFieldValue(summary, SummaryFieldTimeout, strconv.Itoa(plan.AgentUpsertOpts.TimeoutSeconds))
		setSummaryFieldValue(summary, SummaryFieldDefault, strconv.FormatBool(plan.AgentUpsertOpts.SetAsDefault))
	case KindSetDefaultAgent:
		setSummaryFieldValue(summary, SummaryFieldDefault, plan.DefaultAgentID)
	case KindDeleteAgent:
		if plan.AgentDeleteOpts == nil {
			return
		}
		setSummaryFieldValue(summary, SummaryFieldAgentID, plan.AgentDeleteOpts.ID)
	case KindRenameAgent:
		if plan.AgentRenameOpts == nil {
			return
		}
		setSummaryFieldValue(summary, SummaryFieldAgentID, plan.AgentRenameOpts.ToID)
		setSummaryFieldValue(summary, SummaryFieldWorkspace, plan.AgentRenameOpts.TargetWorkspace)
	}
}

func setSummaryFieldValue(summary *ControlPlaneSummary, field SummaryField, value string) {
	state := summary.Fields[field]
	state.Value = strings.TrimSpace(value)
	summary.Fields[field] = state
}

func patchOperationToSummaryField(op PatchOperation) SummaryField {
	switch op {
	case PatchSetAgentID:
		return SummaryFieldAgentID
	case PatchSetProfile:
		return SummaryFieldProfile
	case PatchSetWorkspace:
		return SummaryFieldWorkspace
	case PatchSetTimeout:
		return SummaryFieldTimeout
	case PatchSetDefault:
		return SummaryFieldDefault
	default:
		return ""
	}
}
