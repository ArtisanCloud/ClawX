package project

import (
	"context"
)

func (s *Service) CurrentProject(ctx context.Context, routeKey string) (Resolution, error) {
	projectID, mode, err := s.ResolveProject(ctx, routeKey)
	if err != nil {
		return Resolution{}, err
	}

	record, err := s.GetProject(ctx, projectID)
	if err != nil {
		return Resolution{}, err
	}
	return Resolution{
		Project:     record,
		RouteKey:    normalizeRouteKey(routeKey),
		RoutingMode: mode,
	}, nil
}
