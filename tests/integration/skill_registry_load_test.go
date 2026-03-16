package integration

import (
	"context"
	"strings"
	"testing"

	skilldomain "clawx/internal/domain/skill"
)

func TestSkillRegistryLoadAndConflictState(t *testing.T) {
	stack := newSkillTestStack(t, func(root string) error {
		writeSkillFile(t, root, "user-skills/echo/SKILL.md", `---
name: echo
description: user echo
aliases:
  - repeat
---
请回显用户输入`)
		writeSkillFile(t, root, "workspace/.clawx/skills/echo/SKILL.md", `---
name: echo
description: workspace echo
---
这是低优先级 echo`)
		writeSkillFile(t, root, "workspace/.clawx/skills/broken/SKILL.md", `---
name: broken
---
缺少 description`)
		return nil
	}, nil)

	result, err := stack.registry.Reload(context.Background())
	if err != nil {
		t.Fatalf("reload registry: %v", err)
	}
	if result.Entries < 3 {
		t.Fatalf("expected at least 3 entries, got %d", result.Entries)
	}

	var foundActive bool
	var foundShadowed bool
	var foundInvalid bool
	for _, entry := range stack.registry.List() {
		switch {
		case entry.SkillName == "echo" && entry.Status == skilldomain.StatusActive && entry.Source == skilldomain.SourceUser:
			foundActive = true
		case entry.SkillName == "echo" && entry.Status == skilldomain.StatusShadowed:
			foundShadowed = true
		case strings.Contains(entry.Key, "broken") && entry.Status == skilldomain.StatusInvalid:
			foundInvalid = true
		}
	}
	if !foundActive {
		t.Fatalf("expected user echo to be active")
	}
	if !foundShadowed {
		t.Fatalf("expected workspace echo to be shadowed")
	}
	if !foundInvalid {
		t.Fatalf("expected broken skill to be invalid")
	}
}
