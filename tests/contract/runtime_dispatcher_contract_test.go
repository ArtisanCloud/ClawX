package contract

import (
	"os/exec"
	"strings"
	"testing"
)

func TestRuntimeDispatcherContract(t *testing.T) {
	runRuntimeOrchestratorTargetedTests(t,
		"TestDispatcherIdleFirstAndStickyResource",
		"TestPickDispatchWorkerStickyFallbackToIdle",
	)
	runCmdClawxTargetedTests(t,
		"TestApplyRuntimeExecPlanLeadModeDispatchOnly",
	)
}

func runRuntimeOrchestratorTargetedTests(t *testing.T, testNames ...string) {
	t.Helper()
	pattern := strings.Join(testNames, "|")
	cmd := exec.Command("go", "test", "./internal/application/runtimeorchestrator", "-run", pattern, "-count=1")
	cmd.Dir = "../.."
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("targeted runtimeorchestrator tests failed: %v\n%s", err, string(output))
	}
}
