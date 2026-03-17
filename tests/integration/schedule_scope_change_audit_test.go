package integration

import (
	"context"
	"strings"
	"testing"
)

func TestScheduleScopeChangeAudit(t *testing.T) {
	router, _ := newScheduleControlRouter(t)
	ctx := context.Background()
	conversationID := "schedule-scope-change-conv"
	windowID := "schedule-scope-change-window"
	routeKey := "discord:default:direct:user-a:thread:scope"

	_, _ = router.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey)
	add, err := router.HandleControlCommand(ctx, `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup --route route:scope-a`, conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("add with route scope: %v", err)
	}
	if !strings.Contains(add.Message, "tz=UTC") {
		t.Fatalf("unexpected add message: %q", add.Message)
	}
}
