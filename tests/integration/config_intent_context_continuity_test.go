package integration

import "testing"

func TestConfigIntentContextContinuityIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleConfigChatCommandMultiRoundPatchContextContinuity",
	)
}
