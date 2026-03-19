package contract

import "testing"

func TestConfigApplyOnlyWriteContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandApplyIsOnlyWritePath",
	)
}
