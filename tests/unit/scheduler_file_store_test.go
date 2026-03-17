package unit

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	schedulerdomain "clawx/internal/domain/scheduler"
	"clawx/internal/infrastructure/persistence"
)

func TestSchedulerFileStoreReadWrite(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspaces")
	store, err := persistence.NewSchedulerFileStore(root)
	if err != nil {
		t.Fatalf("new scheduler file store: %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	job := schedulerdomain.Job{
		JobID:        "job-1",
		Name:         "image-cleanup",
		Scope:        schedulerdomain.Scope{ProjectID: "image_tools", AgentID: "main"},
		ScheduleExpr: "0 3 * * 0",
		TaskType:     "image.cleanup",
		TaskArgs:     map[string]string{"retention_days": "30"},
		Status:       schedulerdomain.JobStatusActive,
		NextRunAt:    now.Add(time.Hour),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, err := store.Save(ctx, job); err != nil {
		t.Fatalf("save job: %v", err)
	}
	list, err := store.List(ctx, schedulerdomain.Scope{ProjectID: "image_tools", AgentID: "main"})
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(list) != 1 || list[0].Name != "image-cleanup" {
		t.Fatalf("unexpected list: %+v", list)
	}
	run := schedulerdomain.RunRecord{RunID: "run-1", JobID: "job-1", StartedAt: now, EndedAt: now.Add(time.Second), Result: schedulerdomain.RunResultSuccess, TriggerMode: schedulerdomain.TriggerManual}
	if err := store.Append(ctx, job.Scope, run); err != nil {
		t.Fatalf("append run: %v", err)
	}
	recent, err := store.RecentByJob(ctx, job.Scope, job.JobID, 10)
	if err != nil {
		t.Fatalf("recent runs: %v", err)
	}
	if len(recent) != 1 || recent[0].RunID != "run-1" {
		t.Fatalf("unexpected runs: %+v", recent)
	}
}
