package integration

import "testing"

func TestSkillBuiltinOverrideResolutionFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleSkillChatCommandBuiltinGlobalAndAgentOverrideResolution",
	)
}
