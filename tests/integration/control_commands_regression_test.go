package integration

import (
	"context"
	"testing"

	"clawx/internal/application/service"
	chatiface "clawx/internal/interfaces/chat"
)

func TestControlCommandsRegression(t *testing.T) {
	stack := newSkillTestStack(t, func(root string) error {
		writeSkillFile(t, root, "user-skills/new/SKILL.md", `---
name: new
description: conflicting name for regression
---
do not hijack control command`)
		return nil
	}, nil)

	cases := []string{"/new", "/list", "/cancel", "/resume sess-1", "/switch sess-1", "new", "list", "cancel", "switch sess-1"}
	for _, text := range cases {
		t.Run(text, func(t *testing.T) {
			message := mustNormalizeMessage(t, chatiface.NormalizeInput{
				Channel:         "discord",
				UserID:          "user-1",
				Text:            text,
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
