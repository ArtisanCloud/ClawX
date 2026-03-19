package integration

import "testing"

func TestConfigPlanPatchHistoryFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleConfigChatCommandShowContainsSummaryAndRecentTrails",
	)
}
