package backend

import (
	"context"
	"errors"
	"time"

	"clawx/internal/domain/execution"
)

func (r *DirectRunner) withTimeout(parent context.Context, request execution.Request) (context.Context, context.CancelFunc, time.Duration) {
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = r.timeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	return ctx, cancel, timeout
}

func (r *DirectRunner) registerCancel(sessionID string, cancel context.CancelFunc) {
	if sessionID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancels[sessionID] = cancel
}

func (r *DirectRunner) clearCancel(sessionID string) {
	if sessionID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cancels, sessionID)
}

func mapErrorToResultState(err error) execution.ResultState {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return execution.ResultTimeout
	case errors.Is(err, context.Canceled):
		return execution.ResultCancelled
	default:
		return execution.ResultFailed
	}
}
