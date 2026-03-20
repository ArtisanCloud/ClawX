package contract

import "testing"

func TestSkillRegistryMetadataContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleSkillChatCommandRegistryMetadataValidation",
	)
}
