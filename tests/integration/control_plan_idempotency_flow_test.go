package integration

import "testing"

func TestControlPlanIdempotencyFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestMaybeAutoApplyAgentSwitchNoopStatus",
	)
}
