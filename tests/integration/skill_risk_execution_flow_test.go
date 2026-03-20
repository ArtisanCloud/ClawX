package integration

import "testing"

func TestSkillRiskExecutionFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleSkillChatCommandRiskConfirmRequired",
		"TestHandleSkillChatCommandConfirmRejectNoSideEffect",
		"TestHandleSkillChatCommandAuditReplayConsistency",
	)
}
