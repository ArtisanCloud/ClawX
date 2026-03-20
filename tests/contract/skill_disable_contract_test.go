package contract

import "testing"

func TestSkillDisableContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleSkillChatCommandDisableImmediateEffect",
	)
}
