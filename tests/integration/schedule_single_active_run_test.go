package integration

import (
	"context"
	"testing"
)

func TestScheduleSingleActiveRun(t *testing.T) {
	router, _ := newScheduleControlRouter(t)
	ctx := context.Background()
	conversationID := "schedule-single-run-conv"
	windowID := "schedule-single-run-window"
	routeKey := "discord:default:direct:user-a:thread:single"

	_, _ = router.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup`, conversationID, windowID, routeKey)
	if _, err := router.HandleControlCommand(ctx, "/schedule run image-cleanup", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("run schedule: %v", err)
	}
}
