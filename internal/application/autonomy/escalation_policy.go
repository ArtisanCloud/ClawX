package autonomy

type EscalationDecision struct {
	ShouldEscalate bool
	Reason         string
}

type EscalationPolicy struct{}

func NewEscalationPolicy() EscalationPolicy {
	return EscalationPolicy{}
}

func (EscalationPolicy) Decide(classification FailureClassification, recoveryExhausted bool) EscalationDecision {
	switch classification.Class {
	case FailureClassPermission, FailureClassAuth:
		return EscalationDecision{ShouldEscalate: true, Reason: "credential_or_permission_required"}
	case FailureClassUnknown:
		if recoveryExhausted {
			return EscalationDecision{ShouldEscalate: true, Reason: "unknown_failure_recovery_exhausted"}
		}
		return EscalationDecision{ShouldEscalate: false, Reason: "unknown_failure_should_probe_once"}
	case FailureClassNetwork, FailureClassTool, FailureClassResource:
		if recoveryExhausted {
			return EscalationDecision{ShouldEscalate: true, Reason: "recovery_exhausted"}
		}
		return EscalationDecision{ShouldEscalate: false, Reason: "recoverable_error_should_auto_handle"}
	default:
		return EscalationDecision{ShouldEscalate: false, Reason: "no_escalation_required"}
	}
}
