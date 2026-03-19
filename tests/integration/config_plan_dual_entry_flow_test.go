package integration

import "testing"

func TestConfigPlanDualEntryFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleConfigChatCommandNaturalLanguagePlan",
		"TestHandleConfigChatCommandPlanAndApply",
	)
}
