package contract

import "testing"

func TestConfigPlanLifecycleContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandApplyIsOnlyWritePath",
		"TestHandleConfigChatCommandNonAdminCanManagePlanButCannotApply",
		"TestHandleConfigChatCommandSummaryApplyConsistency",
	)
}
