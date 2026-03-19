package contract

import "testing"

func TestConfigIntentLowConfidenceSuggestOnlyContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandLowConfidenceSuggestOnly",
	)
}

