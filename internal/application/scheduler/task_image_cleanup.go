package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (s *Service) executeImageCleanup(job Job) (string, error) {
	retentionDays := 30
	if raw, ok := job.TaskArgs["retention_days"]; ok && strings.TrimSpace(raw) != "" {
		value, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || value <= 0 {
			return "", newCommandError("schedule_run_failed", "retention_days invalid", mapNextAction("schedule_run_failed"))
		}
		retentionDays = value
	}
	collectionsRoot := filepath.Join(s.workspaceRoot, job.Scope.ProjectID, ".image", "collections")
	entries, err := os.ReadDir(collectionsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "scan=0 deleted=0 kept=0 failed=0", nil
		}
		return "", err
	}
	cutoff := s.now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	scanned := 0
	deleted := 0
	kept := 0
	failed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		scanned++
		path := filepath.Join(collectionsRoot, entry.Name())
		info, err := entry.Info()
		if err != nil {
			failed++
			continue
		}
		if info.ModTime().UTC().Before(cutoff) {
			if err := os.RemoveAll(path); err != nil {
				failed++
				continue
			}
			deleted++
			continue
		}
		kept++
	}
	return fmt.Sprintf("scan=%d deleted=%d kept=%d failed=%d retention_days=%d", scanned, deleted, kept, failed, retentionDays), nil
}
