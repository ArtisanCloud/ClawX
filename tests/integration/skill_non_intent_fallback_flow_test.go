package integration

import "testing"

func TestSkillNonIntentFallbackFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleSkillChatCommandNonIntentFallback",
	)
}
