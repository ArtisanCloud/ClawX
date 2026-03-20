package integration

import "testing"

func TestSkillNLRoutingFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleSkillChatCommandNaturalLanguageRoute",
	)
}
