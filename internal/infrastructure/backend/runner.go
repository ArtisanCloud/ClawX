package backend

import (
	"context"
	"fmt"
	"sync"
	"time"

	"synapsex/internal/domain/execution"
)

type ExecutorFunc func(ctx context.Context, request execution.Request) (execution.Result, error)

type DirectRunner struct {
	name        string
	timeout     time.Duration
	executeFunc ExecutorFunc

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func NewDirectRunner(name string, timeout time.Duration, executeFunc ExecutorFunc) *DirectRunner {
	if name == "" {
		name = "primary"
	}
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	if executeFunc == nil {
		executeFunc = defaultExecute
	}
	return &DirectRunner{
		name:        name,
		timeout:     timeout,
		executeFunc: executeFunc,
		cancels:     make(map[string]context.CancelFunc),
	}
}

func (r *DirectRunner) Name() string {
	return r.name
}

func (r *DirectRunner) Execute(ctx context.Context, request execution.Request) (execution.Result, error) {
	runCtx, cancel, timeout := r.withTimeout(ctx, request)
	r.registerCancel(request.SessionID, cancel)
	defer func() {
		r.clearCancel(request.SessionID)
		cancel()
	}()

	result, err := r.executeFunc(runCtx, request)
	if err != nil {
		return execution.Result{
			BackendSessionID: request.BackendSessionID,
			State:            mapErrorToResultState(err),
			StartedAt:        time.Now().UTC(),
			CompletedAt:      time.Now().UTC(),
			FailureReason:    err.Error(),
		}, err
	}

	if result.BackendSessionID == "" {
		result.BackendSessionID = defaultBackendSessionID(request)
	}
	if result.State == "" {
		result.State = execution.ResultSuccess
	}
	if result.StartedAt.IsZero() {
		result.StartedAt = time.Now().UTC()
	}
	if result.CompletedAt.IsZero() {
		result.CompletedAt = result.StartedAt.Add(timeout)
		if result.State == execution.ResultSuccess {
			result.CompletedAt = time.Now().UTC()
		}
	}
	return result, nil
}

func (r *DirectRunner) HealthCheck(ctx context.Context) error {
	_, err := r.executeFunc(ctx, execution.Request{
		SessionID: "health",
		Input:     "health",
		Timeout:   100 * time.Millisecond,
	})
	return err
}

func defaultExecute(ctx context.Context, request execution.Request) (execution.Result, error) {
	startedAt := time.Now().UTC()
	select {
	case <-ctx.Done():
		return execution.Result{}, ctx.Err()
	default:
	}

	return execution.Result{
		BackendSessionID: defaultBackendSessionID(request),
		Output:           fmt.Sprintf("accepted: %s", request.Input),
		State:            execution.ResultSuccess,
		StartedAt:        startedAt,
		CompletedAt:      time.Now().UTC(),
	}, nil
}

func defaultBackendSessionID(request execution.Request) string {
	if request.BackendSessionID != "" {
		return request.BackendSessionID
	}
	return fmt.Sprintf("backend-%s", request.SessionID)
}

