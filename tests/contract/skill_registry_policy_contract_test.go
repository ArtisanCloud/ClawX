package contract

import "testing"

func TestSkillRegistryPolicyContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleSkillChatCommandRegistryMetadataValidation",
		"TestHandleSkillChatCommandPolicySourceReject",
		"TestHandleSkillChatCommandPolicyVersionReject",
		"TestHandleSkillChatCommandDisableImmediateEffect",
	)
}
