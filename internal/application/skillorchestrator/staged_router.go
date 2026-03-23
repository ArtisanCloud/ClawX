package skillorchestrator

import (
	"strings"
)

type StagedRoutingInput struct {
	Message          string
	CurrentAgentID   string
	ProjectID        string
	SkillCatalogSize int
	TopSkillHints    []string
}

type StagedRoutingResult struct {
	Intent           IntentPlan
	Route            RoutePlan
	SkipRoutePlanner bool
	Fallback         StagedRoutingFallback
	CanExecute       bool
}

type StagedRoutingFallback struct {
	Enabled bool
	Mode    string
	Reason  string
}

type StagedRouter struct{}

func NewStagedRouter() *StagedRouter {
	return &StagedRouter{}
}

func (r *StagedRouter) Plan(input StagedRoutingInput) StagedRoutingResult {
	message := strings.TrimSpace(input.Message)
	lower := strings.ToLower(message)
	route := inferRoute(lower, message)
	risk := inferRisk(lower, message)
	complexity := inferComplexity(lower, message)
	intent := IntentPlan{
		Type:       "intent_plan",
		Intent:     "task.execute",
		Route:      route,
		Risk:       risk,
		Complexity: complexity,
	}
	switch route {
	case "control":
		intent.Intent = "control.apply"
	case "requirement":
		intent.Intent = "requirement.update"
	case "skill":
		intent.Intent = "skill.route"
	}

	routePlan := RoutePlan{
		Type:              "route_plan",
		Route:             route,
		UseRoutePlanner:   complexity != "low" || route == "skill",
		ContextBudgetTier: budgetTier(complexity),
		ToolHints:         buildToolHints(route, input.TopSkillHints),
		Reason:            "staged_router_heuristic",
	}
	result := StagedRoutingResult{
		Intent:           intent.Normalize(),
		Route:            routePlan.Normalize(),
		SkipRoutePlanner: !routePlan.UseRoutePlanner,
		CanExecute:       true,
	}
	result.applyFallbackIfNeeded(message)
	return result
}

func (r *StagedRoutingResult) applyFallbackIfNeeded(message string) {
	mode := ""
	reason := ""
	if strings.TrimSpace(message) == "" {
		mode = "clarify"
		reason = "empty_message"
	}
	if err := r.Intent.Validate(); err != nil && reason == "" {
		mode = "suggest"
		reason = "intent_plan_invalid"
	}
	if err := r.Route.Validate(); err != nil && reason == "" {
		mode = "suggest"
		reason = "route_plan_invalid"
	}
	if strings.TrimSpace(r.Intent.Route) != strings.TrimSpace(r.Route.Route) && reason == "" {
		mode = "suggest"
		reason = "route_mismatch"
	}
	if reason == "" {
		r.Fallback = StagedRoutingFallback{
			Enabled: false,
			Mode:    "",
			Reason:  "",
		}
		r.CanExecute = true
		return
	}
	r.Fallback = StagedRoutingFallback{
		Enabled: true,
		Mode:    mode,
		Reason:  reason,
	}
	r.CanExecute = false
}

func inferRoute(lower string, original string) string {
	if isControlIntent(lower, original) {
		return "control"
	}
	if isRequirementIntent(lower, original) {
		return "requirement"
	}
	if isSkillIntent(lower, original) {
		return "skill"
	}
	return "execute"
}

func isControlIntent(lower string, original string) bool {
	if !strings.Contains(lower, "agent") && !strings.Contains(original, "智能体") {
		return false
	}
	return strings.Contains(lower, "switch") ||
		strings.Contains(lower, "use") ||
		strings.Contains(original, "切换") ||
		strings.Contains(original, "换到") ||
		strings.Contains(original, "改成")
}

func isRequirementIntent(lower string, original string) bool {
	keywords := []string{"需求", "项目需求", "验收标准", "roadmap", "milestone", "requirement", "spec"}
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) || strings.Contains(original, keyword) {
			return true
		}
	}
	return false
}

func isSkillIntent(lower string, original string) bool {
	if strings.Contains(lower, "/skill ") {
		return true
	}
	return strings.Contains(lower, "skill") || strings.Contains(original, "技能")
}

func inferRisk(lower string, original string) string {
	if strings.Contains(lower, "drop") || strings.Contains(lower, "delete") || strings.Contains(original, "删除") {
		return "high"
	}
	if strings.Contains(lower, "apply") || strings.Contains(original, "修改") || strings.Contains(original, "变更") {
		return "medium"
	}
	return "low"
}

func inferComplexity(lower string, original string) string {
	length := len([]rune(strings.TrimSpace(original)))
	if length > 220 {
		return "high"
	}
	if strings.Contains(original, "并且") || strings.Contains(original, "同时") || strings.Contains(original, "然后") || strings.Contains(lower, " and ") {
		return "medium"
	}
	return "low"
}

func budgetTier(complexity string) string {
	switch strings.ToLower(strings.TrimSpace(complexity)) {
	case "high":
		return "large"
	case "medium":
		return "medium"
	default:
		return "small"
	}
}

func buildToolHints(route string, hints []string) []string {
	base := []string{"tool.execute_runtime"}
	switch route {
	case "control":
		base = append(base, "tool.agent_inventory")
	case "requirement":
		base = append(base, "tool.agent_inventory", "tool.requirement_sync")
	case "skill":
		base = append(base, "tool.skill_catalog")
	default:
		base = append(base, "tool.skill_catalog")
	}
	for _, hint := range hints {
		value := strings.TrimSpace(hint)
		if value != "" {
			base = append(base, value)
		}
	}
	return base
}
