package contract

import (
	"context"
	"strings"
	"testing"
)

func TestScheduleRunContract(t *testing.T) {
	router := newScheduleContractRouter(t)
	ctx := context.Background()
	conversationID := "schedule-run-conv"
	windowID := "schedule-run-win"
	routeKey := "discord:default:direct:user-3:thread:schedule"

	_, _ = router.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup --arg retention_days=30`, conversationID, windowID, routeKey)

	run, err := router.HandleControlCommand(ctx, "/schedule run image-cleanup", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("run schedule: %v", err)
	}
	if !strings.Contains(run.Message, "执行完成") {
		t.Fatalf("unexpected run message: %q", run.Message)
	}
}
