package integration

import (
	"context"
	"testing"

	"clawx/internal/application/service"
	chatiface "clawx/internal/interfaces/chat"
)

func TestScheduleNaturalLanguageRouting(t *testing.T) {
	stack := newSkillTestStack(t, nil, nil)
	cases := []struct {
		text string
		want string
	}{
		{text: "每周清理图片记录", want: `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup --arg retention_days=30`},
		{text: "查看定时任务状态", want: "/schedule list"},
		{text: "立即执行图片清理任务", want: "/schedule run image-cleanup"},
	}
	for _, tc := range cases {
		message := mustNormalizeMessage(t, chatiface.NormalizeInput{Channel: "discord", UserID: "user-1", Text: tc.text, IsDirectMessage: true, IsAllowed: true})
		decision, err := stack.router.Route(context.Background(), message)
		if err != nil {
			t.Fatalf("route natural language: %v", err)
		}
		if decision.Kind != service.DecisionControl || decision.Command != tc.want {
			t.Fatalf("decision=%s %q, want control %q", decision.Kind, decision.Command, tc.want)
		}
	}
}

func TestScheduleNaturalLanguageImplementationRequestRoutesToExecute(t *testing.T) {
	stack := newSkillTestStack(t, nil, nil)
	text := "如果我现在想要定时清理上传图片并且自动通知我清理报告，你能实现么？"
	message := mustNormalizeMessage(t, chatiface.NormalizeInput{Channel: "discord", UserID: "user-1", Text: text, IsDirectMessage: true, IsAllowed: true})

	decision, err := stack.router.Route(context.Background(), message)
	if err != nil {
		t.Fatalf("route natural language implementation request: %v", err)
	}
	if decision.Kind != service.DecisionExecute {
		t.Fatalf("decision=%s %q, want execute", decision.Kind, decision.Command)
	}
}
