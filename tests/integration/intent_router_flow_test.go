package integration

import (
	"context"
	"testing"

	"synapsex/internal/application/service"
	chatiface "synapsex/internal/interfaces/chat"
)

func TestIntentRouterFlowPriority(t *testing.T) {
	stack := newSkillTestStack(t, func(root string) error {
		writeSkillFile(t, root, "user-skills/echo/SKILL.md", `---
name: echo
description: echo text
aliases:
  - repeat
---
请回显输入`)
		return nil
	}, nil)

	cases := []struct {
		name string
		text string
		kind service.DecisionKind
	}{
		{name: "control", text: "/list", kind: service.DecisionControl},
		{name: "explicit skill", text: "/skill echo hello", kind: service.DecisionSkill},
		{name: "explicit sx skill", text: "/sx-skill echo hello", kind: service.DecisionSkill},
		{name: "exact name", text: "echo hello", kind: service.DecisionSkill},
		{name: "alias match", text: "repeat hello", kind: service.DecisionSkill},
		{name: "task fallback", text: "do something unrelated", kind: service.DecisionExecute},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message := mustNormalizeMessage(t, normalizeInput(tc.text))
			decision, err := stack.router.Route(context.Background(), message)
			if err != nil {
				t.Fatalf("route error: %v", err)
			}
			if decision.Kind != tc.kind {
				t.Fatalf("unexpected kind: got=%s want=%s", decision.Kind, tc.kind)
			}
		})
	}
}

func normalizeInput(text string) chatiface.NormalizeInput {
	return chatiface.NormalizeInput{
		Channel:         "discord",
		UserID:          "user-1",
		Text:            text,
		IsDirectMessage: true,
		IsAllowed:       true,
	}
}
