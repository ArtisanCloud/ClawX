package contract

import (
	"testing"

	"clawx/internal/application/autonomy"
)

func TestAutonomySchemaAllowlistGateContract(t *testing.T) {
	validAction := `{"type":"action_plan","mode":"execute","actions":[{"kind":"agent.use","agent_id":"bid-all"}]}`
	if err := autonomy.EnforceStructuredPlanGate(validAction); err != nil {
		t.Fatalf("valid action_plan should pass gate: %v", err)
	}

	invalidAction := `{"type":"action_plan","mode":"execute","actions":[{"kind":"agent.delete","agent_id":"bid-all"}]}`
	if err := autonomy.EnforceStructuredPlanGate(invalidAction); err == nil {
		t.Fatalf("action_plan outside allowlist must be rejected")
	}

	validBlocker := `{"type":"execution_blocker","need_user_input":true,"attempted":["a"],"evidence":["b"],"evidence_exec_ids":["rexec-1"]}`
	if err := autonomy.EnforceStructuredPlanGate(validBlocker); err != nil {
		t.Fatalf("valid execution_blocker should pass gate: %v", err)
	}
	invalidBlocker := `{"type":"execution_blocker","need_user_input":true,"attempted":["a"],"evidence":["b"]}`
	if err := autonomy.EnforceStructuredPlanGate(invalidBlocker); err == nil {
		t.Fatalf("execution_blocker missing evidence_exec_ids must be rejected")
	}

	deprecated := `{"type":"requirement_sync","intent":"requirement.update","mode":"execute","requirement":"补充需求"}`
	if err := autonomy.EnforceStructuredPlanGate(deprecated); err == nil {
		t.Fatalf("deprecated requirement_sync must be rejected")
	}
}
