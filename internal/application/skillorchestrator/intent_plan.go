package skillorchestrator

import (
	"fmt"
	"strings"
)

type IntentPlan struct {
	Type          string `json:"type"`
	Intent        string `json:"intent"`
	TargetAgentID string `json:"target_agent_id,omitempty"`
	Route         string `json:"route"`
	Risk          string `json:"risk"`
	Complexity    string `json:"complexity"`
	Reason        string `json:"reason,omitempty"`
}

func (p IntentPlan) Normalize() IntentPlan {
	normalized := p
	normalized.Type = strings.ToLower(strings.TrimSpace(normalized.Type))
	normalized.Intent = strings.TrimSpace(normalized.Intent)
	normalized.TargetAgentID = strings.TrimSpace(normalized.TargetAgentID)
	normalized.Route = strings.ToLower(strings.TrimSpace(normalized.Route))
	normalized.Risk = strings.ToLower(strings.TrimSpace(normalized.Risk))
	normalized.Complexity = strings.ToLower(strings.TrimSpace(normalized.Complexity))
	normalized.Reason = strings.TrimSpace(normalized.Reason)
	return normalized
}

func (p IntentPlan) Validate() error {
	normalized := p.Normalize()
	if normalized.Type != "intent_plan" {
		return fmt.Errorf("intent plan type must be intent_plan")
	}
	if strings.TrimSpace(normalized.Intent) == "" {
		return fmt.Errorf("intent plan intent is required")
	}
	switch normalized.Route {
	case "control", "requirement", "skill", "execute":
	default:
		return fmt.Errorf("intent plan route is invalid")
	}
	switch normalized.Risk {
	case "low", "medium", "high":
	default:
		return fmt.Errorf("intent plan risk is invalid")
	}
	switch normalized.Complexity {
	case "low", "medium", "high":
	default:
		return fmt.Errorf("intent plan complexity is invalid")
	}
	return nil
}
