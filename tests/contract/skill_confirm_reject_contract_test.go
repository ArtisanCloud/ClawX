package contract

import "testing"

func TestSkillConfirmRejectContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleSkillChatCommandConfirmRejectNoSideEffect",
	)
}
