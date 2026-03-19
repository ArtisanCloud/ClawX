package contract

import "testing"

func TestConfigPlanNLCreateContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandNaturalLanguagePlan",
	)
}
