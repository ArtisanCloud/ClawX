package main

import (
	"fmt"
	"os"
	"strings"

	managedservice "clawx/internal/application/managedservice"
	"clawx/internal/application/service"
	"clawx/internal/infrastructure/config"
)

func newServiceCommandService(cfg config.Snapshot) (service.ServiceCommandService, error) {
	workspaceRoot := strings.TrimSpace(cfg.Projects.WorkspaceRoot)
	if workspaceRoot == "" {
		workspaceRoot = config.WorkspaceRoot()
	}
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create service workspace root %q: %w", workspaceRoot, err)
	}
	return managedservice.NewService(workspaceRoot)
}
