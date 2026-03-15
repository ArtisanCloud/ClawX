package service

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"clawx/internal/application/command"
	memoryapp "clawx/internal/application/memory"
	memorydomain "clawx/internal/domain/memory"
	"clawx/internal/domain/session"
)

func (r *Router) buildMemoryContextForSession(ctx context.Context, cmd command.SessionCommand, record session.Record) string {
	if r.memoryLoader == nil || r.scopeResolver == nil {
		return ""
	}
	if strings.TrimSpace(record.BackendSessionID) != "" {
		return ""
	}

	workspaceRoot := strings.TrimSpace(r.cfg.Projects.WorkspaceRoot)
	projectID := normalizeSessionProjectID(cmd.ProjectID)
	if workspaceRoot == "" || projectID == "" {
		return ""
	}
	projectRoot := filepath.Join(workspaceRoot, projectID)
	if stat, err := os.Stat(projectRoot); err != nil || !stat.IsDir() {
		return ""
	}
	guard, err := memoryapp.NewPathGuard(projectRoot)
	if err != nil {
		return ""
	}

	agentID := strings.TrimSpace(record.AgentID)
	if agentID == "" {
		agentID = strings.TrimSpace(cmd.Backend)
	}
	if agentID == "" {
		agentID = strings.TrimSpace(r.cfg.DefaultAgentID)
	}

	scope, err := r.scopeResolver.Resolve(memoryapp.ScopeInput{
		AgentID:   agentID,
		ProjectID: projectID,
		RouteKey:  cmd.WindowID,
		SessionID: record.ID,
	})
	if err != nil {
		return ""
	}

	budget := r.cfg.Memory.TokenBudget
	if budget <= 0 {
		budget = 4096
	}
	profile := memorydomain.MemoryProfile{
		ScopeKey:         scope,
		LoadOrder:        []memorydomain.Layer{memorydomain.LayerAgentPrivate, memorydomain.LayerProjectShare, memorydomain.LayerMainPrivate},
		TokenBudget:      budget,
		ACLMode:          memorydomain.ACLModeStrict,
		AllowMainPrivate: true,
	}

	candidates, denied, err := memoryapp.BuildLayeredCandidates(scope, guard, time.Now().UTC())
	if err != nil {
		return ""
	}

	result := memoryapp.LoaderOutput{}
	if len(candidates) > 0 {
		result, err = r.memoryLoader.Load(ctx, memoryapp.LoaderInput{
			ScopeKey:   scope,
			Profile:    profile,
			Candidates: candidates,
		})
		if err != nil {
			return ""
		}
	}
	audit := memoryapp.BuildAuditFields(scope, profile, result, denied)
	if len(audit.DeniedFiles) > 0 || audit.ErrorSummary != "" {
		log.Printf(
			"memory_load_audit: project_id=%s agent_id=%s memory_scope=%q memory_acl_mode=%s cross_agent_denied=%d cross_project_denied=%d memory_loaded_files=%q memory_denied_files=%q error_summary=%q",
			scope.ProjectID,
			scope.AgentID,
			audit.MemoryScope,
			audit.MemoryACLMode,
			audit.CrossAgentDeniedCount,
			audit.CrossProjectDeniedCount,
			audit.LoadedFiles,
			audit.DeniedFiles,
			audit.ErrorSummary,
		)
	}
	if len(candidates) == 0 {
		return ""
	}
	return strings.TrimSpace(result.PromptContext)
}
