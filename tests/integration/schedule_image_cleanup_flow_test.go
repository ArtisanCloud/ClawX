package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScheduleImageCleanupFlow(t *testing.T) {
	router, workspaceRoot := newScheduleControlRouter(t)
	ctx := context.Background()
	conversationID := "schedule-cleanup-conv"
	windowID := "schedule-cleanup-window"
	routeKey := "discord:default:direct:user-c:thread:schedule"

	_, _ = router.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey)
	_, _ = router.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey)

	oldDir := filepath.Join(workspaceRoot, "image_tools", ".image", "collections", "old")
	newDir := filepath.Join(workspaceRoot, "image_tools", ".image", "collections", "new")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatalf("mkdir old: %v", err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatalf("mkdir new: %v", err)
	}
	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	if err := os.Chtimes(oldDir, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes old: %v", err)
	}

	_, _ = router.HandleControlCommand(ctx, `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup --arg retention_days=30`, conversationID, windowID, routeKey)
	run, err := router.HandleControlCommand(ctx, "/schedule run image-cleanup", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("run schedule: %v", err)
	}
	if !strings.Contains(run.Message, "deleted=") {
		t.Fatalf("unexpected run summary: %q", run.Message)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("old dir should be deleted, err=%v", err)
	}
	if _, err := os.Stat(newDir); err != nil {
		t.Fatalf("new dir should remain: %v", err)
	}
}
