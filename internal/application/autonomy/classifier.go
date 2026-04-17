package autonomy

import (
	"context"
	"errors"
	"strings"
)

type FailureClass string

const (
	FailureClassNetwork    FailureClass = "network"
	FailureClassAuth       FailureClass = "auth"
	FailureClassPermission FailureClass = "permission"
	FailureClassResource   FailureClass = "resource"
	FailureClassTool       FailureClass = "tool"
	FailureClassUnknown    FailureClass = "unknown"
)

type FailureClassification struct {
	Class       FailureClass
	Recoverable bool
	Reason      string
}

func ClassifyFailure(err error) FailureClassification {
	if err == nil {
		return FailureClassification{Class: FailureClassUnknown, Recoverable: false, Reason: "no_error"}
	}
	if errors.Is(err, context.Canceled) {
		return FailureClassification{Class: FailureClassResource, Recoverable: true, Reason: "context_canceled"}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return FailureClassification{Class: FailureClassResource, Recoverable: true, Reason: "deadline_exceeded"}
	}

	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case containsAny(lower, "permission denied", "outside allowed roots", "not allowed", "access denied", "operation not permitted"):
		return FailureClassification{Class: FailureClassPermission, Recoverable: false, Reason: "permission_or_scope_denied"}
	case containsAny(lower, "unauthorized", "forbidden", "invalid token", "authentication", "auth failed", "401", "403"):
		return FailureClassification{Class: FailureClassAuth, Recoverable: false, Reason: "auth_failed"}
	case containsAny(lower, "no such host", "dial tcp", "connection refused", "connection reset", "network is unreachable", "tls handshake", "fetch failed", "timeout awaiting", "temporary failure", "unexpected status 404", "unexpected status 5", "bad gateway", "service unavailable", "gateway timeout", "/v1/responses"):
		return FailureClassification{Class: FailureClassNetwork, Recoverable: true, Reason: "network_unreachable_or_timeout"}
	case containsAny(lower, "rate limit", "too many requests", "out of memory", "cannot allocate memory", "resource exhausted", "too many open files", "deadline exceeded", "signal: killed", "killed", "context deadline exceeded"):
		return FailureClassification{Class: FailureClassResource, Recoverable: true, Reason: "resource_exhausted"}
	case containsAny(lower, "executable file not found", "command not found", "exit code", "exit status", "tool failed", "unsupported command"):
		return FailureClassification{Class: FailureClassTool, Recoverable: true, Reason: "tool_or_command_failed"}
	default:
		return FailureClassification{Class: FailureClassUnknown, Recoverable: true, Reason: "unclassified_retryable"}
	}
}

func containsAny(text string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(text, part) {
			return true
		}
	}
	return false
}
