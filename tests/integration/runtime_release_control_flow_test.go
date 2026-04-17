package integration

import "testing"

func TestRuntimeReleaseControlFlowIntegration(t *testing.T) {
	runIntegrationTargetsOnPackage(t, "./cmd/clawx",
		"TestMaybeAutoApplyActionPlanRuntimeReleaseTaskControlClosedLoop",
	)
}
