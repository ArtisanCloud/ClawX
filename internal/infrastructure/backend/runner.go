package backend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"clawx/internal/domain/execution"
)

type ExecutorFunc func(ctx context.Context, request execution.Request) (execution.Result, error)

type ValidateCWDFunc func(cwd string) error

type Options struct {
	Command     string
	Args        []string
	HealthArgs  []string
	ExecuteFunc ExecutorFunc
	ValidateCWD ValidateCWDFunc
}

var ErrMissingCommand = errors.New("backend command is not configured")

type DirectRunner struct {
	name        string
	timeout     time.Duration
	command     string
	args        []string
	healthArgs  []string
	executeFunc ExecutorFunc
	validateCWD ValidateCWDFunc

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func NewDirectRunner(name string, timeout time.Duration, options Options) *DirectRunner {
	if name == "" {
		name = "primary"
	}
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	if strings.TrimSpace(options.Command) == "" && options.ExecuteFunc == nil {
		options.Command = "cat"
	}
	if options.ExecuteFunc == nil {
		options.ExecuteFunc = buildCLIExecutor(strings.TrimSpace(options.Command), options.Args)
	}

	return &DirectRunner{
		name:        name,
		timeout:     timeout,
		command:     strings.TrimSpace(options.Command),
		args:        append([]string(nil), options.Args...),
		healthArgs:  append([]string(nil), options.HealthArgs...),
		executeFunc: options.ExecuteFunc,
		validateCWD: options.ValidateCWD,
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

	if r.validateCWD != nil {
		if err := r.validateCWD(request.CWD); err != nil {
			return execution.Result{
				BackendSessionID: request.BackendSessionID,
				State:            execution.ResultFailed,
				StartedAt:        time.Now().UTC(),
				CompletedAt:      time.Now().UTC(),
				FailureReason:    err.Error(),
			}, err
		}
	}

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
	commandName := strings.TrimSpace(r.command)
	if commandName == "" {
		return ErrMissingCommand
	}

	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(healthCtx, commandName, r.healthArgs...)
	if len(r.healthArgs) == 0 {
		cmd.Stdin = strings.NewReader("health")
	}

	var stderr bytes.Buffer
	cmd.Stdout = &bytes.Buffer{}
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("backend health check failed: %s", message)
	}
	return nil
}

func buildCLIExecutor(commandName string, args []string) ExecutorFunc {
	return func(ctx context.Context, request execution.Request) (execution.Result, error) {
		if strings.TrimSpace(commandName) == "" {
			return execution.Result{}, ErrMissingCommand
		}

		startedAt := time.Now().UTC()
		cmd := exec.CommandContext(ctx, commandName, args...)
		cmd.Dir = request.CWD
		cmd.Stdin = strings.NewReader(composeExecutionInput(request))

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			failure := strings.TrimSpace(stderr.String())
			if failure == "" {
				failure = err.Error()
			}
			return execution.Result{
				BackendSessionID: defaultBackendSessionID(request),
				Output:           strings.TrimSpace(stdout.String()),
				State:            mapErrorToResultState(err),
				StartedAt:        startedAt,
				CompletedAt:      time.Now().UTC(),
				FailureReason:    failure,
			}, fmt.Errorf("run backend command: %s", failure)
		}

		output := strings.TrimSpace(stdout.String())
		if output == "" {
			output = strings.TrimSpace(stderr.String())
		}

		return execution.Result{
			BackendSessionID: defaultBackendSessionID(request),
			Output:           output,
			State:            execution.ResultSuccess,
			StartedAt:        startedAt,
			CompletedAt:      time.Now().UTC(),
		}, nil
	}
}

func defaultBackendSessionID(request execution.Request) string {
	if request.BackendSessionID != "" {
		return request.BackendSessionID
	}
	return fmt.Sprintf("backend-%s", request.SessionID)
}

func composeExecutionInput(request execution.Request) string {
	memoryContext := strings.TrimSpace(request.MemoryContext)
	input := strings.TrimSpace(request.Input)
	if memoryContext == "" {
		return request.Input
	}
	if input == "" {
		return memoryContext
	}
	return memoryContext + "\n\n---\n\n" + request.Input
}
