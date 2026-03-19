package integration

import "testing"

func TestConfigPlanPatchOverwriteFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleConfigChatCommandMultiRoundPatchContextContinuity",
	)
}
