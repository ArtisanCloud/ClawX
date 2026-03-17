package contract

import (
	"context"
	"strings"
	"testing"
)

func TestScheduleDefaultPolicyContract(t *testing.T) {
	router := newScheduleContractRouter(t)
	ctx := context.Background()
	conversationID := "schedule-default-conv"
	windowID := "schedule-default-win"
	routeKey := "discord:default:direct:user-4:thread:schedule"

	_, _ = router.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey)

	_, err := router.HandleControlCommand(ctx, `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup`, conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("add schedule: %v", err)
	}
	status, err := router.HandleControlCommand(ctx, "/schedule status image-cleanup", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("status schedule: %v", err)
	}
	if !strings.Contains(status.Message, "0 3 * * 0") {
		t.Fatalf("unexpected status message: %q", status.Message)
	}
}
