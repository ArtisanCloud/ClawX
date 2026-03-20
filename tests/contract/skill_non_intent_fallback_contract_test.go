package contract

import "testing"

func TestSkillNonIntentFallbackContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleSkillChatCommandNonIntentFallback",
	)
}
