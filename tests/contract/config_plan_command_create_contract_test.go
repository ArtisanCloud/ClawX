package contract

import "testing"

func TestConfigPlanCommandCreateContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestParseConfigChatCommand",
		"TestHandleConfigChatCommandPlanAndApply",
	)
}
