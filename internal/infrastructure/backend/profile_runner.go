package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"clawx/internal/domain/execution"
)

type Profile struct {
	Kind       string
	Command    string
	Args       []string
	HealthArgs []string
	Model      string
}

func NewProfileRunner(name string, timeout time.Duration, profile Profile, validate ValidateCWDFunc) *DirectRunner {
	command := strings.TrimSpace(profile.Command)
	if command == "" {
		command = defaultProfileCommand(profile.Kind)
	}

	healthArgs := filterProfileHealthArgs(profile)
	return NewDirectRunner(name, timeout, Options{
		Command:     command,
		HealthArgs:  healthArgs,
		ExecuteFunc: buildProfileExecutor(Profile{Kind: profile.Kind, Command: command, Args: profile.Args, HealthArgs: healthArgs, Model: profile.Model}),
		ValidateCWD: validate,
	})
}

func buildProfileExecutor(profile Profile) ExecutorFunc {
	switch strings.ToLower(strings.TrimSpace(profile.Kind)) {
	case "codex-cli":
		return buildCodexCLIExecutor(profile)
	case "claude-cli":
		return buildClaudeCLIExecutor(profile)
	default:
		return buildCLIExecutor(strings.TrimSpace(profile.Command), profile.Args)
	}
}

func buildCodexCLIExecutor(profile Profile) ExecutorFunc {
	commandName := strings.TrimSpace(profile.Command)
	args := append([]string(nil), profile.Args...)

	return func(ctx context.Context, request execution.Request) (execution.Result, error) {
		if commandName == "" {
			return execution.Result{}, ErrMissingCommand
		}

		startedAt := time.Now().UTC()
		existingThreadID := normalizeCodexThreadID(request.BackendSessionID)
		outputFile, err := os.CreateTemp("", "clawx-codex-last-*.txt")
		if err != nil {
			return execution.Result{}, err
		}
		outputPath := outputFile.Name()
		_ = outputFile.Close()
		defer os.Remove(outputPath)

		composedInput := composeCodexExecutionInput(request)
		cmdArgs := buildCodexExecArgs(args, profile.Model, request.CWD, request.AllowedRoots, outputPath, existingThreadID, composedInput)

		cmd := exec.CommandContext(ctx, commandName, cmdArgs...)
		cmd.Dir = request.CWD

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		resolvedThreadID := existingThreadID
		if err := cmd.Run(); err != nil {
			completedAt := time.Now().UTC()
			if parsedThreadID := parseCodexThreadID(stdout.String()); parsedThreadID != "" {
				resolvedThreadID = parsedThreadID
			}
			if resolvedThreadID == "" {
				resolvedThreadID = defaultBackendSessionID(request)
			}

			failure := strings.TrimSpace(stderr.String())
			if failure == "" {
				failure = err.Error()
			}
			output := readTrimmedFile(outputPath)
			if traceErr := appendCodexTrace(
				request,
				commandName,
				cmdArgs,
				mapErrorToResultState(err),
				startedAt,
				completedAt,
				resolvedThreadID,
				output,
				stdout.String(),
				stderr.String(),
				err,
			); traceErr != nil {
				// Trace errors must not block execution results.
			}
			return execution.Result{
				BackendSessionID: resolvedThreadID,
				Output:           output,
				State:            mapErrorToResultState(err),
				StartedAt:        startedAt,
				CompletedAt:      completedAt,
				FailureReason:    failure,
			}, fmt.Errorf("run codex cli: %s", failure)
		}

		completedAt := time.Now().UTC()
		if parsedThreadID := parseCodexThreadID(stdout.String()); parsedThreadID != "" {
			resolvedThreadID = parsedThreadID
		}
		if resolvedThreadID == "" {
			resolvedThreadID = defaultBackendSessionID(request)
		}

		output := readTrimmedFile(outputPath)
		if output == "" {
			output = strings.TrimSpace(stderr.String())
		}
		if output == "" {
			output = "执行完成，无可见输出"
		}
		if traceErr := appendCodexTrace(
			request,
			commandName,
			cmdArgs,
			execution.ResultSuccess,
			startedAt,
			completedAt,
			resolvedThreadID,
			output,
			stdout.String(),
			stderr.String(),
			nil,
		); traceErr != nil {
			// Trace errors must not block execution results.
		}

		return execution.Result{
			BackendSessionID: resolvedThreadID,
			Output:           output,
			State:            execution.ResultSuccess,
			StartedAt:        startedAt,
			CompletedAt:      completedAt,
		}, nil
	}
}

func buildCodexExecArgs(baseArgs []string, model, cwd string, allowedRoots []string, outputPath, threadID, prompt string) []string {
	cmdArgs := []string{"exec"}
	cmdArgs = append(cmdArgs, baseArgs...)
	if strings.TrimSpace(model) != "" {
		cmdArgs = append(cmdArgs, "--model", strings.TrimSpace(model))
	}
	// Enforce writable workspace and explicit working directory per request.
	cmdArgs = append(cmdArgs, "--sandbox", "workspace-write")
	if strings.TrimSpace(cwd) != "" {
		cmdArgs = append(cmdArgs, "--cd", strings.TrimSpace(cwd))
	}
	for _, root := range normalizeWritableRootsForCodex(cwd, allowedRoots) {
		cmdArgs = append(cmdArgs, "--add-dir", root)
	}
	cmdArgs = append(cmdArgs, "--skip-git-repo-check", "--json", "--output-last-message", outputPath)
	if strings.TrimSpace(threadID) != "" {
		cmdArgs = append(cmdArgs, "resume", strings.TrimSpace(threadID))
	}
	cmdArgs = append(cmdArgs, prompt)
	return cmdArgs
}

func normalizeWritableRootsForCodex(cwd string, roots []string) []string {
	normalizedCWD := strings.TrimSpace(cwd)
	seen := make(map[string]struct{}, len(roots)+1)
	result := make([]string, 0, len(roots)+1)
	appendRoot := func(raw string) {
		value := strings.TrimSpace(raw)
		if value == "" {
			return
		}
		if value == normalizedCWD {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	for _, root := range roots {
		appendRoot(root)
	}
	return result
}

func buildClaudeCLIExecutor(profile Profile) ExecutorFunc {
	commandName := strings.TrimSpace(profile.Command)
	args := append([]string(nil), profile.Args...)

	return func(ctx context.Context, request execution.Request) (execution.Result, error) {
		if commandName == "" {
			return execution.Result{}, ErrMissingCommand
		}

		startedAt := time.Now().UTC()
		composedInput := composeExecutionInput(request)
		cmdArgs := append([]string(nil), args...)
		cmdArgs = append(cmdArgs, "--print", "--output-format", "text", "--no-session-persistence")
		if profile.Model != "" {
			cmdArgs = append(cmdArgs, "--model", profile.Model)
		}
		cmdArgs = append(cmdArgs, composedInput)

		cmd := exec.CommandContext(ctx, commandName, cmdArgs...)
		cmd.Dir = request.CWD

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
			}, fmt.Errorf("run claude cli: %s", failure)
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

func composeCodexExecutionInput(request execution.Request) string {
	base := composeExecutionInput(request)
	workspace := strings.TrimSpace(request.CWD)
	allowedRoots := normalizeGuardrailRoots(request.AllowedRoots, workspace)
	var b strings.Builder
	b.WriteString("Workspace Guardrails:\n")
	b.WriteString("- Current workspace: ")
	if workspace == "" {
		b.WriteString("-")
	} else {
		b.WriteString(workspace)
	}
	b.WriteString("\n")
	if len(allowedRoots) == 0 {
		b.WriteString("- Writable roots: (none declared; use current workspace only)\n")
	} else {
		b.WriteString("- Writable roots:\n")
		for _, root := range allowedRoots {
			b.WriteString("  - ")
			b.WriteString(root)
			b.WriteString("\n")
		}
	}
	b.WriteString("- For absolute paths under writable roots, execute directly without extra confirmation.\n")
	b.WriteString("- If a requested path is outside writable roots, explain the boundary and ask for authorization/config update.\n\n")
	b.WriteString(base)
	return b.String()
}

func normalizeGuardrailRoots(roots []string, workspace string) []string {
	seen := make(map[string]struct{}, len(roots)+1)
	result := make([]string, 0, len(roots)+1)
	appendRoot := func(raw string) {
		root := strings.TrimSpace(raw)
		if root == "" {
			return
		}
		if _, ok := seen[root]; ok {
			return
		}
		seen[root] = struct{}{}
		result = append(result, root)
	}
	for _, root := range roots {
		appendRoot(root)
	}
	appendRoot(workspace)
	return result
}

func defaultProfileCommand(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "codex-cli":
		return "codex"
	case "claude-cli":
		return "claude"
	default:
		return ""
	}
}

func filterProfileHealthArgs(profile Profile) []string {
	if len(profile.HealthArgs) > 0 {
		return append([]string(nil), profile.HealthArgs...)
	}
	switch strings.ToLower(strings.TrimSpace(profile.Kind)) {
	case "codex-cli", "claude-cli":
		return []string{"--version"}
	default:
		return nil
	}
}

func readTrimmedFile(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}

func normalizeCodexThreadID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "backend-") {
		return ""
	}
	return value
}

func parseCodexThreadID(raw string) string {
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var event struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if strings.TrimSpace(event.Type) == "thread.started" && strings.TrimSpace(event.ThreadID) != "" {
			return strings.TrimSpace(event.ThreadID)
		}
	}
	return ""
}
