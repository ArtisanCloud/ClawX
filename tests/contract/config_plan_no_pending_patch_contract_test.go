package contract

import "testing"

func TestConfigPlanNoPendingPatchContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandNaturalLanguagePatchWithoutPendingPlanFallsBack",
	)
}
