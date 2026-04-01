package contract

import (
	"testing"

	"clawx/internal/application/autonomy"
)

func TestAutonomyNonNecessaryEscalationContract(t *testing.T) {
	policy := autonomy.NewEscalationPolicy()
	decision := policy.Decide(autonomy.FailureClassification{
		Class:       autonomy.FailureClassNetwork,
		Recoverable: true,
		Reason:      "network_unreachable_or_timeout",
	}, false)
	if decision.ShouldEscalate {
		t.Fatalf("recoverable network failure should not escalate before recovery exhausted")
	}

	permission := policy.Decide(autonomy.FailureClassification{
		Class:       autonomy.FailureClassPermission,
		Recoverable: false,
		Reason:      "permission_or_scope_denied",
	}, false)
	if !permission.ShouldEscalate {
		t.Fatalf("permission failure must escalate")
	}
}
