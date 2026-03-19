package configplan

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type PatchOperation string

const (
	PatchSetProfile   PatchOperation = "set_profile"
	PatchSetWorkspace                = "set_workspace"
	PatchSetTimeout                  = "set_timeout"
	PatchSetDefault                  = "set_default"
	PatchSetAgentID                  = "set_agent_id"
)

type Patch struct {
	Operation PatchOperation
	Value     string
	By        string
	Source    Source
	At        time.Time
}

func ApplyPatch(plan Plan, patch Patch) (Plan, error) {
	updated := plan.Normalize()
	if patch.At.IsZero() {
		patch.At = time.Now().UTC()
	}
	patch.By = strings.TrimSpace(patch.By)
	patch.Value = strings.TrimSpace(patch.Value)
	if strings.TrimSpace(string(patch.Source)) == "" {
		patch.Source = updated.Source
	}
	if updated.AgentUpsertOpts == nil {
		return Plan{}, fmt.Errorf("plan patch requires upsert-agent plan")
	}
	value := patch.Value
	switch patch.Operation {
	case PatchSetProfile:
		if value == "" {
			return Plan{}, fmt.Errorf("profile cannot be empty")
		}
		updated.AgentUpsertOpts.ProfileID = value
	case PatchSetWorkspace:
		if value == "" {
			return Plan{}, fmt.Errorf("workspace cannot be empty")
		}
		updated.AgentUpsertOpts.Workspace = value
	case PatchSetTimeout:
		timeout, err := strconv.Atoi(value)
		if err != nil || timeout <= 0 {
			return Plan{}, fmt.Errorf("timeout must be positive integer")
		}
		updated.AgentUpsertOpts.TimeoutSeconds = timeout
	case PatchSetDefault:
		updated.AgentUpsertOpts.SetAsDefault = parseBool(value)
	case PatchSetAgentID:
		if value == "" {
			return Plan{}, fmt.Errorf("agent id cannot be empty")
		}
		oldID := strings.TrimSpace(updated.AgentUpsertOpts.ID)
		oldWorkspace := strings.TrimSpace(updated.AgentUpsertOpts.Workspace)
		updated.AgentUpsertOpts.ID = value
		updated.AgentUpsertOpts.Workspace = rewriteWorkspaceOnAgentIDChange(oldWorkspace, oldID, value)
	default:
		return Plan{}, fmt.Errorf("unsupported patch operation %q", patch.Operation)
	}
	updated.Version++
	updated.Summary = buildPlanSummary(updated)
	updated = ApplySummaryOnPatch(updated, patch)
	if ShouldRebuildSummary(updated) {
		updated = RebuildSummary(updated, "summary_inconsistent_after_patch")
	}
	return updated, updated.Validate()
}

func buildPlanSummary(plan Plan) string {
	if plan.Kind == KindSetDefaultAgent {
		return fmt.Sprintf("设置默认 Agent 为 `%s`", strings.TrimSpace(plan.DefaultAgentID))
	}
	if plan.Kind == KindDeleteAgent && plan.AgentDeleteOpts != nil {
		return fmt.Sprintf("删除 Agent `%s` (delete_workspace=%t)", plan.AgentDeleteOpts.ID, plan.AgentDeleteOpts.DeleteWorkspace)
	}
	if plan.Kind == KindRenameAgent && plan.AgentRenameOpts != nil {
		return fmt.Sprintf("重命名 Agent `%s` -> `%s` (migrate_workspace=%t, workspace=`%s`)",
			plan.AgentRenameOpts.FromID,
			plan.AgentRenameOpts.ToID,
			plan.AgentRenameOpts.MigrateWorkspace,
			plan.AgentRenameOpts.TargetWorkspace,
		)
	}
	if plan.AgentUpsertOpts == nil {
		return strings.TrimSpace(plan.Summary)
	}
	opts := plan.AgentUpsertOpts
	return fmt.Sprintf("新增/更新 Agent `%s` (profile=`%s`, workspace=`%s`, timeout=%ds, default=%t)",
		opts.ID, opts.ProfileID, opts.Workspace, opts.TimeoutSeconds, opts.SetAsDefault)
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on", "默认", "是":
		return true
	default:
		return false
	}
}

func rewriteWorkspaceOnAgentIDChange(workspace, oldID, newID string) string {
	workspace = strings.TrimSpace(workspace)
	oldID = strings.TrimSpace(oldID)
	newID = strings.TrimSpace(newID)
	if workspace == "" || oldID == "" || newID == "" || oldID == newID {
		return workspace
	}
	if strings.HasSuffix(workspace, "/"+oldID) {
		return strings.TrimSuffix(workspace, oldID) + newID
	}
	if strings.HasSuffix(workspace, "\\"+oldID) {
		return strings.TrimSuffix(workspace, oldID) + newID
	}
	return workspace
}
