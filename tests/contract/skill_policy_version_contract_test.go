package contract

import "testing"

func TestSkillPolicyVersionContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleSkillChatCommandPolicyVersionReject",
	)
}
