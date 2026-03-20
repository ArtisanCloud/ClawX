package contract

import "testing"

func TestSkillAgentStateIntegrationContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleSkillChatCommandAgentStateProvider",
	)
}
