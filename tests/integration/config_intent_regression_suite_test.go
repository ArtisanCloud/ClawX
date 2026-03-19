package integration

import "testing"

func TestConfigIntentRegressionSuite(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleConfigChatCommandNaturalLanguagePlan",
		"TestHandleConfigChatCommandNaturalLanguagePatchWorkspace",
		"TestHandleConfigChatCommandNonAdminCanManagePlanButCannotApply",
		"TestHandleConfigChatCommandShowContainsSummaryAndRecentTrails",
	)
}
