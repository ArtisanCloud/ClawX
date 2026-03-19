package integration

import "testing"

func TestConfigApplySuccessFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleConfigChatCommandPlanAndApply",
		"TestHandleConfigChatCommandApplyCreatesWorkspaceDir",
	)
}
