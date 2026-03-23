package contract

import (
	"testing"

	"clawx/internal/application/skillorchestrator"
)

func TestControlPlanSchemaContract(t *testing.T) {
	text := `{
  "type": "control_plan",
  "intent": "agent.use",
  "target": {"agent_id": "bid-all"},
  "mode": "execute",
  "risk": "low",
  "reason": "用户请求切换智能体"
}`
	plan, ok, err := skillorchestrator.ParseControlPlanFromText(text)
	if err != nil {
		t.Fatalf("parse control plan: %v", err)
	}
	if !ok {
		t.Fatalf("expected control plan to be detected")
	}
	if plan.Intent != "agent.use" {
		t.Fatalf("unexpected intent: %q", plan.Intent)
	}
	if plan.AgentID() != "bid-all" {
		t.Fatalf("unexpected agent id: %q", plan.AgentID())
	}
}
