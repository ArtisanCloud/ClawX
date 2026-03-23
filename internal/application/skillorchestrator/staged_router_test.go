package skillorchestrator

import "testing"

func TestStagedRouterPlanRequirement(t *testing.T) {
	router := NewStagedRouter()
	result := router.Plan(StagedRoutingInput{
		Message:        "请更新项目需求和里程碑",
		CurrentAgentID: "main",
		ProjectID:      "main",
	})
	if result.Intent.Route != "requirement" {
		t.Fatalf("expected requirement route, got %q", result.Intent.Route)
	}
	if result.Intent.Intent != "requirement.update" {
		t.Fatalf("unexpected intent: %q", result.Intent.Intent)
	}
}

func TestStagedRouterPlanControl(t *testing.T) {
	router := NewStagedRouter()
	result := router.Plan(StagedRoutingInput{
		Message:        "切换到 bid-all 智能体",
		CurrentAgentID: "main",
		ProjectID:      "main",
	})
	if result.Intent.Route != "control" {
		t.Fatalf("expected control route, got %q", result.Intent.Route)
	}
	if result.SkipRoutePlanner {
		// control should usually short-circuit with low complexity
		return
	}
}

func TestStagedRouterPlanShortCircuitLowComplexity(t *testing.T) {
	router := NewStagedRouter()
	result := router.Plan(StagedRoutingInput{
		Message:        "现在有多少个智能体？",
		CurrentAgentID: "main",
		ProjectID:      "main",
	})
	if result.SkipRoutePlanner != true {
		t.Fatalf("expected low complexity request to short-circuit route planner")
	}
	if result.Fallback.Enabled {
		t.Fatalf("expected no fallback for normal request")
	}
	if !result.CanExecute {
		t.Fatalf("expected can execute for normal request")
	}
}

func TestStagedRouterPlanRouteHitSkill(t *testing.T) {
	router := NewStagedRouter()
	result := router.Plan(StagedRoutingInput{
		Message:        "请帮我安装技能 bid.collect 到 bid-all",
		CurrentAgentID: "main",
		ProjectID:      "main",
	})
	if result.Intent.Route != "skill" {
		t.Fatalf("expected skill route, got %q", result.Intent.Route)
	}
	if result.Route.UseRoutePlanner != true {
		t.Fatalf("expected skill route to use route planner")
	}
	if result.Fallback.Enabled {
		t.Fatalf("expected no fallback for valid skill request")
	}
}

func TestStagedRouterPlanFallbackNoExecute(t *testing.T) {
	router := NewStagedRouter()
	result := router.Plan(StagedRoutingInput{
		Message:        "   ",
		CurrentAgentID: "main",
		ProjectID:      "main",
	})
	if !result.Fallback.Enabled {
		t.Fatalf("expected fallback for empty message")
	}
	if result.Fallback.Mode != "clarify" {
		t.Fatalf("expected clarify fallback, got %q", result.Fallback.Mode)
	}
	if result.Fallback.Reason != "empty_message" {
		t.Fatalf("unexpected fallback reason: %q", result.Fallback.Reason)
	}
	if result.CanExecute {
		t.Fatalf("fallback path must not execute")
	}
}
