package contract

import "testing"

func TestConfigAuditDiffContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandNaturalLanguagePatchWorkspace",
		"TestHandleConfigChatCommandSummaryApplyConsistency",
	)
}
