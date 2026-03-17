package integration

import (
	"context"
	"testing"
)

func TestScheduleScopeAgentIsolation(t *testing.T) {
	routerA, workspaceRoot := newScheduleControlRouter(t)
	routerB, _ := newScheduleControlRouterOnWorkspaceWithAgent(t, workspaceRoot, "other")
	ctx := context.Background()
	conversationA := "schedule-agent-iso-conv-a"
	conversationB := "schedule-agent-iso-conv-b"
	windowID := "schedule-agent-iso-window"
	routeKey := "discord:default:direct:user-a:thread:agent"

	_, _ = routerA.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationA, windowID, routeKey)
	_, _ = routerA.HandleControlCommand(ctx, "/project use image_tools", conversationA, windowID, routeKey)
	_, _ = routerA.HandleControlCommand(ctx, `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup`, conversationA, windowID, routeKey)

	_, _ = routerB.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationB, windowID, routeKey)
	_, _ = routerB.HandleControlCommand(ctx, "/project use image_tools", conversationB, windowID, routeKey)

	if _, err := routerB.HandleControlCommand(ctx, "/schedule status image-cleanup", conversationB, windowID, routeKey); err == nil {
		t.Fatalf("expected agent isolation error")
	}
}
