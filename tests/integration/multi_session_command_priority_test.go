package integration

import (
	"context"
	"testing"

	"clawx/internal/application/service"
	chatiface "clawx/internal/interfaces/chat"
)

func TestMultiSessionCommandPriorityBuiltInControlAlwaysWins(t *testing.T) {
	stack := newSkillTestStack(t, func(root string) error {
		writeSkillFile(t, root, "user-skills/switch/SKILL.md", `---
name: switch
description: conflicting name for control priority regression
---
this skill should never hijack built-in control command`)
		writeSkillFile(t, root, "user-skills/current/SKILL.md", `---
name: current
description: conflicting name for control priority regression
---
this skill should never hijack built-in control command`)
		return nil
	}, nil)

	cases := []string{
		"/new",
		"/resume sess-1",
		"/switch sess-1",
		"/list",
		"/current",
		"/cancel",
		"new",
		"resume sess-1",
		"switch sess-1",
		"list",
		"current",
		"cancel",
	}

	for _, text := range cases {
		t.Run(text, func(t *testing.T) {
			message := mustNormalizeMessage(t, chatiface.NormalizeInput{
				Channel:         "discord",
				UserID:          "user-1",
				Text:            text,
				WindowID:        "window-priority-us3",
				IsDirectMessage: true,
				IsAllowed:       true,
			})
			decision, err := stack.router.Route(context.Background(), message)
			if err != nil {
				t.Fatalf("route command %q: %v", text, err)
			}
			if decision.Kind != service.DecisionControl {
				t.Fatalf("command %q routed to %s, want control", text, decision.Kind)
			}
		})
	}
}
