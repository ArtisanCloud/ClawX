package contract

import (
	"context"
	"strings"
	"testing"
)

func TestScheduleStateContractPauseResumeRemove(t *testing.T) {
	router := newScheduleContractRouter(t)
	ctx := context.Background()
	conversationID := "schedule-state-conv"
	windowID := "schedule-state-win"
	routeKey := "discord:default:direct:user-2:thread:schedule"

	_, _ = router.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup`, conversationID, windowID, routeKey)

	pause, err := router.HandleControlCommand(ctx, "/schedule pause image-cleanup", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("pause schedule: %v", err)
	}
	if !strings.Contains(pause.Message, "已暂停") {
		t.Fatalf("unexpected pause message: %q", pause.Message)
	}

	resume, err := router.HandleControlCommand(ctx, "/schedule resume image-cleanup", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("resume schedule: %v", err)
	}
	if !strings.Contains(resume.Message, "已恢复") {
		t.Fatalf("unexpected resume message: %q", resume.Message)
	}

	remove, err := router.HandleControlCommand(ctx, "/schedule remove image-cleanup", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("remove schedule: %v", err)
	}
	if !strings.Contains(remove.Message, "已删除") {
		t.Fatalf("unexpected remove message: %q", remove.Message)
	}
}
