package skillorchestrator

import (
	"context"
	"fmt"
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

type UpgradeService struct {
	registry *RegistryService
	policy   *PolicyEngine
}

func NewUpgradeService(registry *RegistryService, policy *PolicyEngine) *UpgradeService {
	return &UpgradeService{registry: registry, policy: policy}
}

func (s *UpgradeService) Upgrade(ctx context.Context, skillID, targetVersion string) (skilldomain.SkillMetadata, error) {
	if s == nil || s.registry == nil {
		return skilldomain.SkillMetadata{}, fmt.Errorf("upgrade service is not configured")
	}
	skillID = strings.TrimSpace(skillID)
	targetVersion = strings.TrimSpace(targetVersion)
	if skillID == "" || targetVersion == "" {
		return skilldomain.SkillMetadata{}, fmt.Errorf("skill id and target version are required")
	}
	current, err := s.registry.Get(ctx, skillID)
	if err != nil {
		return skilldomain.SkillMetadata{}, err
	}
	current.Version = targetVersion
	current = current.Normalize()
	if s.policy != nil {
		if err := s.policy.Evaluate(ctx, current); err != nil {
			return skilldomain.SkillMetadata{}, err
		}
	}
	if err := s.registry.Register(ctx, current); err != nil {
		return skilldomain.SkillMetadata{}, err
	}
	return current, nil
}
