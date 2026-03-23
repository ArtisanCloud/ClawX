package skillorchestrator

import (
	"fmt"
	"strings"
)

type RoutePlan struct {
	Type              string   `json:"type"`
	Route             string   `json:"route"`
	UseRoutePlanner   bool     `json:"use_route_planner"`
	ContextBudgetTier string   `json:"context_budget_tier"`
	ToolHints         []string `json:"tool_hints,omitempty"`
	Reason            string   `json:"reason,omitempty"`
}

func (p RoutePlan) Normalize() RoutePlan {
	normalized := p
	normalized.Type = strings.ToLower(strings.TrimSpace(normalized.Type))
	normalized.Route = strings.ToLower(strings.TrimSpace(normalized.Route))
	normalized.ContextBudgetTier = strings.ToLower(strings.TrimSpace(normalized.ContextBudgetTier))
	normalized.Reason = strings.TrimSpace(normalized.Reason)
	hints := make([]string, 0, len(normalized.ToolHints))
	for _, hint := range normalized.ToolHints {
		value := strings.TrimSpace(hint)
		if value != "" {
			hints = append(hints, value)
		}
	}
	normalized.ToolHints = hints
	return normalized
}

func (p RoutePlan) Validate() error {
	normalized := p.Normalize()
	if normalized.Type != "route_plan" {
		return fmt.Errorf("route plan type must be route_plan")
	}
	switch normalized.Route {
	case "control", "requirement", "skill", "execute":
	default:
		return fmt.Errorf("route plan route is invalid")
	}
	switch normalized.ContextBudgetTier {
	case "small", "medium", "large":
	default:
		return fmt.Errorf("route plan context budget tier is invalid")
	}
	return nil
}
