package project

import (
	"context"
	"fmt"
	"os"

	projectdomain "clawx/internal/domain/project"
)

func (s *Service) RepairProject(ctx context.Context, projectID string) (projectdomain.Record, error) {
	projectID = normalizeProjectID(projectID)
	if projectID == "" {
		return projectdomain.Record{}, projectdomain.ErrInvalidProject
	}

	registry, _, err := s.loadAndEnsureRegistry(ctx)
	if err != nil {
		return projectdomain.Record{}, err
	}

	record, ok := registry.Projects[projectID]
	if !ok {
		return projectdomain.Record{}, projectdomain.ErrProjectNotFound
	}
	if record.WorkspacePath == "" {
		return projectdomain.Record{}, projectdomain.ErrInvalidProject
	}
	if err := os.MkdirAll(record.WorkspacePath, 0o755); err != nil {
		return projectdomain.Record{}, fmt.Errorf("repair project workspace: %w", err)
	}
	if err := s.ensureMemoryTemplate(ctx, projectID, record.WorkspacePath); err != nil {
		return projectdomain.Record{}, err
	}
	record.Status = projectdomain.StatusActive
	record.UpdatedAt = s.clock()
	registry.Projects[projectID] = record
	registry.UpdatedAt = s.clock()
	if err := s.registryRepo.Save(ctx, registry); err != nil {
		return projectdomain.Record{}, err
	}
	return record, nil
}
