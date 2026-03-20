package contract

import (
	"context"
	"testing"

	"clawx/internal/application/skillregistry"
	skilldomain "clawx/internal/domain/skill"
	skillsinfra "clawx/internal/infrastructure/skills"
)

func TestSkillBuiltinWebFetchContract(t *testing.T) {
	snapshot, err := skillregistry.BuildSnapshot(context.Background(), []skillsinfra.SourceSpec{
		{Source: skilldomain.SourceBuiltin, Root: builtinSkillRootFromContract()},
	}, nil, 1)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	entry := findEntryByName(snapshot, "web-fetch")
	if entry == nil {
		t.Fatalf("web-fetch not found")
	}
	if entry.Definition == nil {
		t.Fatalf("web-fetch definition should not be nil")
	}
	if entry.Definition.Description == "" {
		t.Fatalf("web-fetch description should not be empty")
	}
	if len(entry.Definition.Aliases) == 0 {
		t.Fatalf("web-fetch aliases should not be empty")
	}
}
