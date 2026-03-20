package skillorchestrator

import (
	"context"
	"fmt"
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

type RegistryService struct {
	repository skilldomain.RegistryRepository
}

func NewRegistryService(repository skilldomain.RegistryRepository) *RegistryService {
	return &RegistryService{repository: repository}
}

func (s *RegistryService) Register(ctx context.Context, metadata skilldomain.SkillMetadata) error {
	if s == nil || s.repository == nil {
		return fmt.Errorf("registry repository is not configured")
	}
	normalized := metadata.Normalize()
	if err := normalized.Validate(); err != nil {
		return err
	}
	return s.repository.Upsert(ctx, normalized)
}

func (s *RegistryService) Get(ctx context.Context, skillID string) (skilldomain.SkillMetadata, error) {
	if s == nil || s.repository == nil {
		return skilldomain.SkillMetadata{}, fmt.Errorf("registry repository is not configured")
	}
	return s.repository.GetByID(ctx, strings.TrimSpace(skillID))
}

func (s *RegistryService) List(ctx context.Context) ([]skilldomain.SkillMetadata, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("registry repository is not configured")
	}
	return s.repository.List(ctx)
}

func (s *RegistryService) Delete(ctx context.Context, skillID string) error {
	if s == nil || s.repository == nil {
		return fmt.Errorf("registry repository is not configured")
	}
	return s.repository.Delete(ctx, strings.TrimSpace(skillID))
}
