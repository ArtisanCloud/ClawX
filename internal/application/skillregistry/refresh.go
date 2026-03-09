package skillregistry

import (
	"context"
	"fmt"
	"strings"
	"time"

	skilldomain "synapsex/internal/domain/skill"
	skillsinfra "synapsex/internal/infrastructure/skills"
)

func BuildSnapshot(
	ctx context.Context,
	sources []skillsinfra.SourceSpec,
	disabled map[string]struct{},
	nextVersion int64,
) (skilldomain.RegistrySnapshot, error) {
	select {
	case <-ctx.Done():
		return skilldomain.RegistrySnapshot{}, ctx.Err()
	default:
	}

	candidates, err := skillsinfra.DiscoverCandidates(sources)
	if err != nil {
		return skilldomain.RegistrySnapshot{}, err
	}

	entries := make([]skilldomain.CatalogEntry, 0, len(candidates))
	for _, candidate := range candidates {
		select {
		case <-ctx.Done():
			return skilldomain.RegistrySnapshot{}, ctx.Err()
		default:
		}

		parsed, parseErr := skillsinfra.ParseManifest(candidate.ManifestPath, candidate.Source)
		if parseErr != nil {
			name := inferSkillName(candidate.BaseDir)
			entry := skilldomain.CatalogEntry{
				Key:       skilldomain.BuildEntryKey(candidate.Source, name, candidate.ManifestPath),
				SkillName: name,
				Source:    candidate.Source,
				Status:    skilldomain.StatusInvalid,
				Errors:    []string{parseErr.Error()},
				UpdatedAt: time.Now().UTC(),
			}
			entries = append(entries, entry)
			continue
		}

		entry := skilldomain.NewCatalogEntry(parsed.Definition)
		if _, ok := disabled[entry.SkillName]; ok {
			entry.Status = skilldomain.StatusDisabled
		}
		entries = append(entries, entry)
	}

	entries = resolveConflicts(entries)
	skilldomain.SortEntries(entries)

	snapshot := skilldomain.RegistrySnapshot{
		Version:     maxInt64(nextVersion, 1),
		Entries:     entries,
		GeneratedAt: time.Now().UTC(),
	}
	return snapshot, nil
}

func inferSkillName(baseDir string) string {
	base := strings.TrimSpace(baseDir)
	if base == "" {
		return "unknown"
	}
	parts := strings.Split(strings.Trim(base, "/"), "/")
	if len(parts) == 0 {
		return "unknown"
	}
	name := strings.TrimSpace(parts[len(parts)-1])
	if name == "" {
		return "unknown"
	}
	return skilldomain.NormalizeName(name)
}

func maxInt64(left, right int64) int64 {
	if left >= right {
		return left
	}
	return right
}

func FormatList(entries []skilldomain.CatalogEntry) string {
	if len(entries) == 0 {
		return "暂无可见 Skill"
	}
	lines := make([]string, 0, len(entries)+1)
	lines = append(lines, "Skill 列表:")
	for _, entry := range entries {
		line := fmt.Sprintf("- %s [%s] source=%s", entry.SkillName, entry.Status, entry.Source)
		if entry.ShadowedBy != "" {
			line += fmt.Sprintf(" shadowed_by=%s", entry.ShadowedBy)
		}
		if len(entry.Errors) > 0 {
			line += fmt.Sprintf(" errors=%s", strings.Join(entry.Errors, "; "))
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
