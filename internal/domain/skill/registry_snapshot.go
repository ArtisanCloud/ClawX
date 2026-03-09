package skill

import (
	"sort"
	"strings"
	"time"
)

type RegistrySnapshot struct {
	Version     int64
	Entries     []CatalogEntry
	GeneratedAt time.Time
}

func EmptySnapshot() RegistrySnapshot {
	return RegistrySnapshot{
		Version:     0,
		Entries:     nil,
		GeneratedAt: time.Now().UTC(),
	}
}

func (s RegistrySnapshot) ActiveDefinitions() []Definition {
	result := make([]Definition, 0, len(s.Entries))
	for _, entry := range s.Entries {
		if entry.Status != StatusActive || entry.Definition == nil {
			continue
		}
		result = append(result, *entry.Definition)
	}
	sort.Slice(result, func(i, j int) bool {
		left := NormalizeName(result[i].Name)
		right := NormalizeName(result[j].Name)
		if left == right {
			return result[i].ManifestPath < result[j].ManifestPath
		}
		return left < right
	})
	return result
}

func (s RegistrySnapshot) FindEntryByName(name string) (CatalogEntry, bool) {
	target := NormalizeName(name)
	for _, entry := range s.Entries {
		if NormalizeName(entry.SkillName) == target {
			return entry, true
		}
	}
	return CatalogEntry{}, false
}

func (s RegistrySnapshot) FindActiveDefinition(name string) (Definition, bool) {
	target := NormalizeName(name)
	for _, entry := range s.Entries {
		if entry.Status != StatusActive || entry.Definition == nil {
			continue
		}
		if NormalizeName(entry.SkillName) == target {
			return *entry.Definition, true
		}
	}
	return Definition{}, false
}

func (s RegistrySnapshot) HasName(name string) bool {
	_, ok := s.FindEntryByName(name)
	return ok
}

func SortEntries(entries []CatalogEntry) {
	sort.Slice(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]
		ln := NormalizeName(left.SkillName)
		rn := NormalizeName(right.SkillName)
		if ln != rn {
			return ln < rn
		}
		ls := strings.TrimSpace(string(left.Source))
		rs := strings.TrimSpace(string(right.Source))
		if ls != rs {
			return SourcePriority(left.Source) > SourcePriority(right.Source)
		}
		return left.Key < right.Key
	})
}
