package contract

import "testing"

func TestConfigPlanShowContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandShowContainsSummaryAndRecentTrails",
	)
}
