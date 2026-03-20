package unit

import (
	"testing"

	skilldomain "clawx/internal/domain/skill"
)

func TestSkillMetadataNormalizeAndValidate(t *testing.T) {
	metadata := skilldomain.SkillMetadata{
		SkillID:             " bid.collect ",
		Version:             " v1.0.0 ",
		Source:              skilldomain.RegistrySourceBuiltin,
		Capabilities:        []string{"crawl", "crawl", "extract"},
		InputSchema:         map[string]any{"type": "object"},
		RiskLevel:           skilldomain.RiskMedium,
		RequiredPermissions: []string{"workspace:read"},
		Enabled:             true,
	}
	metadata = metadata.Normalize()
	if err := metadata.Validate(); err != nil {
		t.Fatalf("validate metadata: %v", err)
	}
	if metadata.SkillID != "bid.collect" {
		t.Fatalf("unexpected skill id: %q", metadata.SkillID)
	}
	if len(metadata.Capabilities) != 2 {
		t.Fatalf("unexpected capabilities: %#v", metadata.Capabilities)
	}
}

func TestSkillPolicyChecks(t *testing.T) {
	policy := skilldomain.SkillPolicy{
		AllowedSources:      []skilldomain.RegistrySource{skilldomain.RegistrySourceBuiltin, skilldomain.RegistrySourceClawHub},
		VersionStrategy:     skilldomain.VersionStrategyPin,
		PinnedVersions:      map[string]string{"bid.collect": "v1.0.0"},
		DisabledSkills:      []string{"skill.disabled"},
		AllowedCapabilities: []string{"crawl", "extract"},
	}
	policy = policy.Normalize()
	if err := policy.Validate(); err != nil {
		t.Fatalf("validate policy: %v", err)
	}
	if !policy.SourceAllowed(skilldomain.RegistrySourceBuiltin) {
		t.Fatalf("builtin source should be allowed")
	}
	if policy.SourceAllowed(skilldomain.RegistrySourceGit) {
		t.Fatalf("git source should be rejected")
	}
	if !policy.VersionAllowed("bid.collect", "v1.0.0") {
		t.Fatalf("pinned version should be allowed")
	}
	if policy.VersionAllowed("bid.collect", "v2.0.0") {
		t.Fatalf("unpinned version should be rejected")
	}
	if !policy.SkillDisabled("skill.disabled") {
		t.Fatalf("disabled skill not recognized")
	}
	if !policy.CapabilityAllowed("crawl") || policy.CapabilityAllowed("delete") {
		t.Fatalf("capability allowlist behavior mismatch")
	}
}

func TestSkillBindingAndActionValidate(t *testing.T) {
	binding := skilldomain.SkillBinding{
		SkillID: "bid.collect",
		Version: "v1.0.0",
		Scope:   skilldomain.ScopeAgentLocal,
		AgentID: "bid-all",
	}
	if err := binding.Validate(); err != nil {
		t.Fatalf("validate binding: %v", err)
	}

	action := skilldomain.SkillAction{
		Intent:               "install_skill",
		SkillID:              "bid.collect",
		Arguments:            map[string]any{"agent_id": "bid-all"},
		Confidence:           0.91,
		RiskLevel:            skilldomain.RiskHigh,
		RequiresConfirmation: true,
		Source:               "nl",
	}
	if err := action.Validate(); err != nil {
		t.Fatalf("validate action: %v", err)
	}
}
