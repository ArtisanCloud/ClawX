package project

import (
	"context"

	projectdomain "clawx/internal/domain/project"
)

func (s *Service) ListProjects(ctx context.Context) ([]projectdomain.Record, error) {
	registry, _, err := s.loadAndEnsureRegistry(ctx)
	if err != nil {
		return nil, err
	}
	if len(registry.Projects) == 0 {
		return nil, nil
	}
	result := make([]projectdomain.Record, 0, len(registry.Projects))
	for _, item := range registry.Projects {
		result = append(result, item)
	}
	sortProjects(result)
	return result, nil
}

func (s *Service) GetProject(ctx context.Context, projectID string) (projectdomain.Record, error) {
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
	return record, nil
}
