package integration

import "testing"

func TestSkillRegistryPolicyFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleSkillChatCommandRegistryMetadataValidation",
		"TestHandleSkillChatCommandPolicySourceReject",
		"TestHandleSkillChatCommandPolicyVersionReject",
		"TestHandleSkillChatCommandDisableImmediateEffect",
		"TestHandleSkillChatCommandAgentStateProvider",
	)
}
