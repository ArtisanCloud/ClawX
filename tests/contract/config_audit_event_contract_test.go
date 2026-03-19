package contract

import "testing"

func TestConfigAuditEventContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandShowContainsSummaryAndRecentTrails",
		"TestHandleConfigChatCommandNonAdminCanManagePlanButCannotApply",
	)
}
