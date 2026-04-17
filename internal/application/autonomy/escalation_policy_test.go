package autonomy

import "testing"

func TestEscalationPolicyDecide(t *testing.T) {
	policy := NewEscalationPolicy()

	if got := policy.Decide(FailureClassification{Class: FailureClassPermission, Recoverable: false}, false); !got.ShouldEscalate {
		t.Fatalf("permission should escalate")
	}
	if got := policy.Decide(FailureClassification{Class: FailureClassAuth, Recoverable: false}, false); !got.ShouldEscalate {
		t.Fatalf("auth should escalate")
	}
	if got := policy.Decide(FailureClassification{Class: FailureClassNetwork, Recoverable: true}, false); got.ShouldEscalate {
		t.Fatalf("recoverable network without exhausted retries should not escalate")
	}
	if got := policy.Decide(FailureClassification{Class: FailureClassTool, Recoverable: true}, true); !got.ShouldEscalate {
		t.Fatalf("tool with exhausted recovery should escalate")
	}
	if got := policy.Decide(FailureClassification{Class: FailureClassUnknown, Recoverable: true}, false); got.ShouldEscalate {
		t.Fatalf("unknown should not escalate before recovery probe exhausted")
	}
}
