package integration

import "testing"

func TestConfigNonAdminAllowedFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleConfigChatCommandNonAdminCanManagePlanButCannotApply",
	)
}

