package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	memoryapp "clawx/internal/application/memory"
	projectapp "clawx/internal/application/project"
	"clawx/internal/application/service"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
)

func newProjectCommandService(cfg config.Snapshot) (service.ProjectCommandService, error) {
	stateRoot := filepath.Join(config.StateDir(), "projects")
	if err := os.MkdirAll(stateRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create project state directory %q: %w", stateRoot, err)
	}

	workspaceRoot := strings.TrimSpace(cfg.Projects.WorkspaceRoot)
	if workspaceRoot == "" {
		workspaceRoot = config.WorkspaceRoot()
	}
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create project workspace root %q: %w", workspaceRoot, err)
	}

	defaultProjectID := strings.TrimSpace(cfg.Projects.DefaultProjectID)
	if defaultProjectID == "" {
		defaultProjectID = "main"
	}

	registryStore, err := persistence.NewProjectRegistryFileStore(filepath.Join(stateRoot, "projects.json"))
	if err != nil {
		return nil, err
	}
	bindingStore, err := persistence.NewProjectBindingFileStore(filepath.Join(stateRoot, "bindings.json"))
	if err != nil {
		return nil, err
	}
	proposalStore, err := persistence.NewProjectProposalFileStore(filepath.Join(stateRoot, "proposals.json"))
	if err != nil {
		return nil, err
	}

	return projectapp.NewService(
		registryStore,
		bindingStore,
		proposalStore,
		projectapp.WithWorkspaceRoot(workspaceRoot),
		projectapp.WithDefaultProjectID(defaultProjectID),
		projectapp.WithProposalTTL(10*time.Minute),
		projectapp.WithMemoryTemplateManager(memoryapp.NewTemplateManager()),
	), nil
}
