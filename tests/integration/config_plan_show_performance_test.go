package integration

import "testing"

func TestConfigPlanShowPerformanceIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleConfigChatCommandShowPerformanceWithHighPatchCount",
	)
}
