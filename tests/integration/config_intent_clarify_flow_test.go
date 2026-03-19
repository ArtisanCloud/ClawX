package integration

import (
	"os/exec"
	"strings"
	"testing"
)

func TestConfigIntentClarifyFlowIntegration(t *testing.T) {
	runCmdClawxIntegrationTargets(t,
		"TestHandleConfigChatCommandMixedIntentConfirmConfigAppliesConfigPath",
		"TestHandleConfigChatCommandMixedIntentConfirmTaskStopsConfigFlow",
	)
}

func runCmdClawxIntegrationTargets(t *testing.T, testNames ...string) {
	t.Helper()
	pattern := strings.Join(testNames, "|")
	cmd := exec.Command("go", "test", "./cmd/clawx", "-run", pattern, "-count=1")
	cmd.Dir = "../.."
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("targeted cmd/clawx tests failed: %v\n%s", err, string(output))
	}
}

