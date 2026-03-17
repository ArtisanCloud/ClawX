package integration

import (
	"context"
	"strings"
	"testing"
)

func TestScheduleFailKeepActive(t *testing.T) {
	router, _ := newScheduleControlRouter(t)
	ctx := context.Background()
	conversationID := "schedule-fail-active-conv"
	windowID := "schedule-fail-active-window"
	routeKey := "discord:default:direct:user-a:thread:failactive"
	_, _ = router.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, `/schedule add bad-job --cron "0 3 * * 0" --task unsupported.task`, conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, "/schedule run bad-job", conversationID, windowID, routeKey)
	status, err := router.HandleControlCommand(ctx, "/schedule status bad-job", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("status bad job: %v", err)
	}
	if !strings.Contains(status.Message, "[active]") {
		t.Fatalf("job should remain active after single failure: %q", status.Message)
	}
}
