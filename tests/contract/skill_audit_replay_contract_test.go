package contract

import "testing"

func TestSkillAuditReplayContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleSkillChatCommandAuditReplayConsistency",
	)
}
