package contract

import "testing"

func TestConfigNonAdminAllowedFlowContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandNonAdminCanManagePlanButCannotApply",
	)
}

