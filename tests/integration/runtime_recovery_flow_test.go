package integration

import "testing"

func TestRuntimeRecoveryFlowIntegration(t *testing.T) {
	runIntegrationTargetsOnPackage(t, "./internal/application/runtimeorchestrator",
		"TestRecoveryEngineRequeueAndReassignFromStaleWorker",
	)
}
