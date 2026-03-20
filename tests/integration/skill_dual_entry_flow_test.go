package integration

import "testing"

func TestSkillDualEntryFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleSkillChatCommandNaturalLanguageRoute",
		"TestHandleSkillChatCommandCommandMapping",
	)
}
