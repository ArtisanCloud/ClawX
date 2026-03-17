package integration

import (
	"context"
	"strings"
	"testing"
)

func TestScheduleCommandFlow(t *testing.T) {
	router, _ := newScheduleControlRouter(t)
	ctx := context.Background()
	conversationID := "schedule-flow-conv"
	windowID := "schedule-flow-window"
	routeKey := "discord:default:direct:user-a:thread:schedule"

	_, _ = router.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey)

	if _, err := router.HandleControlCommand(ctx, `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup`, conversationID, windowID, routeKey); err != nil {
		t.Fatalf("add schedule: %v", err)
	}
	list, err := router.HandleControlCommand(ctx, "/schedule list", conversationID, windowID, routeKey)
	if err != nil || !strings.Contains(list.Message, "image-cleanup") {
		t.Fatalf("list failed: %v message=%q", err, list.Message)
	}
	if _, err := router.HandleControlCommand(ctx, "/schedule pause image-cleanup", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("pause schedule: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/schedule resume image-cleanup", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("resume schedule: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/schedule run image-cleanup", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("run schedule: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/schedule remove image-cleanup", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("remove schedule: %v", err)
	}
}
