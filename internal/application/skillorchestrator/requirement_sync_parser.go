package skillorchestrator

import (
	"encoding/json"
	"regexp"
	"strings"
)

type RequirementSyncPlan struct {
	Type        string `json:"type"`
	Intent      string `json:"intent"`
	Mode        string `json:"mode"`
	AgentID     string `json:"agent_id,omitempty"`
	Requirement string `json:"requirement"`
	Reason      string `json:"reason,omitempty"`
}

var requirementSyncJSONFencePattern = regexp.MustCompile("(?is)```(?:json)?\\s*(\\{.*?\\})\\s*```")

func ParseRequirementSyncPlanFromText(text string) (RequirementSyncPlan, bool, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return RequirementSyncPlan{}, false, nil
	}

	candidate, found := extractRequirementSyncJSONCandidate(trimmed)
	if !found {
		return RequirementSyncPlan{}, false, nil
	}

	var plan RequirementSyncPlan
	if err := json.Unmarshal([]byte(candidate), &plan); err != nil {
		return RequirementSyncPlan{}, true, err
	}
	plan = normalizeRequirementSyncPlan(plan)
	if !isValidRequirementSyncPlan(plan) {
		return RequirementSyncPlan{}, false, nil
	}
	return plan, true, nil
}

func extractRequirementSyncJSONCandidate(text string) (string, bool) {
	if strings.HasPrefix(text, "{") && strings.HasSuffix(text, "}") {
		return text, true
	}
	matches := requirementSyncJSONFencePattern.FindStringSubmatch(text)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1]), true
	}
	return "", false
}

func normalizeRequirementSyncPlan(plan RequirementSyncPlan) RequirementSyncPlan {
	plan.Type = strings.ToLower(strings.TrimSpace(plan.Type))
	plan.Intent = strings.ToLower(strings.TrimSpace(plan.Intent))
	plan.Mode = strings.ToLower(strings.TrimSpace(plan.Mode))
	plan.AgentID = strings.TrimSpace(plan.AgentID)
	plan.Requirement = strings.TrimSpace(plan.Requirement)
	plan.Reason = strings.TrimSpace(plan.Reason)
	return plan
}

func isValidRequirementSyncPlan(plan RequirementSyncPlan) bool {
	if plan.Type != "requirement_sync" {
		return false
	}
	if plan.Intent != "requirement.update" {
		return false
	}
	if plan.Mode != "execute" && plan.Mode != "suggest" {
		return false
	}
	return plan.Requirement != ""
}
