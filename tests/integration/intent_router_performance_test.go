package integration

import (
	"context"
	"sort"
	"testing"
	"time"

	chatiface "synapsex/internal/interfaces/chat"
)

func TestIntentRouterDecisionPerformance(t *testing.T) {
	stack := newSkillTestStack(t, func(root string) error {
		writeSkillFile(t, root, "user-skills/echo/SKILL.md", `---
name: echo
description: echo text
aliases:
  - repeat
---
echo`)
		return nil
	}, nil)

	durations := make([]int64, 0, 200)
	for i := 0; i < 200; i++ {
		message := mustNormalizeMessage(t, chatiface.NormalizeInput{
			Channel:         "discord",
			UserID:          "user-1",
			Text:            "repeat payload",
			IsDirectMessage: true,
			IsAllowed:       true,
		})
		started := time.Now()
		if _, err := stack.router.Route(context.Background(), message); err != nil {
			t.Fatalf("route message: %v", err)
		}
		durations = append(durations, time.Since(started).Milliseconds())
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[int(float64(len(durations))*0.95)-1]
	if p95 > 100 {
		t.Fatalf("p95 routing too slow: %dms", p95)
	}
}
