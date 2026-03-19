package integration

import "testing"

func TestConfigPlanMetricsStepCountIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleConfigChatCommandInteractionStepMetric",
	)
}

