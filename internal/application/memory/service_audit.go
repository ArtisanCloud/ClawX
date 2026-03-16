package memory

import (
	"context"
	"errors"
	"strings"

	memorydomain "clawx/internal/domain/memory"
)

func (s *CommandService) Audit(ctx context.Context, input AuditInput) (AuditResult, error) {
	scoped, err := s.resolveScopeContext(ctx, scopeInput{
		RouteKey:        input.RouteKey,
		ProjectID:       input.ProjectID,
		AgentID:         input.AgentID,
		UserID:          input.UserID,
		IsDirectMessage: input.IsDirectMessage,
	})
	if err != nil {
		return AuditResult{}, err
	}

	result := AuditResult{}
	if s.templateRepo != nil {
		manifest, err := s.templateRepo.GetManifest(ctx, scoped.projectID)
		switch {
		case err == nil:
			result.TemplateVersion = strings.TrimSpace(manifest.TemplateVersion)
			result.RequiredFiles = len(manifest.RequiredFiles)
			for _, relativePath := range manifest.RequiredFiles {
				if _, readErr := s.templateRepo.ReadFile(ctx, scoped.projectID, relativePath); readErr != nil {
					result.MissingRequired++
				}
			}
		case errors.Is(err, memorydomain.ErrMemoryNotFound):
			result.TemplateVersion = "missing"
		default:
			return AuditResult{}, newCommandError("audit_corrupted", err.Error())
		}
	}
	if strings.TrimSpace(result.TemplateVersion) == "" {
		result.TemplateVersion = "unknown"
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	recentErrors := make([]string, 0)
	if s.auditRepo != nil {
		records, err := s.auditRepo.ListByProject(ctx, scoped.projectID, limit)
		if err != nil {
			return AuditResult{}, newCommandError("audit_corrupted", err.Error())
		}
		for _, record := range records {
			for _, denied := range record.DeniedFiles {
				lower := strings.ToLower(strings.TrimSpace(denied))
				if lower == "" {
					continue
				}
				if strings.Contains(lower, "acl") || strings.Contains(lower, "cross_agent") || strings.Contains(lower, "cross_project") || strings.Contains(lower, "rejected_acl") {
					result.ACLDeniedCount++
				}
				if strings.Contains(lower, "budget") || strings.Contains(lower, "token_budget_exceeded") {
					result.BudgetSkippedCount++
				}
			}
			for _, item := range record.ErrorFiles {
				item = strings.TrimSpace(item)
				if item != "" {
					recentErrors = append(recentErrors, item)
				}
			}
		}
	}
	result.RecentErrors = uniqueStrings(recentErrors)
	return result, nil
}
