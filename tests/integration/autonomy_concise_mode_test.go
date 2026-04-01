package integration

import "testing"

func TestAutonomyConciseModeIntegration(t *testing.T) {
	runIntegrationTargetsOnPackage(t, "./cmd/clawx",
		"TestSuppressVerboseCodeBlocks",
		"TestShouldDeliverOutputFiles",
		"TestFormatTaskReceiptProgress",
		"TestFormatExecutionFailureResponseNoEscalation",
	)
}
