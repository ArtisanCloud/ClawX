package integration

import (
	"context"
	"testing"
)

func TestScheduleFailureIsolation(t *testing.T) {
	router, _ := newScheduleControlRouter(t)
	ctx := context.Background()
	conversationID := "schedule-failure-iso-conv"
	windowID := "schedule-failure-iso-window"
	routeKey := "discord:default:direct:user-a:thread:failure"

	_, _ = router.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, `/schedule add bad-job --cron "0 3 * * 0" --task unsupported.task`, conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, `/schedule add good-job --cron "0 3 * * 0" --task image.cleanup`, conversationID, windowID, routeKey)
	if _, err := router.HandleControlCommand(ctx, "/schedule run bad-job", conversationID, windowID, routeKey); err == nil {
		t.Fatalf("unsupported task should fail")
	}
	if _, err := router.HandleControlCommand(ctx, "/schedule run good-job", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("good task should still run: %v", err)
	}
}
