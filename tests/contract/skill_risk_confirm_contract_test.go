package contract

import "testing"

func TestSkillRiskConfirmContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleSkillChatCommandRiskConfirmRequired",
	)
}
