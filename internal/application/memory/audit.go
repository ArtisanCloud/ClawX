package memory

import (
	"fmt"
	"slices"
	"strings"

	memorydomain "clawx/internal/domain/memory"
)

type AuditFields struct {
	MemoryScope             string
	MemoryACLMode           string
	LoadedFiles             []string
	DeniedFiles             []string
	ErrorSummary            string
	CrossAgentDeniedCount   int
	CrossProjectDeniedCount int
}

func BuildAuditFields(scope memorydomain.MemoryScopeKey, profile memorydomain.MemoryProfile, output LoaderOutput, denied []DeniedCandidate) AuditFields {
	fields := AuditFields{
		MemoryScope:   fmt.Sprintf("agent=%s project=%s route=%s chat_mode=%s", scope.AgentID, scope.ProjectID, scope.RouteKey, scope.ChatMode),
		MemoryACLMode: string(profile.ACLMode),
		LoadedFiles:   uniqueStrings(output.LoadedFiles),
		ErrorSummary:  strings.TrimSpace(output.ErrorSummary),
	}
	if fields.MemoryACLMode == "" {
		fields.MemoryACLMode = string(memorydomain.ACLModeStrict)
	}

	deniedValues := make([]string, 0, len(output.DeniedFiles)+len(denied))
	for _, path := range output.DeniedFiles {
		path = strings.TrimSpace(path)
		if path != "" {
			deniedValues = append(deniedValues, path)
		}
	}
	for _, item := range denied {
		if strings.TrimSpace(item.Path) == "" {
			continue
		}
		reason := strings.TrimSpace(item.Reason)
		if reason == "" {
			reason = "denied"
		}
		if reason == "cross_agent_denied" {
			fields.CrossAgentDeniedCount++
		}
		if reason == "cross_project_denied" {
			fields.CrossProjectDeniedCount++
		}
		deniedValues = append(deniedValues, fmt.Sprintf("%s|%s", item.Path, reason))
	}
	fields.DeniedFiles = uniqueStrings(deniedValues)
	return fields
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil
	}
	slices.Sort(result)
	return result
}
