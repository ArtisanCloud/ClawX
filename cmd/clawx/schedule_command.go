package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	schedulerapp "clawx/internal/application/scheduler"
	"clawx/internal/application/service"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
)

func newScheduleCommandService(cfg config.Snapshot) (service.ScheduleCommandService, *schedulerapp.LoopRunner, error) {
	workspaceRoot := strings.TrimSpace(cfg.Projects.WorkspaceRoot)
	if workspaceRoot == "" {
		workspaceRoot = config.WorkspaceRoot()
	}
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create scheduler workspace root %q: %w", workspaceRoot, err)
	}
	store, err := persistence.NewSchedulerFileStore(workspaceRoot)
	if err != nil {
		return nil, nil, err
	}
	svc, err := schedulerapp.NewService(workspaceRoot, store, store)
	if err != nil {
		return nil, nil, err
	}
	runner := schedulerapp.NewLoopRunner(svc, 30*time.Second)
	return svc, runner, nil
}
