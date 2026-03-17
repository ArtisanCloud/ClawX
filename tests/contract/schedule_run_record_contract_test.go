package contract

import (
	"context"
	"testing"
)

func TestScheduleRunRecordContract(t *testing.T) {
	router := newScheduleContractRouter(t)
	ctx := context.Background()
	conversationID := "schedule-record-conv"
	windowID := "schedule-record-win"
	routeKey := "discord:default:direct:user-7:thread:schedule"

	_, _ = router.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup`, conversationID, windowID, routeKey)
	if _, err := router.HandleControlCommand(ctx, "/schedule run image-cleanup", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("run schedule: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/schedule status image-cleanup", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("status schedule: %v", err)
	}
}
