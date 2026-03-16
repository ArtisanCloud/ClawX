package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	memoryapp "clawx/internal/application/memory"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
)

func newMemoryCommandService(cfg config.Snapshot, projectResolver memoryapp.ProjectResolver) (*memoryapp.CommandService, error) {
	workspaceRoot := strings.TrimSpace(cfg.Projects.WorkspaceRoot)
	if workspaceRoot == "" {
		workspaceRoot = config.WorkspaceRoot()
	}
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create memory workspace root %q: %w", workspaceRoot, err)
	}

	templateStore, err := persistence.NewMemoryTemplateFileStore(workspaceRoot)
	if err != nil {
		return nil, err
	}
	journalStore, err := persistence.NewMemoryJournalFileStore(workspaceRoot)
	if err != nil {
		return nil, err
	}
	auditStore, err := persistence.NewMemoryAuditFileStore(workspaceRoot)
	if err != nil {
		return nil, err
	}
	digestStore, err := persistence.NewMemoryDigestFileStore(workspaceRoot)
	if err != nil {
		return nil, err
	}

	return memoryapp.NewCommandService(
		templateStore,
		journalStore,
		auditStore,
		digestStore,
		memoryapp.WithCommandWorkspaceRoot(filepath.Clean(workspaceRoot)),
		memoryapp.WithCommandProjectResolver(projectResolver),
		memoryapp.WithCommandOwnerAllowlist(cfg.Memory.OwnerAllowlist),
		memoryapp.WithCommandAutoDigestEnabled(cfg.Memory.AutoDigestEnabled),
	), nil
}
