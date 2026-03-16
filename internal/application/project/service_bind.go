package project

import (
	"context"

	projectdomain "clawx/internal/domain/project"
)

func (s *Service) BindRoute(ctx context.Context, routeKey, projectID, updatedBy string) (projectdomain.RouteBinding, error) {
	return s.UseProject(ctx, routeKey, projectID, updatedBy)
}

func (s *Service) UnbindRoute(ctx context.Context, routeKey string) error {
	if s.bindingRepo == nil {
		return nil
	}
	routeKey = normalizeRouteKey(routeKey)
	if routeKey == "" {
		return projectdomain.ErrInvalidProject
	}
	return s.bindingRepo.DeleteByRouteKey(ctx, routeKey)
}
