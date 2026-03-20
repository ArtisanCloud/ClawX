package skillorchestrator

import (
	"context"
	"fmt"
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

type ToggleService struct {
	policyRepo skilldomain.PolicyRepository
}

func NewToggleService(policyRepo skilldomain.PolicyRepository) *ToggleService {
	return &ToggleService{policyRepo: policyRepo}
}

func (s *ToggleService) Disable(ctx context.Context, skillID string) error {
	return s.updateDisabledSkill(ctx, skillID, true)
}

func (s *ToggleService) Enable(ctx context.Context, skillID string) error {
	return s.updateDisabledSkill(ctx, skillID, false)
}

func (s *ToggleService) updateDisabledSkill(ctx context.Context, skillID string, disabled bool) error {
	if s == nil || s.policyRepo == nil {
		return fmt.Errorf("toggle service is not configured")
	}
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return fmt.Errorf("skill id is required")
	}
	policy, err := s.policyRepo.Load(ctx)
	if err != nil {
		return err
	}
	policy = policy.Normalize()
	next := make([]string, 0, len(policy.DisabledSkills)+1)
	found := false
	for _, item := range policy.DisabledSkills {
		if strings.TrimSpace(item) == skillID {
			found = true
			if disabled {
				next = append(next, skillID)
			}
			continue
		}
		next = append(next, item)
	}
	if disabled && !found {
		next = append(next, skillID)
	}
	policy.DisabledSkills = next
	return s.policyRepo.Save(ctx, policy)
}
