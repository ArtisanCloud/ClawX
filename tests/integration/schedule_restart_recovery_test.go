package integration

import (
	"context"
	"testing"
)

func TestScheduleRestartRecovery(t *testing.T) {
	routerA, workspaceRoot := newScheduleControlRouter(t)
	ctx := context.Background()
	conversationID := "schedule-restart-conv"
	windowID := "schedule-restart-window"
	routeKey := "discord:default:direct:user-a:thread:restart"
	_, _ = routerA.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey)
	_, _ = routerA.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey)
	_, _ = routerA.HandleControlCommand(ctx, `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup`, conversationID, windowID, routeKey)

	routerB, _ := newScheduleControlRouterOnWorkspace(t, workspaceRoot)
	_, _ = routerB.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey)
	_, _ = routerB.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey)
	if _, err := routerB.HandleControlCommand(ctx, "/schedule status image-cleanup", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("schedule should recover after restart: %v", err)
	}
}
