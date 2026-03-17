package contract

import (
	"context"
	"strings"
	"testing"
)

func TestScheduleCommandContractAddListStatus(t *testing.T) {
	router := newScheduleContractRouter(t)
	ctx := context.Background()
	conversationID := "schedule-contract-conv"
	windowID := "schedule-contract-win"
	routeKey := "discord:default:direct:user-1:thread:schedule"

	if _, err := router.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("use project: %v", err)
	}

	add, err := router.HandleControlCommand(ctx, `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup`, conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("add schedule: %v", err)
	}
	if !strings.Contains(add.Message, "定时任务已创建") {
		t.Fatalf("unexpected add message: %q", add.Message)
	}

	list, err := router.HandleControlCommand(ctx, "/schedule list", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("list schedule: %v", err)
	}
	if !strings.Contains(list.Message, "image-cleanup") {
		t.Fatalf("unexpected list message: %q", list.Message)
	}

	status, err := router.HandleControlCommand(ctx, "/schedule status image-cleanup", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("status schedule: %v", err)
	}
	if !strings.Contains(status.Message, "定时任务状态") {
		t.Fatalf("unexpected status message: %q", status.Message)
	}
}
