package skillorchestrator

import (
	"context"
	"fmt"
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

type PolicyEngine struct {
	repository skilldomain.PolicyRepository
}

func NewPolicyEngine(repository skilldomain.PolicyRepository) *PolicyEngine {
	return &PolicyEngine{repository: repository}
}

func (e *PolicyEngine) Evaluate(ctx context.Context, metadata skilldomain.SkillMetadata) error {
	if e == nil {
		return fmt.Errorf("policy engine is nil")
	}
	policy := skilldomain.SkillPolicy{}.Normalize()
	if e.repository != nil {
		loaded, err := e.repository.Load(ctx)
		if err != nil {
			return err
		}
		policy = loaded.Normalize()
	}
	if !policy.SourceAllowed(metadata.Source) {
		return fmt.Errorf("source %q is not allowed by skill policy", metadata.Source)
	}
	if policy.SkillDisabled(metadata.SkillID) || !metadata.Enabled {
		return fmt.Errorf("skill %q is disabled", metadata.SkillID)
	}
	if !policy.VersionAllowed(metadata.SkillID, metadata.Version) {
		return fmt.Errorf("version %q for skill %q is rejected by policy", metadata.Version, metadata.SkillID)
	}
	for _, capability := range metadata.Capabilities {
		if !policy.CapabilityAllowed(capability) {
			return fmt.Errorf("capability %q is not allowed for skill %q", capability, metadata.SkillID)
		}
	}
	return nil
}

func EnforceCapabilityAllowlist(policy skilldomain.SkillPolicy, capability string) error {
	if !policy.CapabilityAllowed(strings.TrimSpace(capability)) {
		return fmt.Errorf("capability %q is not allowed", strings.TrimSpace(capability))
	}
	return nil
}
