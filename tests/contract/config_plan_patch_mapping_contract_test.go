package contract

import "testing"

func TestConfigPlanPatchMappingContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandNaturalLanguagePatchWorkspace",
		"TestHandleConfigChatCommandNaturalLanguagePatchTimeout",
		"TestHandleConfigChatCommandMultiRoundPatchContextContinuity",
	)
}
