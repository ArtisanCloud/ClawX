package skillorchestrator

import (
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

type ResolveRequest struct {
	SkillID   string
	ProjectID string
	AgentID   string
}

type BindingResolver struct{}

func NewBindingResolver() *BindingResolver {
	return &BindingResolver{}
}

func (r *BindingResolver) Resolve(bindings []skilldomain.SkillBinding, request ResolveRequest) (skilldomain.SkillBinding, bool) {
	skillID := strings.TrimSpace(request.SkillID)
	if skillID == "" {
		return skilldomain.SkillBinding{}, false
	}
	agentID := strings.TrimSpace(request.AgentID)
	projectID := strings.TrimSpace(request.ProjectID)

	var (
		agentChoice   skilldomain.SkillBinding
		projectChoice skilldomain.SkillBinding
		globalChoice  skilldomain.SkillBinding
		hasAgent      bool
		hasProject    bool
		hasGlobal     bool
	)
	for _, raw := range bindings {
		item := raw.Normalize()
		if strings.TrimSpace(item.SkillID) != skillID {
			continue
		}
		switch item.Scope {
		case skilldomain.ScopeAgentLocal:
			if agentID != "" && item.AgentID == agentID {
				agentChoice = item
				hasAgent = true
			}
		case skilldomain.ScopeProject:
			if projectID != "" && item.ProjectID == projectID {
				projectChoice = item
				hasProject = true
			}
		case skilldomain.ScopeGlobal:
			globalChoice = item
			hasGlobal = true
		}
	}
	if hasAgent {
		return agentChoice, true
	}
	if hasProject {
		return projectChoice, true
	}
	if hasGlobal {
		return globalChoice, true
	}
	return skilldomain.SkillBinding{}, false
}
