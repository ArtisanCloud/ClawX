package contract

import "testing"

func TestConfigPlanSummaryApplyConsistencyContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandSummaryApplyConsistency",
	)
}
