package configplan

import (
	"strconv"
	"strings"
	"time"
)

func ApplySummaryOnPatch(plan Plan, patch Patch) Plan {
	updated := plan.Normalize()
	updated.PatchHistory = append(updated.PatchHistory, patch)

	if updated.CompressedSummary == nil {
		return RebuildSummary(updated, "init")
	}

	summary := cloneSummary(*updated.CompressedSummary)
	summary.PatchCount = len(updated.PatchHistory)
	summary.Version++
	summary.LastPatchedBy = strings.TrimSpace(patch.By)
	summary.LastPatchedAt = patch.At
	summary.LastPatchSource = patch.Source

	applySnapshotToSummary(&summary, updated)
	applyPatchMetadataToSummary(&summary, patch)

	updated.CompressedSummary = &summary
	updated.SummaryVersion = summary.Version
	return updated
}

func RebuildSummary(plan Plan, reason string) Plan {
	updated := plan.Normalize()
	summary := ProjectSummaryFromPlan(updated)
	summary.RebuildReason = strings.TrimSpace(reason)
	if summary.RebuildReason == "" {
		summary.RebuildReason = "manual_rebuild"
	}
	if summary.RebuiltAt.IsZero() {
		summary.RebuiltAt = time.Now().UTC()
	}
	summary.Version = maxInt64(summary.Version, updated.SummaryVersion+1)
	updated.CompressedSummary = &summary
	updated.SummaryVersion = summary.Version
	return updated
}

func ShouldRebuildSummary(plan Plan) bool {
	normalized := plan.Normalize()
	if normalized.CompressedSummary == nil {
		return true
	}
	if normalized.CompressedSummary.Version != normalized.SummaryVersion {
		return true
	}
	if normalized.CompressedSummary.Fields == nil {
		return true
	}
	if normalized.Kind == KindUpsertAgent {
		required := []SummaryField{
			SummaryFieldAgentID,
			SummaryFieldProfile,
			SummaryFieldWorkspace,
			SummaryFieldTimeout,
			SummaryFieldDefault,
		}
		for _, field := range required {
			if _, ok := normalized.CompressedSummary.Fields[field]; !ok {
				return true
			}
		}
	}
	return false
}

func FieldValueFromPlan(plan Plan, field SummaryField) string {
	normalized := plan.Normalize()
	if normalized.Kind == KindUpsertAgent && normalized.AgentUpsertOpts != nil {
		switch field {
		case SummaryFieldAgentID:
			return normalized.AgentUpsertOpts.ID
		case SummaryFieldProfile:
			return normalized.AgentUpsertOpts.ProfileID
		case SummaryFieldWorkspace:
			return normalized.AgentUpsertOpts.Workspace
		case SummaryFieldTimeout:
			return strconv.Itoa(normalized.AgentUpsertOpts.TimeoutSeconds)
		case SummaryFieldDefault:
			return strconv.FormatBool(normalized.AgentUpsertOpts.SetAsDefault)
		}
	}
	if normalized.Kind == KindSetDefaultAgent && field == SummaryFieldDefault {
		return normalized.DefaultAgentID
	}
	if normalized.Kind == KindDeleteAgent && normalized.AgentDeleteOpts != nil && field == SummaryFieldAgentID {
		return normalized.AgentDeleteOpts.ID
	}
	if normalized.Kind == KindRenameAgent && normalized.AgentRenameOpts != nil {
		switch field {
		case SummaryFieldAgentID:
			return normalized.AgentRenameOpts.ToID
		case SummaryFieldWorkspace:
			return normalized.AgentRenameOpts.TargetWorkspace
		}
	}
	return ""
}

func cloneSummary(in ControlPlaneSummary) ControlPlaneSummary {
	out := in
	out.Fields = make(map[SummaryField]SummaryFieldState, len(in.Fields))
	for key, value := range in.Fields {
		out.Fields[key] = value
	}
	return out
}

func maxInt64(a, b int64) int64 {
	if a >= b {
		return a
	}
	return b
}
