package skill

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

var ErrInvalidDefinition = errors.New("invalid skill definition")

type Source string

const (
	SourceUser      Source = "user"
	SourceWorkspace Source = "workspace"
	SourceBuiltin   Source = "builtin"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusInvalid  Status = "invalid"
	StatusDisabled Status = "disabled"
	StatusShadowed Status = "shadowed"
)

type Definition struct {
	Name            string
	Description     string
	Aliases         []string
	InstructionBody string
	Source          Source
	BaseDir         string
	ManifestPath    string
	LoadedAt        time.Time
}

func (d Definition) Normalize() (Definition, error) {
	out := d
	out.Name = NormalizeName(out.Name)
	out.Description = strings.TrimSpace(out.Description)
	out.BaseDir = strings.TrimSpace(out.BaseDir)
	out.ManifestPath = strings.TrimSpace(out.ManifestPath)
	out.InstructionBody = strings.TrimSpace(out.InstructionBody)
	out.Aliases = NormalizeAliases(out.Aliases)
	if out.LoadedAt.IsZero() {
		out.LoadedAt = time.Now().UTC()
	}
	if out.Name == "" || out.Description == "" || out.BaseDir == "" || out.ManifestPath == "" {
		return Definition{}, ErrInvalidDefinition
	}
	return out, nil
}

type CatalogEntry struct {
	Key        string
	SkillName  string
	Source     Source
	Status     Status
	ShadowedBy string
	Errors     []string
	Definition *Definition
	UpdatedAt  time.Time
}

func NewCatalogEntry(def Definition) CatalogEntry {
	name := NormalizeName(def.Name)
	key := BuildEntryKey(def.Source, name, def.ManifestPath)
	return CatalogEntry{
		Key:        key,
		SkillName:  name,
		Source:     def.Source,
		Status:     StatusActive,
		Definition: &def,
		UpdatedAt:  time.Now().UTC(),
	}
}

func BuildEntryKey(source Source, name, manifestPath string) string {
	cleanName := NormalizeName(name)
	cleanPath := filepath.Clean(strings.TrimSpace(manifestPath))
	return fmt.Sprintf("%s:%s:%s", strings.TrimSpace(string(source)), cleanName, cleanPath)
}

func SourcePriority(source Source) int {
	switch source {
	case SourceUser:
		return 3
	case SourceWorkspace:
		return 2
	case SourceBuiltin:
		return 1
	default:
		return 0
	}
}

func NormalizeName(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

func NormalizeAliases(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := NormalizeName(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
