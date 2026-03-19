package contract

import "testing"

func TestConfigApplyPermissionContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandNonAdminCanManagePlanButCannotApply",
	)
}
