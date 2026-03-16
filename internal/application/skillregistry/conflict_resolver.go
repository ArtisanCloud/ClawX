package skillregistry

import (
	"sort"

	skilldomain "clawx/internal/domain/skill"
)

func resolveConflicts(entries []skilldomain.CatalogEntry) []skilldomain.CatalogEntry {
	if len(entries) == 0 {
		return nil
	}
	byName := make(map[string][]int)
	for idx := range entries {
		if entries[idx].SkillName == "" {
			continue
		}
		byName[skilldomain.NormalizeName(entries[idx].SkillName)] = append(byName[skilldomain.NormalizeName(entries[idx].SkillName)], idx)
	}

	for _, indices := range byName {
		if len(indices) <= 1 {
			continue
		}
		sort.Slice(indices, func(i, j int) bool {
			left := entries[indices[i]]
			right := entries[indices[j]]
			lp := skilldomain.SourcePriority(left.Source)
			rp := skilldomain.SourcePriority(right.Source)
			if lp != rp {
				return lp > rp
			}
			return left.Key < right.Key
		})

		winner := indices[0]
		for _, idx := range indices[1:] {
			entry := entries[idx]
			if entry.Status != skilldomain.StatusDisabled && entry.Status != skilldomain.StatusInvalid {
				entry.Status = skilldomain.StatusShadowed
				entry.ShadowedBy = entries[winner].Key
			}
			entries[idx] = entry
		}
	}
	return entries
}
