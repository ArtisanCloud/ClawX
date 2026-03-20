package integration

import "testing"

func TestSkillMarketplaceInstallFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleSkillChatCommandMarketplaceInstallFromPackageArchive",
		"TestInstallServiceRollbackOnRegistryFailure",
	)
}
