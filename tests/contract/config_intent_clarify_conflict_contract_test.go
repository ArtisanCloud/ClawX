package contract

import (
	"os/exec"
	"strings"
	"testing"
)

func TestConfigIntentClarifyConflictContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandMixedIntentRequiresClarification",
	)
}

func TestConfigIntentLowConfidenceContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandLowConfidenceSuggestOnly",
	)
}

func TestConfigNonAdminAllowedOpsContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestHandleConfigChatCommandNonAdminCanManagePlanButCannotApply",
	)
}

func runCmdClawxTargetedTests(t *testing.T, testNames ...string) {
	t.Helper()
	pattern := strings.Join(testNames, "|")
	cmd := exec.Command("go", "test", "./cmd/clawx", "-run", pattern, "-count=1")
	cmd.Dir = "../.."
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("targeted cmd/clawx tests failed: %v\n%s", err, string(output))
	}
}

