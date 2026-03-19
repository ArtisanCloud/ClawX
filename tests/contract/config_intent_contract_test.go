package contract

import "testing"

func TestConfigIntentContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandNaturalLanguagePlan",
		"TestHandleConfigChatCommandMixedIntentRequiresClarification",
		"TestHandleConfigChatCommandLowConfidenceSuggestOnly",
	)
}
