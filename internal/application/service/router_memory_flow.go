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

type memorySessionLoad struct {
	PromptContext string
	MemoryScope   string
	MemoryACLMode string
}

func (r *Router) buildMemoryContextForSession(ctx context.Context, cmd command.SessionCommand, record session.Record) memorySessionLoad {
	defaultResult := memorySessionLoad{
		MemoryScope:   "-",
		MemoryACLMode: string(memorydomain.ACLModeStrict),
	}
	if r.memoryLoader == nil || r.scopeResolver == nil {
		return defaultResult
	}

	workspaceRoot := strings.TrimSpace(r.cfg.Projects.WorkspaceRoot)
	projectID := normalizeSessionProjectID(cmd.ProjectID)
	if workspaceRoot == "" || projectID == "" {
		return defaultResult
	}
	projectRoot := filepath.Join(workspaceRoot, projectID)
	if stat, err := os.Stat(projectRoot); err != nil || !stat.IsDir() {
		return defaultResult
	}
	guard, err := memoryapp.NewPathGuard(projectRoot)
	if err != nil {
		return defaultResult
	}

	agentID := strings.TrimSpace(record.AgentID)
	if agentID == "" {
		agentID = strings.TrimSpace(cmd.Backend)
	}
	if agentID == "" {
		agentID = strings.TrimSpace(r.cfg.DefaultAgentID)
	}
	if agentID == "" {
		agentID = "main"
	}

	routeKey := strings.TrimSpace(cmd.RouteKey)
	if routeKey == "" {
		routeKey = strings.TrimSpace(cmd.WindowID)
	}
	scope, err := r.scopeResolver.Resolve(memoryapp.ScopeInput{
		AgentID:   agentID,
		ProjectID: projectID,
		RouteKey:  routeKey,
		SessionID: record.ID,
	})
	if err != nil {
		return defaultResult
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
	acl := memoryapp.ApplyLoaderACL(memoryapp.LoaderACLInput{
		Profile:         profile,
		RouteKey:        routeKey,
		UserID:          cmd.UserID,
		IsDirectMessage: cmd.IsDirectMessage,
		OwnerAllowlist:  r.cfg.Memory.OwnerAllowlist,
	})
	profile = acl.Profile
	scope = profile.ScopeKey

	candidates, denied, err := memoryapp.BuildLayeredCandidates(scope, guard, time.Now().UTC())
	if err != nil {
		return memorySessionLoad{
			MemoryScope:   "-",
			MemoryACLMode: string(profile.ACLMode),
		}
	}

	result := memoryapp.LoaderOutput{}
	if strings.TrimSpace(record.BackendSessionID) == "" && len(candidates) > 0 {
		result, err = r.memoryLoader.Load(ctx, memoryapp.LoaderInput{
			ScopeKey:   scope,
			Profile:    profile,
			Candidates: candidates,
		})
		if err != nil {
			return memorySessionLoad{
				MemoryScope:   "-",
				MemoryACLMode: string(profile.ACLMode),
			}
		}
	}

	audit := memoryapp.BuildAuditFields(scope, profile, result, denied)
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

	return memorySessionLoad{
		PromptContext: strings.TrimSpace(result.PromptContext),
		MemoryScope:   audit.MemoryScope,
		MemoryACLMode: audit.MemoryACLMode,
	}
}
