package contract

import (
	"context"
	"path/filepath"
	goruntime "runtime"
	"testing"

	"clawx/internal/application/skillregistry"
	skilldomain "clawx/internal/domain/skill"
	skillsinfra "clawx/internal/infrastructure/skills"
)

func TestSkillBuiltinDiscoveryContract(t *testing.T) {
	snapshot, err := skillregistry.BuildSnapshot(context.Background(), []skillsinfra.SourceSpec{
		{Source: skilldomain.SourceBuiltin, Root: builtinSkillRootFromContract()},
	}, nil, 1)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	assertSkillPresent(t, snapshot, "web-search")
	assertSkillPresent(t, snapshot, "web-fetch")
}

func builtinSkillRootFromContract() string {
	_, file, _, _ := goruntime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "internal", "skills", "builtin"))
}

func assertSkillPresent(t *testing.T, snapshot skilldomain.RegistrySnapshot, name string) {
	t.Helper()
	for _, entry := range snapshot.Entries {
		if entry.SkillName == name {
			if entry.Status != skilldomain.StatusActive {
				t.Fatalf("skill %s should be active, got %s", name, entry.Status)
			}
			return
		}
	}
	t.Fatalf("skill %s not found in snapshot", name)
}
