package integration

import "testing"

func TestAutonomyRecoveryFlowIntegration(t *testing.T) {
	runIntegrationTargetsOnPackage(t, "./cmd/clawx",
		"TestTryRecoverSessionFlowSuccessAfterRetry",
		"TestTryRecoverSessionFlowFailureConverges",
		"TestHandleExecutionFailureEscalationPrompt",
	)
}
