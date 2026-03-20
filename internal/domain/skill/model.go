package skill

import (
	"errors"
	"sort"
	"strings"
	"time"
)

var ErrInvalidSkillMetadata = errors.New("invalid skill metadata")
var ErrSkillMetadataNotFound = errors.New("skill metadata not found")

type RegistrySource string

const (
	RegistrySourceBuiltin RegistrySource = "builtin"
	RegistrySourceClawHub RegistrySource = "clawhub"
	RegistrySourceLocal   RegistrySource = "local"
	RegistrySourceGit     RegistrySource = "git"
)

type SkillMetadata struct {
	SkillID             string
	Version             string
	Source              RegistrySource
	Capabilities        []string
	InputSchema         map[string]any
	RiskLevel           RiskLevel
	RequiredPermissions []string
	Enabled             bool
	UpdatedAt           time.Time
}

func (m SkillMetadata) Normalize() SkillMetadata {
	out := m
	out.SkillID = strings.TrimSpace(out.SkillID)
	out.Version = strings.TrimSpace(out.Version)
	out.Source = RegistrySource(strings.TrimSpace(string(out.Source)))
	out.Capabilities = normalizeStringList(out.Capabilities)
	out.RequiredPermissions = normalizeStringList(out.RequiredPermissions)
	if out.InputSchema == nil {
		out.InputSchema = map[string]any{}
	}
	if out.UpdatedAt.IsZero() {
		out.UpdatedAt = time.Now().UTC()
	}
	if strings.TrimSpace(string(out.RiskLevel)) == "" {
		out.RiskLevel = RiskLow
	}
	return out
}

func (m SkillMetadata) Validate() error {
	normalized := m.Normalize()
	if normalized.SkillID == "" || normalized.Version == "" {
		return ErrInvalidSkillMetadata
	}
	switch normalized.Source {
	case RegistrySourceBuiltin, RegistrySourceClawHub, RegistrySourceLocal, RegistrySourceGit:
	default:
		return ErrInvalidSkillMetadata
	}
	if err := normalized.RiskLevel.Validate(); err != nil {
		return err
	}
	return nil
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}
