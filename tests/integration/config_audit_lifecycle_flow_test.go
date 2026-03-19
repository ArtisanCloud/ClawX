package integration

import "testing"

func TestConfigAuditLifecycleFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleConfigChatCommandShowContainsSummaryAndRecentTrails",
		"TestHandleConfigChatCommandSummaryApplyConsistency",
		"TestHandleConfigChatCommandNonAdminCanManagePlanButCannotApply",
	)
}
