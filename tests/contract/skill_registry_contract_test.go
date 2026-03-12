package contract

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"clawx/internal/application/skillregistry"
	skilldomain "clawx/internal/domain/skill"
	skillsinfra "clawx/internal/infrastructure/skills"
)

func TestSkillRegistryContractConflictAndInvalid(t *testing.T) {
	root := t.TempDir()
	userRoot := filepath.Join(root, "user")
	workspaceRoot := filepath.Join(root, "workspace")
	if err := os.MkdirAll(filepath.Join(userRoot, "echo"), 0o755); err != nil {
		t.Fatalf("mkdir user skill: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "echo"), 0o755); err != nil {
		t.Fatalf("mkdir workspace skill: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "broken"), 0o755); err != nil {
		t.Fatalf("mkdir broken skill: %v", err)
	}

	if err := os.WriteFile(filepath.Join(userRoot, "echo", "SKILL.md"), []byte(`---
name: echo
description: user echo
---
user`), 0o644); err != nil {
		t.Fatalf("write user skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "echo", "SKILL.md"), []byte(`---
name: echo
description: workspace echo
---
workspace`), 0o644); err != nil {
		t.Fatalf("write workspace skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "broken", "SKILL.md"), []byte(`---
name: broken
---
broken`), 0o644); err != nil {
		t.Fatalf("write broken skill: %v", err)
	}

	snapshot, err := skillregistry.BuildSnapshot(context.Background(), []skillsinfra.SourceSpec{
		{Source: skilldomain.SourceUser, Root: userRoot},
		{Source: skilldomain.SourceWorkspace, Root: workspaceRoot},
	}, nil, 1)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if len(snapshot.Entries) < 3 {
		t.Fatalf("expected 3 entries, got %d", len(snapshot.Entries))
	}

	activeCount := 0
	shadowedCount := 0
	invalidCount := 0
	for _, entry := range snapshot.Entries {
		if entry.SkillName == "echo" && entry.Status == skilldomain.StatusActive {
			activeCount++
		}
		if entry.SkillName == "echo" && entry.Status == skilldomain.StatusShadowed {
			shadowedCount++
		}
		if entry.Status == skilldomain.StatusInvalid {
			invalidCount++
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active echo, got %d", activeCount)
	}
	if shadowedCount != 1 {
		t.Fatalf("expected one shadowed echo, got %d", shadowedCount)
	}
	if invalidCount < 1 {
		t.Fatalf("expected at least one invalid skill")
	}
}
