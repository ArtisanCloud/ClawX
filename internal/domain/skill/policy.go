package skill

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidSkillPolicy = errors.New("invalid skill policy")

type VersionStrategy string

const (
	VersionStrategyPin        VersionStrategy = "pin"
	VersionStrategyCompatible VersionStrategy = "compatible"
	VersionStrategyLatest     VersionStrategy = "latest"
)

type SkillPolicy struct {
	AllowedSources      []RegistrySource
	BlockedSources      []RegistrySource
	VersionStrategy     VersionStrategy
	PinnedVersions      map[string]string
	DisabledSkills      []string
	AllowedCapabilities []string
	UpdatedAt           time.Time
}

func (p SkillPolicy) Normalize() SkillPolicy {
	out := p
	if out.UpdatedAt.IsZero() {
		out.UpdatedAt = time.Now().UTC()
	}
	if strings.TrimSpace(string(out.VersionStrategy)) == "" {
		out.VersionStrategy = VersionStrategyLatest
	}
	if out.PinnedVersions == nil {
		out.PinnedVersions = map[string]string{}
	}
	normalizedPins := make(map[string]string, len(out.PinnedVersions))
	for key, value := range out.PinnedVersions {
		k := strings.TrimSpace(key)
		v := strings.TrimSpace(value)
		if k == "" || v == "" {
			continue
		}
		normalizedPins[k] = v
	}
	out.PinnedVersions = normalizedPins
	out.DisabledSkills = normalizeStringList(out.DisabledSkills)
	out.AllowedCapabilities = normalizeStringList(out.AllowedCapabilities)
	out.AllowedSources = normalizeSourceList(out.AllowedSources)
	out.BlockedSources = normalizeSourceList(out.BlockedSources)
	return out
}

func (p SkillPolicy) Validate() error {
	normalized := p.Normalize()
	switch normalized.VersionStrategy {
	case VersionStrategyPin, VersionStrategyCompatible, VersionStrategyLatest:
	default:
		return ErrInvalidSkillPolicy
	}
	return nil
}

func (p SkillPolicy) SourceAllowed(source RegistrySource) bool {
	normalized := p.Normalize()
	src := RegistrySource(strings.TrimSpace(string(source)))
	for _, item := range normalized.BlockedSources {
		if item == src {
			return false
		}
	}
	if len(normalized.AllowedSources) == 0 {
		return true
	}
	for _, item := range normalized.AllowedSources {
		if item == src {
			return true
		}
	}
	return false
}

func (p SkillPolicy) SkillDisabled(skillID string) bool {
	target := strings.TrimSpace(skillID)
	if target == "" {
		return false
	}
	for _, item := range p.Normalize().DisabledSkills {
		if item == target {
			return true
		}
	}
	return false
}

func (p SkillPolicy) VersionAllowed(skillID, version string) bool {
	normalized := p.Normalize()
	skillID = strings.TrimSpace(skillID)
	version = strings.TrimSpace(version)
	if skillID == "" || version == "" {
		return false
	}
	switch normalized.VersionStrategy {
	case VersionStrategyLatest, VersionStrategyCompatible:
		return true
	case VersionStrategyPin:
		pinned, ok := normalized.PinnedVersions[skillID]
		if !ok {
			return false
		}
		return pinned == version
	default:
		return false
	}
}

func (p SkillPolicy) CapabilityAllowed(capability string) bool {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return true
	}
	allowed := p.Normalize().AllowedCapabilities
	if len(allowed) == 0 {
		return true
	}
	for _, item := range allowed {
		if item == capability {
			return true
		}
	}
	return false
}

func normalizeSourceList(values []RegistrySource) []RegistrySource {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[RegistrySource]struct{}, len(values))
	out := make([]RegistrySource, 0, len(values))
	for _, value := range values {
		item := RegistrySource(strings.TrimSpace(string(value)))
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
