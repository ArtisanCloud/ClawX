package integration

import (
	"os/exec"
	"strings"
	"testing"
)

func TestStagedRoutingFlowIntegration(t *testing.T) {
	runIntegrationTargetsOnPackage(t, "./internal/application/skillorchestrator",
		"TestStagedRouterPlanRouteHitSkill",
		"TestStagedRouterPlanShortCircuitLowComplexity",
		"TestStagedRouterPlanFallbackNoExecute",
	)
}

func runIntegrationTargetsOnPackage(t *testing.T, pkg string, testNames ...string) {
	t.Helper()
	pattern := strings.Join(testNames, "|")
	cmd := exec.Command("go", "test", pkg, "-run", pattern, "-count=1")
	cmd.Dir = "../.."
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("targeted tests failed: pkg=%s err=%v\n%s", pkg, err, string(output))
	}
}
