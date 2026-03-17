package integration

import (
	"context"
	"testing"
)

func TestMemoryScheduleCoexist(t *testing.T) {
	router, _ := newScheduleControlRouter(t)
	ctx := context.Background()
	conversationID := "memory-schedule-coexist-conv"
	windowID := "memory-schedule-coexist-window"
	routeKey := "discord:default:direct:user-a:thread:coexist2"
	_, _ = router.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey)
	if _, err := router.HandleControlCommand(ctx, "/memory note test-note", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("memory note: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup`, conversationID, windowID, routeKey); err != nil {
		t.Fatalf("add schedule: %v", err)
	}
}
