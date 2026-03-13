package project

import (
	"context"
	"fmt"
	"strings"

	projectdomain "clawx/internal/domain/project"
)

func (s *Service) UseProject(ctx context.Context, routeKey, projectID, updatedBy string) (projectdomain.RouteBinding, error) {
	if s.bindingRepo == nil {
		return projectdomain.RouteBinding{}, fmt.Errorf("project binding repository is required")
	}
	routeKey = normalizeRouteKey(routeKey)
	projectID = normalizeProjectID(projectID)
	if routeKey == "" || projectID == "" {
		return projectdomain.RouteBinding{}, projectdomain.ErrInvalidProject
	}

	registry, _, err := s.loadAndEnsureRegistry(ctx)
	if err != nil {
		return projectdomain.RouteBinding{}, err
	}
	record, ok := registry.Projects[projectID]
	if !ok {
		return projectdomain.RouteBinding{}, projectdomain.ErrProjectNotFound
	}
	if record.Status == projectdomain.StatusBroken {
		return projectdomain.RouteBinding{}, projectdomain.ErrInvalidProject
	}

	binding := projectdomain.RouteBinding{
		RouteKey:  routeKey,
		ProjectID: projectID,
		UpdatedAt: s.clock(),
		UpdatedBy: strings.TrimSpace(updatedBy),
		Source:    projectdomain.SourceManual,
	}
	if binding.UpdatedBy == "" {
		binding.UpdatedBy = "system"
	}
	if err := s.bindingRepo.Upsert(ctx, binding); err != nil {
		return projectdomain.RouteBinding{}, err
	}
	return binding, nil
}
