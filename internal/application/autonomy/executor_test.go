package autonomy

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRecoveryExecutor_RecoversWithinRetries(t *testing.T) {
	executor := NewRecoveryExecutor(NewDefaultRecoveryRegistry())
	executor.sleepFn = func(context.Context, time.Duration) error { return nil }

	attempts := 0
	result := executor.Execute(context.Background(), FailureClassification{
		Class:       FailureClassNetwork,
		Recoverable: true,
		Reason:      "network_unreachable_or_timeout",
	}, func(_ context.Context, attempt int) error {
		attempts++
		if attempt >= 2 {
			return nil
		}
		return errors.New("temporary network failure")
	})

	if !result.Recovered {
		t.Fatalf("expected recovered result: %#v", result)
	}
	if result.Attempts != 2 || attempts != 2 {
		t.Fatalf("expected 2 attempts, got result=%d local=%d", result.Attempts, attempts)
	}
}

func TestRecoveryExecutor_SkipsPermissionPolicy(t *testing.T) {
	executor := NewRecoveryExecutor(NewDefaultRecoveryRegistry())
	called := 0
	result := executor.Execute(context.Background(), FailureClassification{
		Class:       FailureClassPermission,
		Recoverable: false,
		Reason:      "permission_or_scope_denied",
	}, func(_ context.Context, _ int) error {
		called++
		return nil
	})
	if !result.Skipped || result.StopReason != "not_retryable" {
		t.Fatalf("expected skipped non-retryable, got: %#v", result)
	}
	if called != 0 {
		t.Fatalf("attempt function should not be called, got: %d", called)
	}
}

func TestRecoveryExecutor_ExhaustsRetries(t *testing.T) {
	executor := NewRecoveryExecutor(NewDefaultRecoveryRegistry())
	executor.sleepFn = func(context.Context, time.Duration) error { return nil }
	result := executor.Execute(context.Background(), FailureClassification{
		Class:       FailureClassTool,
		Recoverable: true,
		Reason:      "tool_or_command_failed",
	}, func(_ context.Context, _ int) error {
		return errors.New("tool failed")
	})
	if result.Recovered {
		t.Fatalf("expected unrecovered result")
	}
	if result.StopReason != "max_retries_exhausted" {
		t.Fatalf("unexpected stop reason: %s", result.StopReason)
	}
	if result.Attempts != 2 {
		t.Fatalf("expected 2 attempts for tool policy, got: %d", result.Attempts)
	}
}
