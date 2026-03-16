package project

import (
	"context"
	"strings"

	projectdomain "clawx/internal/domain/project"
)

func (s *Service) AuditProjects(ctx context.Context) (projectdomain.AuditReport, error) {
	registry, changed, err := s.loadAndEnsureRegistry(ctx)
	if err != nil {
		return projectdomain.AuditReport{}, err
	}

	report := projectdomain.AuditReport{
		TotalProjects: len(registry.Projects),
		CheckedAt:     s.clock(),
	}
	for _, item := range registry.Projects {
		switch item.Status {
		case projectdomain.StatusBroken:
			report.BrokenProjects++
		default:
			report.ActiveProjects++
		}
	}

	if s.bindingRepo == nil {
		if changed {
			report.ChangedProjectID = changedProjectIDs(registry.Projects, projectdomain.StatusBroken)
		}
		return report, nil
	}

	bindings, err := s.bindingRepo.List(ctx)
	if err != nil {
		return projectdomain.AuditReport{}, err
	}
	report.TotalBindings = len(bindings)
	for _, binding := range bindings {
		projectID := normalizeProjectID(binding.ProjectID)
		record, ok := registry.Projects[projectID]
		if !ok {
			report.BrokenBindings++
			report.BindingIssues = append(report.BindingIssues, projectdomain.BindingIssue{
				RouteKey:  binding.RouteKey,
				ProjectID: projectID,
				Reason:    "project_not_found",
			})
			continue
		}
		if record.Status == projectdomain.StatusBroken {
			report.BrokenBindings++
			report.BindingIssues = append(report.BindingIssues, projectdomain.BindingIssue{
				RouteKey:  binding.RouteKey,
				ProjectID: projectID,
				Reason:    "project_broken",
			})
		}
	}
	if changed {
		report.ChangedProjectID = changedProjectIDs(registry.Projects, projectdomain.StatusBroken)
	}
	return report, nil
}

func changedProjectIDs(projects map[string]projectdomain.Record, status projectdomain.Status) []string {
	if len(projects) == 0 {
		return nil
	}
	ids := make([]string, 0, len(projects))
	for _, item := range projects {
		if item.Status != status {
			continue
		}
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	sortStrings(ids)
	return ids
}

func sortStrings(values []string) {
	if len(values) < 2 {
		return
	}
	for i := 0; i < len(values)-1; i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}
