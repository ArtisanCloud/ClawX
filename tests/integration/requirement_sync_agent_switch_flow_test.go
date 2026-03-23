package integration

import "testing"

func TestRequirementSyncAgentSwitchFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestMaybeAutoApplyRequirementSyncFromModel_WithTargetAgentSwitch",
	)
}
