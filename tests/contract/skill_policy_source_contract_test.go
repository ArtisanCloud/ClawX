package contract

import "testing"

func TestSkillPolicySourceContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleSkillChatCommandPolicySourceReject",
	)
}
