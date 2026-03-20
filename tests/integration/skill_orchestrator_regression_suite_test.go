package integration

import "testing"

func TestSkillOrchestratorRegressionSuite(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleSkillChatCommandNaturalLanguageRoute",
		"TestHandleSkillChatCommandCommandMapping",
		"TestHandleSkillChatCommandLowConfidenceClarify",
		"TestHandleSkillChatCommandNonIntentFallback",
		"TestHandleSkillChatCommandRegistryMetadataValidation",
		"TestHandleSkillChatCommandPolicySourceReject",
		"TestHandleSkillChatCommandPolicyVersionReject",
		"TestHandleSkillChatCommandDisableImmediateEffect",
		"TestHandleSkillChatCommandAgentStateProvider",
	)
	TestSkillBindingScopeFlowIntegration(t)
	TestSkillRiskExecutionFlowIntegration(t)
}
