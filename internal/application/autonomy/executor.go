package autonomy

import (
	"context"
	"fmt"
	"time"
)

type AttemptFunc func(ctx context.Context, attempt int) error

type RecoveryExecutionResult struct {
	Policy      RecoveryPolicy
	Attempts    int
	Recovered   bool
	Skipped     bool
	LastError   error
	StopReason  string
	Duration    time.Duration
	Recoverable bool
}

type RecoveryExecutor struct {
	registry *RecoveryRegistry
	sleepFn  func(context.Context, time.Duration) error
	nowFn    func() time.Time
}

func NewRecoveryExecutor(registry *RecoveryRegistry) *RecoveryExecutor {
	if registry == nil {
		registry = NewDefaultRecoveryRegistry()
	}
	return &RecoveryExecutor{
		registry: registry,
		sleepFn:  sleepWithContext,
		nowFn:    time.Now,
	}
}

func (e *RecoveryExecutor) Execute(ctx context.Context, classification FailureClassification, attempt AttemptFunc) RecoveryExecutionResult {
	started := e.nowFn()
	result := RecoveryExecutionResult{
		Recoverable: classification.Recoverable,
	}
	if attempt == nil {
		result.Skipped = true
		result.StopReason = "attempt_func_missing"
		result.Duration = e.nowFn().Sub(started)
		return result
	}
	policy, ok := e.registry.Resolve(classification.Class)
	if !ok {
		result.Skipped = true
		result.StopReason = "policy_not_found"
		result.Duration = e.nowFn().Sub(started)
		return result
	}
	result.Policy = policy
	if !classification.Recoverable || !policy.Retryable || policy.MaxRetries <= 0 {
		result.Skipped = true
		result.StopReason = "not_retryable"
		result.Duration = e.nowFn().Sub(started)
		return result
	}

	runCtx := ctx
	cancel := func() {}
	if policy.MaxDuration > 0 {
		runCtx, cancel = context.WithTimeout(ctx, policy.MaxDuration)
	}
	defer cancel()

	for current := 1; current <= policy.MaxRetries; current++ {
		result.Attempts++
		err := attempt(runCtx, current)
		if err == nil {
			result.Recovered = true
			result.StopReason = "recovered"
			result.Duration = e.nowFn().Sub(started)
			return result
		}
		result.LastError = err
		if current == policy.MaxRetries {
			result.StopReason = "max_retries_exhausted"
			break
		}
		backoff := policy.BackoffForAttempt(current)
		if backoff <= 0 {
			continue
		}
		if sleepErr := e.sleepFn(runCtx, backoff); sleepErr != nil {
			result.LastError = fmt.Errorf("sleep interrupted: %w", sleepErr)
			result.StopReason = "sleep_interrupted"
			break
		}
	}
	result.Duration = e.nowFn().Sub(started)
	return result
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
