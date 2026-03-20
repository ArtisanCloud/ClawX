package skillorchestrator

import (
	"context"
	"fmt"
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

type BindRequest struct {
	SkillID   string
	Version   string
	Scope     skilldomain.BindingScope
	ProjectID string
	AgentID   string
}

type BindingService struct {
	repository skilldomain.BindingRepository
	registry   *RegistryService
}

func NewBindingService(repository skilldomain.BindingRepository, registry *RegistryService) *BindingService {
	return &BindingService{repository: repository, registry: registry}
}

func (s *BindingService) Bind(ctx context.Context, request BindRequest) (skilldomain.SkillBinding, error) {
	if s == nil || s.repository == nil {
		return skilldomain.SkillBinding{}, fmt.Errorf("binding repository is not configured")
	}
	binding := skilldomain.SkillBinding{
		SkillID:   strings.TrimSpace(request.SkillID),
		Version:   strings.TrimSpace(request.Version),
		Scope:     request.Scope,
		ProjectID: strings.TrimSpace(request.ProjectID),
		AgentID:   strings.TrimSpace(request.AgentID),
	}.Normalize()

	if binding.Version == "" && s.registry != nil {
		metadata, err := s.registry.Get(ctx, binding.SkillID)
		if err == nil {
			binding.Version = strings.TrimSpace(metadata.Version)
		}
	}
	if err := binding.Validate(); err != nil {
		return skilldomain.SkillBinding{}, err
	}
	if err := s.repository.Upsert(ctx, binding); err != nil {
		return skilldomain.SkillBinding{}, err
	}
	return binding, nil
}

func (s *BindingService) Unbind(ctx context.Context, request BindRequest) error {
	if s == nil || s.repository == nil {
		return fmt.Errorf("binding repository is not configured")
	}
	binding := skilldomain.SkillBinding{
		SkillID:   strings.TrimSpace(request.SkillID),
		Version:   strings.TrimSpace(request.Version),
		Scope:     request.Scope,
		ProjectID: strings.TrimSpace(request.ProjectID),
		AgentID:   strings.TrimSpace(request.AgentID),
	}.Normalize()
	if err := binding.Validate(); err != nil {
		return err
	}
	return s.repository.Delete(ctx, binding)
}

func (s *BindingService) List(ctx context.Context, request ResolveRequest) ([]skilldomain.SkillBinding, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("binding repository is not configured")
	}
	all, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	projectID := strings.TrimSpace(request.ProjectID)
	agentID := strings.TrimSpace(request.AgentID)
	skillID := strings.TrimSpace(request.SkillID)
	out := make([]skilldomain.SkillBinding, 0, len(all))
	for _, item := range all {
		binding := item.Normalize()
		if skillID != "" && binding.SkillID != skillID {
			continue
		}
		switch binding.Scope {
		case skilldomain.ScopeGlobal:
			out = append(out, binding)
		case skilldomain.ScopeProject:
			if projectID != "" && binding.ProjectID == projectID {
				out = append(out, binding)
			}
		case skilldomain.ScopeAgentLocal:
			if agentID != "" && binding.AgentID == agentID {
				out = append(out, binding)
			}
		}
	}
	return out, nil
}

func (s *BindingService) Resolve(ctx context.Context, request ResolveRequest) (skilldomain.SkillBinding, bool) {
	items, err := s.List(ctx, request)
	if err != nil {
		return skilldomain.SkillBinding{}, false
	}
	resolver := NewBindingResolver()
	return resolver.Resolve(items, request)
}

func (s *BindingService) ListAll(ctx context.Context) ([]skilldomain.SkillBinding, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("binding repository is not configured")
	}
	return s.repository.List(ctx)
}
