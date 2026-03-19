package integration

import "testing"

func TestConfigNoApplyNoWriteFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleConfigChatCommandApplyIsOnlyWritePath",
	)
}
