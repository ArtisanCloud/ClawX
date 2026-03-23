package skillorchestrator

import "testing"

func TestRoutePlanValidate(t *testing.T) {
	plan := RoutePlan{
		Type:              "route_plan",
		Route:             "execute",
		UseRoutePlanner:   true,
		ContextBudgetTier: "large",
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("expected valid route plan: %v", err)
	}
}

func TestRoutePlanValidateInvalidTier(t *testing.T) {
	plan := RoutePlan{
		Type:              "route_plan",
		Route:             "execute",
		ContextBudgetTier: "x",
	}
	if err := plan.Validate(); err == nil {
		t.Fatalf("expected invalid tier error")
	}
}
