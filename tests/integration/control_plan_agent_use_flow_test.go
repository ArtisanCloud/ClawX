package integration

import "testing"

func TestControlPlanAgentUseFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestMaybeAutoApplyAgentSwitchFromModelOutput",
	)
}

