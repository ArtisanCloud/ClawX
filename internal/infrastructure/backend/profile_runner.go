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
	"unicode/utf8"

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
		cmdArgs := buildCodexExecArgs(
			args,
			profile.Model,
			request.CWD,
			request.AllowedRoots,
			request.PromptCacheKey,
			request.PromptCacheRetention,
			outputPath,
			existingThreadID,
			composedInput,
		)

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

			failure := parseCodexFailureMessage(stdout.String())
			if failure == "" {
				failure = strings.TrimSpace(stderr.String())
			}
			if failure == "" {
				failure = err.Error()
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				lowerFailure := strings.ToLower(strings.TrimSpace(failure))
				if lowerFailure == "" || strings.Contains(lowerFailure, "signal: killed") {
					failure = ctxErr.Error()
				}
			}
			output := readTrimmedFile(outputPath)
			cachedTokens, promptTokens, completionTokens, totalTokens := parseCodexPromptUsage(stdout.String())
			if traceErr := appendCodexTrace(
				request,
				commandName,
				cmdArgs,
				mapErrorToResultState(err),
				startedAt,
				completedAt,
				resolvedThreadID,
				cachedTokens,
				promptTokens,
				completionTokens,
				totalTokens,
				output,
				stdout.String(),
				stderr.String(),
				err,
			); traceErr != nil {
				// Trace errors must not block execution results.
			}
			return execution.Result{
				BackendSessionID:     resolvedThreadID,
				Output:               output,
				PromptCacheKey:       strings.TrimSpace(request.PromptCacheKey),
				PromptCacheRetention: strings.TrimSpace(request.PromptCacheRetention),
				PromptCachedTokens:   cachedTokens,
				PromptTokens:         promptTokens,
				CompletionTokens:     completionTokens,
				TotalTokens:          totalTokens,
				State:                mapErrorToResultState(err),
				StartedAt:            startedAt,
				CompletedAt:          completedAt,
				FailureReason:        failure,
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
		cachedTokens, promptTokens, completionTokens, totalTokens := parseCodexPromptUsage(stdout.String())
		if traceErr := appendCodexTrace(
			request,
			commandName,
			cmdArgs,
			execution.ResultSuccess,
			startedAt,
			completedAt,
			resolvedThreadID,
			cachedTokens,
			promptTokens,
			completionTokens,
			totalTokens,
			output,
			stdout.String(),
			stderr.String(),
			nil,
		); traceErr != nil {
			// Trace errors must not block execution results.
		}

		return execution.Result{
			BackendSessionID:     resolvedThreadID,
			Output:               output,
			PromptCacheKey:       strings.TrimSpace(request.PromptCacheKey),
			PromptCacheRetention: strings.TrimSpace(request.PromptCacheRetention),
			PromptCachedTokens:   cachedTokens,
			PromptTokens:         promptTokens,
			CompletionTokens:     completionTokens,
			TotalTokens:          totalTokens,
			State:                execution.ResultSuccess,
			StartedAt:            startedAt,
			CompletedAt:          completedAt,
		}, nil
	}
}

func buildCodexExecArgs(baseArgs []string, model, cwd string, allowedRoots []string, promptCacheKey string, promptCacheRetention string, outputPath, threadID, prompt string) []string {
	cmdArgs := []string{"exec"}
	cmdArgs = append(cmdArgs, sanitizeCodexCLIArgs(baseArgs)...)
	if strings.TrimSpace(model) != "" {
		cmdArgs = append(cmdArgs, "--model", sanitizeCodexCLIArg(strings.TrimSpace(model)))
	}
	if value := strings.TrimSpace(promptCacheKey); value != "" {
		cmdArgs = append(cmdArgs, "-c", sanitizeCodexCLIArg(fmt.Sprintf("prompt_cache_key=%q", value)))
	}
	if value := strings.TrimSpace(promptCacheRetention); value != "" {
		cmdArgs = append(cmdArgs, "-c", sanitizeCodexCLIArg(fmt.Sprintf("prompt_cache_retention=%q", value)))
	}
	// Enforce writable workspace and explicit working directory per request.
	cmdArgs = append(cmdArgs, "--sandbox", "workspace-write")
	if strings.TrimSpace(cwd) != "" {
		cmdArgs = append(cmdArgs, "--cd", sanitizeCodexCLIArg(strings.TrimSpace(cwd)))
	}
	for _, root := range normalizeWritableRootsForCodex(cwd, allowedRoots) {
		cmdArgs = append(cmdArgs, "--add-dir", sanitizeCodexCLIArg(root))
	}
	cmdArgs = append(cmdArgs, "--skip-git-repo-check", "--json", "--output-last-message", sanitizeCodexCLIArg(outputPath))
	if strings.TrimSpace(threadID) != "" {
		cmdArgs = append(cmdArgs, "resume", sanitizeCodexCLIArg(strings.TrimSpace(threadID)))
	}
	cmdArgs = append(cmdArgs, sanitizeCodexCLIArg(prompt))
	return cmdArgs
}

func sanitizeCodexCLIArg(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return value
	}
	if utf8.ValidString(value) {
		return value
	}
	// Replace invalid sequences so codex exec never fails on malformed bytes.
	return strings.ToValidUTF8(value, " ")
}

func sanitizeCodexCLIArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	out := make([]string, 0, len(args))
	for _, arg := range args {
		out = append(out, sanitizeCodexCLIArg(arg))
	}
	return out
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

func parseCodexPromptUsage(raw string) (cachedTokens int, promptTokens int, completionTokens int, totalTokens int) {
	lines := strings.Split(raw, "\n")
	cachedTokens = 0
	promptTokens = 0
	completionTokens = 0
	totalTokens = 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			continue
		}
		if value, ok := nestedInt(payload, "usage", "prompt_tokens_details", "cached_tokens"); ok {
			cachedTokens = value
		}
		if value, ok := nestedInt(payload, "response", "usage", "prompt_tokens_details", "cached_tokens"); ok {
			cachedTokens = value
		}
		if value, ok := nestedInt(payload, "usage", "prompt_tokens"); ok {
			promptTokens = value
		}
		if value, ok := nestedInt(payload, "response", "usage", "prompt_tokens"); ok {
			promptTokens = value
		}
		if value, ok := nestedInt(payload, "usage", "completion_tokens"); ok {
			completionTokens = value
		}
		if value, ok := nestedInt(payload, "response", "usage", "completion_tokens"); ok {
			completionTokens = value
		}
		if value, ok := nestedInt(payload, "usage", "total_tokens"); ok {
			totalTokens = value
		}
		if value, ok := nestedInt(payload, "response", "usage", "total_tokens"); ok {
			totalTokens = value
		}
	}
	if totalTokens == 0 && (promptTokens > 0 || completionTokens > 0) {
		totalTokens = promptTokens + completionTokens
	}
	return cachedTokens, promptTokens, completionTokens, totalTokens
}

func parseCodexFailureMessage(raw string) string {
	lines := strings.Split(raw, "\n")
	message := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var event struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Error   *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		switch strings.TrimSpace(event.Type) {
		case "turn.failed":
			if event.Error != nil && strings.TrimSpace(event.Error.Message) != "" {
				message = strings.TrimSpace(event.Error.Message)
			}
		case "error":
			if strings.TrimSpace(event.Message) != "" {
				message = strings.TrimSpace(event.Message)
			}
		}
	}
	return compactCodexFailureMessage(message)
}

func compactCodexFailureMessage(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	value = strings.Join(strings.Fields(value), " ")
	const maxLen = 800
	if len(value) > maxLen {
		return strings.TrimSpace(value[:maxLen]) + "..."
	}
	return value
}

func nestedInt(payload map[string]any, path ...string) (int, bool) {
	if len(path) == 0 {
		return 0, false
	}
	var current any = payload
	for _, segment := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return 0, false
		}
		next, exists := obj[segment]
		if !exists {
			return 0, false
		}
		current = next
	}
	switch value := current.(type) {
	case float64:
		return int(value), true
	case int:
		return value, true
	case int64:
		return int(value), true
	case json.Number:
		if parsed, err := value.Int64(); err == nil {
			return int(parsed), true
		}
	}
	return 0, false
}
