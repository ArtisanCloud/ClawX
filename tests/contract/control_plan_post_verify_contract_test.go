package contract

import "testing"

func TestControlPlanPostVerifyContract(t *testing.T) {
	runCmdClawxTargetedTests(t,
		"TestMaybeAutoApplyAgentSwitchFromModelOutput",
	)
}
