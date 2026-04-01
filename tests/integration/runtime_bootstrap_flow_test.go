package integration

import "testing"

func TestRuntimeBootstrapFlowIntegration(t *testing.T) {
	runIntegrationTargetsOnPackage(t, "./cmd/clawx",
		"TestMaybeAutoApplyActionPlanRuntimeBootstrap",
	)
	runIntegrationTargetsOnPackage(t, "./internal/application/runtimeorchestrator",
		"TestBootstrapCreatesRuntimeStateFiles",
		"TestBootstrapUsesDefaultWorkerRoles",
	)
}
