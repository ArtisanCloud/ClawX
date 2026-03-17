package integration

import (
	"context"
	"testing"
)

func TestServiceScheduleCoexist(t *testing.T) {
	router, _ := newScheduleControlRouter(t)
	ctx := context.Background()
	conversationID := "service-schedule-coexist-conv"
	windowID := "service-schedule-coexist-window"
	routeKey := "discord:default:direct:user-a:thread:coexist"
	_, _ = router.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey)
	if _, err := router.HandleControlCommand(ctx, "/service start worker -- sleep 1", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("start service: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup`, conversationID, windowID, routeKey); err != nil {
		t.Fatalf("add schedule: %v", err)
	}
}
