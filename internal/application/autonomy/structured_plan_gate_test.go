package autonomy

import "testing"

func TestEnforceStructuredPlanGate(t *testing.T) {
	validAction := `{"type":"action_plan","mode":"execute","actions":[{"kind":"agent.use","agent_id":"bid-all"}]}`
	if err := EnforceStructuredPlanGate(validAction); err != nil {
		t.Fatalf("valid action_plan should pass: %v", err)
	}

	invalidAction := `{"type":"action_plan","mode":"execute","actions":[{"kind":"agent.delete","agent_id":"bid-all"}]}`
	if err := EnforceStructuredPlanGate(invalidAction); err == nil {
		t.Fatalf("invalid allowlist action_plan should be rejected")
	}

	validBlocker := `{"type":"execution_blocker","need_user_input":true,"attempted":["a"],"evidence":["b"],"evidence_exec_ids":["rexec-1"]}`
	if err := EnforceStructuredPlanGate(validBlocker); err != nil {
		t.Fatalf("valid execution_blocker should pass: %v", err)
	}
	invalidBlocker := `{"type":"execution_blocker","need_user_input":true,"attempted":["a"],"evidence":["b"]}`
	if err := EnforceStructuredPlanGate(invalidBlocker); err == nil {
		t.Fatalf("execution_blocker without evidence_exec_ids should be rejected")
	}

	deprecated := `{"type":"control_plan","intent":"agent.use","target":{"agent_id":"bid-all"},"mode":"execute","risk":"low"}`
	if err := EnforceStructuredPlanGate(deprecated); err == nil {
		t.Fatalf("deprecated control_plan should be rejected")
	}
}
