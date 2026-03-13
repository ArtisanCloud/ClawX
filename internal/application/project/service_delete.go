package project

import (
	"context"
	"strings"

	projectdomain "clawx/internal/domain/project"
)

func (s *Service) DeleteProject(ctx context.Context, projectID string, force bool) (projectdomain.Record, error) {
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
	if projectID == normalizeProjectID(registry.DefaultProjectID) {
		return projectdomain.Record{}, projectdomain.ErrInvalidProject
	}

	if !force {
		inUse, err := s.projectHasBindings(ctx, projectID)
		if err != nil {
			return projectdomain.Record{}, err
		}
		if inUse {
			return projectdomain.Record{}, projectdomain.ErrProjectInUse
		}
		if s.sessionChecker != nil {
			busy, err := s.sessionChecker(ctx, projectID)
			if err != nil {
				return projectdomain.Record{}, err
			}
			if busy {
				return projectdomain.Record{}, projectdomain.ErrProjectBusy
			}
		}
	}

	delete(registry.Projects, projectID)
	registry.UpdatedAt = s.clock()
	if err := s.registryRepo.Save(ctx, registry); err != nil {
		return projectdomain.Record{}, err
	}

	if force && s.bindingRepo != nil {
		bindings, err := s.bindingRepo.List(ctx)
		if err != nil {
			return projectdomain.Record{}, err
		}
		for _, binding := range bindings {
			if normalizeProjectID(binding.ProjectID) != projectID {
				continue
			}
			if err := s.bindingRepo.DeleteByRouteKey(ctx, strings.TrimSpace(binding.RouteKey)); err != nil {
				return projectdomain.Record{}, err
			}
		}
	}
	return record, nil
}

func (s *Service) projectHasBindings(ctx context.Context, projectID string) (bool, error) {
	if s.bindingRepo == nil {
		return false, nil
	}
	bindings, err := s.bindingRepo.List(ctx)
	if err != nil {
		return false, err
	}
	for _, binding := range bindings {
		if normalizeProjectID(binding.ProjectID) == projectID {
			return true, nil
		}
	}
	return false, nil
}
