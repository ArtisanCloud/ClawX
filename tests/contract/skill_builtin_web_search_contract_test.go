package contract

import (
	"context"
	"testing"

	"clawx/internal/application/skillregistry"
	skilldomain "clawx/internal/domain/skill"
	skillsinfra "clawx/internal/infrastructure/skills"
)

func TestSkillBuiltinWebSearchContract(t *testing.T) {
	snapshot, err := skillregistry.BuildSnapshot(context.Background(), []skillsinfra.SourceSpec{
		{Source: skilldomain.SourceBuiltin, Root: builtinSkillRootFromContract()},
	}, nil, 1)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	entry := findEntryByName(snapshot, "web-search")
	if entry == nil {
		t.Fatalf("web-search not found")
	}
	if entry.Definition == nil {
		t.Fatalf("web-search definition should not be nil")
	}
	if entry.Definition.Description == "" {
		t.Fatalf("web-search description should not be empty")
	}
	if len(entry.Definition.Aliases) == 0 {
		t.Fatalf("web-search aliases should not be empty")
	}
}

func findEntryByName(snapshot skilldomain.RegistrySnapshot, name string) *skilldomain.CatalogEntry {
	for idx := range snapshot.Entries {
		if snapshot.Entries[idx].SkillName == name {
			return &snapshot.Entries[idx]
		}
	}
	return nil
}
