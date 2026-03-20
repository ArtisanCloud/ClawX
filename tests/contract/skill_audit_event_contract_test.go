package contract

import "testing"

func TestSkillAuditEventContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleSkillChatCommandAuditReplayConsistency",
	)
}
