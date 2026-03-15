package service

import (
	"context"
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
		LoadOrder:        []memorydomain.Layer{memorydomain.LayerProjectShare, memorydomain.LayerMainPrivate},
		TokenBudget:      budget,
		ACLMode:          memorydomain.ACLModeStrict,
		AllowMainPrivate: true,
	}

	candidates := buildProjectMemoryCandidates(projectRoot)
	if len(candidates) == 0 {
		return ""
	}

	result, err := r.memoryLoader.Load(ctx, memoryapp.LoaderInput{
		ScopeKey:   scope,
		Profile:    profile,
		Candidates: candidates,
	})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(result.PromptContext)
}

func buildProjectMemoryCandidates(projectRoot string) []memorydomain.MemoryLoadItem {
	ordered := []struct {
		layer memorydomain.Layer
		path  string
	}{
		{layer: memorydomain.LayerProjectShare, path: "IDENTITY.md"},
		{layer: memorydomain.LayerProjectShare, path: "SOUL.md"},
		{layer: memorydomain.LayerProjectShare, path: "USER.md"},
		{layer: memorydomain.LayerProjectShare, path: "TOOLS.md"},
		{layer: memorydomain.LayerProjectShare, path: "AGENTS.md"},
		{layer: memorydomain.LayerProjectShare, path: "HEARTBEAT.md"},
		{layer: memorydomain.LayerMainPrivate, path: "MEMORY.md"},
		{layer: memorydomain.LayerProjectShare, path: filepath.ToSlash(filepath.Join("memory", time.Now().UTC().Format("2006-01-02")+".md"))},
	}

	items := make([]memorydomain.MemoryLoadItem, 0, len(ordered))
	for idx, candidate := range ordered {
		absPath := filepath.Join(projectRoot, filepath.FromSlash(candidate.path))
		if stat, err := os.Stat(absPath); err == nil && !stat.IsDir() {
			items = append(items, memorydomain.MemoryLoadItem{
				Layer:    candidate.layer,
				Path:     absPath,
				Priority: idx + 1,
			})
		}
	}
	return items
}
