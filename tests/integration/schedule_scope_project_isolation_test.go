package integration

import (
	"context"
	"testing"
)

func TestScheduleScopeProjectIsolation(t *testing.T) {
	router, _ := newScheduleControlRouter(t)
	ctx := context.Background()
	conversationID := "schedule-project-iso-conv"
	windowID := "schedule-project-iso-window"
	routeA := "discord:default:direct:user-a:thread:a"
	routeB := "discord:default:direct:user-a:thread:b"

	_, _ = router.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeA)
	_, _ = router.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeA)
	_, _ = router.HandleControlCommand(ctx, `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup`, conversationID, windowID, routeA)

	_, _ = router.HandleControlCommand(ctx, "/project create another 另一个项目", conversationID, windowID, routeB)
	_, _ = router.HandleControlCommand(ctx, "/project use another", conversationID, windowID, routeB)
	if _, err := router.HandleControlCommand(ctx, "/schedule status image-cleanup", conversationID, windowID, routeB); err == nil {
		t.Fatalf("expected project isolation error")
	}
}
