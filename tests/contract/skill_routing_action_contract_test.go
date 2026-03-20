package contract

import "testing"

func TestSkillRoutingActionContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleSkillChatCommandNaturalLanguageRoute",
	)
}
