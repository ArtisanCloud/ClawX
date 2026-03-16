package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	projectdomain "clawx/internal/domain/project"
)

func (s *Service) CreateProject(ctx context.Context, projectID, name, workspacePath string) (projectdomain.Record, error) {
	if s.registryRepo == nil {
		return projectdomain.Record{}, fmt.Errorf("project registry repository is required")
	}

	projectID = normalizeProjectID(projectID)
	if projectID == "" {
		return projectdomain.Record{}, projectdomain.ErrInvalidProject
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = projectID
	}
	workspacePath = strings.TrimSpace(workspacePath)
	if workspacePath == "" {
		workspacePath = filepath.Join(s.workspaceRoot, projectID)
	}
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		return projectdomain.Record{}, fmt.Errorf("create project workspace: %w", err)
	}
	if err := s.ensureMemoryTemplate(ctx, projectID, workspacePath); err != nil {
		return projectdomain.Record{}, err
	}

	registry, _, err := s.loadAndEnsureRegistry(ctx)
	if err != nil {
		return projectdomain.Record{}, err
	}
	if _, exists := registry.Projects[projectID]; exists {
		return projectdomain.Record{}, fmt.Errorf("project %q already exists", projectID)
	}

	now := s.clock()
	record := projectdomain.Record{
		ID:            projectID,
		Name:          name,
		WorkspacePath: workspacePath,
		Status:        projectdomain.StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	registry.Projects[projectID] = record
	registry.UpdatedAt = now
	if strings.TrimSpace(registry.DefaultProjectID) == "" {
		registry.DefaultProjectID = projectID
	}
	if err := s.registryRepo.Save(ctx, registry); err != nil {
		return projectdomain.Record{}, err
	}
	return record, nil
}
