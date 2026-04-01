package contract

import "testing"

func TestSpecKitGateAndReceiptContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestMaybeAutoApplyActionPlanSpecKitCompletenessGate",
		"TestMaybeAutoApplyActionPlanSpecKitCompletenessGateFromCommandPath",
		"TestApplyExecutionCompletionGateSkipsForSpecKitContinuation",
		"TestRenderRuntimeExecUserMessageStructuredSections",
	)
}
