package contract

import "testing"

func TestConfigPlanShowSummaryContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandShowContainsSummaryAndRecentTrails",
	)
}
