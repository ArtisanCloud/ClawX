package integration

import (
	"context"
	"testing"

	"clawx/internal/application/service"
	chatiface "clawx/internal/interfaces/chat"
)

func TestNaturalLanguageServiceRouting(t *testing.T) {
	stack := newSkillTestStack(t, nil, nil)

	cases := []struct {
		text string
		want string
	}{
		{text: "请在当前项目启动图片工具服务", want: "/service start image-tool -- go run ./cmd/imagectl"},
		{text: "帮我看下图片工具服务状态", want: "/service status image-tool"},
		{text: "查看图片工具服务日志", want: "/service logs image-tool --tail=50"},
		{text: "停掉图片工具服务", want: "/service stop image-tool"},
	}

	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			message := mustNormalizeMessage(t, chatiface.NormalizeInput{
				Channel:         "discord",
				UserID:          "user-1",
				Text:            tc.text,
				IsDirectMessage: true,
				IsAllowed:       true,
			})
			decision, err := stack.router.Route(context.Background(), message)
			if err != nil {
				t.Fatalf("route natural language: %v", err)
			}
			if decision.Kind != service.DecisionControl {
				t.Fatalf("decision kind=%s, want control", decision.Kind)
			}
			if decision.Command != tc.want {
				t.Fatalf("decision command=%q, want %q", decision.Command, tc.want)
			}
		})
	}
}
