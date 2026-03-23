package skillorchestrator

import "testing"

func TestIntentPlanValidate(t *testing.T) {
	plan := IntentPlan{
		Type:       "intent_plan",
		Intent:     "requirement.update",
		Route:      "requirement",
		Risk:       "low",
		Complexity: "medium",
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("expected valid intent plan: %v", err)
	}
}

func TestIntentPlanValidateInvalidRoute(t *testing.T) {
	plan := IntentPlan{
		Type:       "intent_plan",
		Intent:     "x",
		Route:      "unknown",
		Risk:       "low",
		Complexity: "low",
	}
	if err := plan.Validate(); err == nil {
		t.Fatalf("expected invalid route error")
	}
}
