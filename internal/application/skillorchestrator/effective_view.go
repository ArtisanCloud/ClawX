package skillorchestrator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

type EffectiveSkill struct {
	SkillID    string
	Version    string
	Scope      skilldomain.BindingScope
	ProjectID  string
	AgentID    string
	Source     skilldomain.RegistrySource
	Enabled    bool
	Resolution string
}

type EffectiveViewService struct {
	bindingRepo skilldomain.BindingRepository
	registry    *RegistryService
	resolver    *BindingResolver
}

func NewEffectiveViewService(bindingRepo skilldomain.BindingRepository, registry *RegistryService) *EffectiveViewService {
	return &EffectiveViewService{
		bindingRepo: bindingRepo,
		registry:    registry,
		resolver:    NewBindingResolver(),
	}
}

func (s *EffectiveViewService) ListEffective(ctx context.Context, request ResolveRequest) ([]EffectiveSkill, error) {
	if s == nil || s.bindingRepo == nil {
		return nil, fmt.Errorf("binding repository is not configured")
	}
	bindings, err := s.bindingRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	bySkill := make(map[string][]skilldomain.SkillBinding)
	for _, item := range bindings {
		binding := item.Normalize()
		if binding.SkillID == "" {
			continue
		}
		bySkill[binding.SkillID] = append(bySkill[binding.SkillID], binding)
	}
	skillIDs := make([]string, 0, len(bySkill))
	for skillID := range bySkill {
		skillIDs = append(skillIDs, skillID)
	}
	sort.Strings(skillIDs)

	out := make([]EffectiveSkill, 0, len(skillIDs))
	for _, skillID := range skillIDs {
		resolved, ok := s.resolver.Resolve(bySkill[skillID], ResolveRequest{
			SkillID:   skillID,
			ProjectID: request.ProjectID,
			AgentID:   request.AgentID,
		})
		if !ok {
			continue
		}
		entry := EffectiveSkill{
			SkillID:   resolved.SkillID,
			Version:   resolved.Version,
			Scope:     resolved.Scope,
			ProjectID: resolved.ProjectID,
			AgentID:   resolved.AgentID,
			Enabled:   true,
		}
		if s.registry != nil {
			if metadata, regErr := s.registry.Get(ctx, skillID); regErr == nil {
				entry.Source = metadata.Source
				entry.Enabled = metadata.Enabled
			}
		}
		explain := ExplainResolution(bySkill[skillID], ResolveRequest{
			SkillID:   skillID,
			ProjectID: request.ProjectID,
			AgentID:   request.AgentID,
		})
		entry.Resolution = strings.TrimSpace(explain.Message)
		out = append(out, entry)
	}
	return out, nil
}
