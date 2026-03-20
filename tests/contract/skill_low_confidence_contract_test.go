package contract

import "testing"

func TestSkillLowConfidenceContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleSkillChatCommandLowConfidenceClarify",
	)
}
