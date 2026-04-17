package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"clawx/internal/application/runtimeorchestrator"
	"clawx/internal/infrastructure/config"
)

type runtimeExecCommand struct {
	Cmd            string `json:"cmd"`
	CWD            string `json:"cwd,omitempty"`
	Reason         string `json:"reason,omitempty"`
	FallbackSource string `json:"fallback_source,omitempty"`
}

type runtimeExecPlan struct {
	Type                           string               `json:"type"`
	Mode                           string               `json:"mode"`
	Commands                       []runtimeExecCommand `json:"commands"`
	Reason                         string               `json:"reason,omitempty"`
	RuntimeExecDecisionMode        string               `json:"runtime_exec_decision_mode,omitempty"`
	RuntimeExecDecisionApplySource string               `json:"runtime_exec_decision_apply_source,omitempty"`
	RuntimeExecDecisionLockSource  string               `json:"runtime_exec_decision_lock_source,omitempty"`
}

type runtimeExecApplyResult struct {
	Applied     bool
	Status      string
	Message     string
	Executed    int
	Failed      int
	Total       int
	FirstReason string
	FailedCmd   string
	SuccessInfo []string
	ExecIDs     []string
}

type runtimeExecFailureState struct {
	signature string
	count     int
	updatedAt time.Time
}

type runtimeExecFailureTracker struct {
	mu             sync.Mutex
	byConversation map[string]runtimeExecFailureState
}

var globalRuntimeExecFailureTracker = &runtimeExecFailureTracker{
	byConversation: map[string]runtimeExecFailureState{},
}

func formatRuntimeExecApplyResult(result runtimeExecApplyResult) string {
	if msg := strings.TrimSpace(result.Message); msg != "" {
		return msg
	}
	return fmt.Sprintf("runtime.exec status=%s executed=%d failed=%d", strings.TrimSpace(result.Status), result.Executed, result.Failed)
}

func applyRuntimeExecPlan(
	ctx context.Context,
	runtime agentRuntime,
	plan runtimeExecPlan,
	fallbackCWD string,
	conversationID string,
) runtimeExecApplyResult {
	if strings.TrimSpace(plan.Reason) != "" {
		appendTrackingGoal(runtime, fallbackCWD, conversationID, strings.TrimSpace(plan.Reason))
	}
	if plan.Mode != "execute" {
		reason := strings.TrimSpace(plan.Reason)
		if reason == "" {
			reason = "计划为建议模式，未自动执行。"
		}
		return runtimeExecApplyResult{
			Applied: false,
			Status:  "suggest",
			Message: reason,
		}
	}
	if len(plan.Commands) == 0 {
		return runtimeExecApplyResult{
			Applied: false,
			Status:  "rejected",
			Message: "runtime.exec 缺少 commands，未执行。",
		}
	}
	workspaceRoot := resolveRuntimeExecWorkspaceRoot(runtime, fallbackCWD)
	if leadMode, metaPath, leadErr := loadLeadModeFromRuntimeMeta(workspaceRoot); leadErr == nil && leadMode {
		scheduled := scheduleRuntimeExecViaLead(runtime, plan, workspaceRoot, metaPath, conversationID)
		appendTrackingExecution(runtime, workspaceRoot, conversationID, scheduled)
		return scheduled
	}

	executed := 0
	failed := 0
	execIDs := make([]string, 0, len(plan.Commands))
	firstReason := ""
	firstFailedCmd := ""
	successInfo := make([]string, 0, 2)
	decisionMode := normalizeRuntimeExecDecisionMode(plan.RuntimeExecDecisionMode)
	decisionApplySource := strings.TrimSpace(plan.RuntimeExecDecisionApplySource)
	decisionLockSource := strings.TrimSpace(plan.RuntimeExecDecisionLockSource)
	if decisionMode != "" && decisionLockSource == "" {
		_, lockSource := resolveRuntimeExecDecisionLockState(conversationID)
		decisionLockSource = strings.TrimSpace(lockSource)
	}
	for idx, step := range plan.Commands {
		cmdline := strings.TrimSpace(step.Cmd)
		if cmdline == "" {
			failed++
			if firstReason == "" {
				firstReason = fmt.Sprintf("step %d 空命令", idx+1)
			}
			continue
		}
		if err := validateRuntimeExecCommand(cmdline); err != nil {
			failed++
			if firstReason == "" {
				firstReason = fmt.Sprintf("step %d 命令被拒绝（%v）", idx+1, err)
			}
			continue
		}

		execCWD := strings.TrimSpace(step.CWD)
		if execCWD == "" {
			execCWD = strings.TrimSpace(fallbackCWD)
		}
		if execCWD == "" {
			execCWD = "."
		}
		if err := runtime.cfgSnapshot.ValidateWorkingDirectory(execCWD); err != nil {
			failed++
			if firstReason == "" {
				firstReason = fmt.Sprintf("step %d cwd 超出允许范围（%v）", idx+1, err)
			}
			continue
		}
		if suppressed, suppressedCmd := shouldSuppressDuplicateRuntimeExec(conversationID, execCWD, cmdline); suppressed {
			failed++
			if firstReason == "" {
				firstReason = "重复失败命令已抑制：" + suppressedCmd
			}
			if firstFailedCmd == "" {
				firstFailedCmd = suppressedCmd
			}
			continue
		}

		out, runErr, execID := performRuntimeExecStep(
			ctx,
			runtime,
			conversationID,
			execCWD,
			cmdline,
			strings.TrimSpace(plan.Reason),
			strings.TrimSpace(step.Reason),
			decisionMode,
			decisionApplySource,
			decisionLockSource,
			strings.TrimSpace(step.FallbackSource),
		)
		execIDs = append(execIDs, execID)
		if runErr != nil {
			failed++
			if firstReason == "" {
				firstReason = fmt.Sprintf("step %d 执行失败：%s", idx+1, extractFailureEvidence(out, runErr.Error()))
			}
			if firstFailedCmd == "" {
				firstFailedCmd = summarizeCommand(cmdline)
			}
			continue
		}
		executed++
		if len(successInfo) < 2 {
			if info := summarizeSuccessEvidence(step, execCWD, out); info != "" {
				successInfo = append(successInfo, info)
			}
		}
	}
	autoRepairRounds := 0
	decisionNeeded := false
	decisionReason := ""
	executed, failed, execIDs, firstReason, autoRepairRounds, decisionNeeded, decisionReason = tryAutoRepairLoop(ctx, runtime, conversationID, fallbackCWD, firstReason, executed, failed, execIDs)

	status := "applied"
	if failed > 0 {
		status = "partial"
	}
	if executed == 0 {
		status = "failed"
	}
	if firstReason == "" {
		firstReason = "无"
	}
	decisionNeeded, decisionReason = augmentDecisionFromFailureHistory(conversationID, firstReason, decisionNeeded, decisionReason)
	logPath := filepath.Join(config.StateDir(), "logs", "runtime_exec.jsonl")
	execIDLine := "exec_ids: " + strings.Join(execIDs, ", ")
	if len(execIDs) == 0 {
		execIDLine = "exec_ids: (none)"
	}
	failureEvidence := collectUniqueFailureEvidence(execIDs, 3)
	totalSteps := executed + failed
	decisionContextLine := buildRuntimeExecDecisionContextLine(plan)
	message := renderRuntimeExecUserMessage(status, totalSteps, executed, failed, firstReason, firstFailedCmd, successInfo, failureEvidence, logPath, execIDLine, autoRepairRounds, decisionNeeded, decisionReason, decisionContextLine)
	result := runtimeExecApplyResult{
		Applied:     executed > 0,
		Status:      status,
		Message:     message,
		Executed:    executed,
		Failed:      failed,
		Total:       totalSteps,
		FirstReason: firstReason,
		FailedCmd:   firstFailedCmd,
		SuccessInfo: append([]string(nil), successInfo...),
		ExecIDs:     append([]string(nil), execIDs...),
	}
	appendTrackingExecution(runtime, fallbackCWD, conversationID, result)
	return result
}

func resolveRuntimeExecWorkspaceRoot(runtime agentRuntime, fallbackCWD string) string {
	root := strings.TrimSpace(fallbackCWD)
	if root != "" {
		return root
	}
	return strings.TrimSpace(runtime.cwd)
}

func loadLeadModeFromRuntimeMeta(workspaceRoot string) (bool, string, error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return false, "", nil
	}
	metaPath := filepath.Join(root, ".clawx", "runtime", "runtime_meta.json")
	body, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, metaPath, nil
		}
		return false, metaPath, err
	}
	var meta struct {
		LeadMode bool `json:"lead_mode"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		return false, metaPath, err
	}
	return meta.LeadMode, metaPath, nil
}

func scheduleRuntimeExecViaLead(
	runtime agentRuntime,
	plan runtimeExecPlan,
	workspaceRoot string,
	metaPath string,
	conversationID string,
) runtimeExecApplyResult {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return runtimeExecApplyResult{
			Applied: false,
			Status:  "failed",
			Message: "检测到 Lead 调度模式，但未解析到 workspace，未执行。",
		}
	}
	runtimeDir := filepath.Join(root, ".clawx", "runtime")
	queueFile := filepath.Join(runtimeDir, "tasks.jsonl")
	workerFile := filepath.Join(runtimeDir, "worker_states.json")
	heartbeatFile := filepath.Join(runtimeDir, "heartbeats.json")
	dispatchFile := filepath.Join(runtimeDir, "dispatch_state.json")

	queue := runtimeorchestrator.NewQueue(queueFile)
	registry := runtimeorchestrator.NewRegistry(workerFile, heartbeatFile)
	dispatcher := runtimeorchestrator.NewDispatcher(queue, registry, dispatchFile)

	enqueued := 0
	for _, step := range plan.Commands {
		cmdline := strings.TrimSpace(step.Cmd)
		if cmdline == "" {
			continue
		}
		taskCWD := strings.TrimSpace(step.CWD)
		if taskCWD == "" {
			taskCWD = root
		}
		resourceKey := strings.ToLower(strings.TrimSpace(taskCWD))
		payload := map[string]interface{}{
			"cmd":                                cmdline,
			"cwd":                                taskCWD,
			"reason":                             strings.TrimSpace(step.Reason),
			"fallback_source":                    strings.TrimSpace(step.FallbackSource),
			"runtime_exec_decision_mode":         strings.TrimSpace(plan.RuntimeExecDecisionMode),
			"runtime_exec_decision_apply_source": strings.TrimSpace(plan.RuntimeExecDecisionApplySource),
			"runtime_exec_decision_lock_source":  strings.TrimSpace(plan.RuntimeExecDecisionLockSource),
			"resource_key":                       resourceKey,
			"conversation":                       strings.TrimSpace(conversationID),
		}
		if _, err := queue.Enqueue(runtimeorchestrator.RuntimeTask{
			Source:  "runtime.exec",
			Intent:  "runtime.exec",
			Payload: payload,
			Status:  runtimeorchestrator.TaskQueued,
		}); err != nil {
			return runtimeExecApplyResult{
				Applied: false,
				Status:  "failed",
				Message: "Lead 调度入队失败：" + err.Error(),
			}
		}
		enqueued++
	}
	if enqueued == 0 {
		return runtimeExecApplyResult{
			Applied: false,
			Status:  "rejected",
			Message: "runtime.exec 缺少有效命令，未入队。",
		}
	}
	decisions, err := dispatcher.DispatchQueued(time.Now().UTC(), enqueued)
	if err != nil {
		return runtimeExecApplyResult{
			Applied: false,
			Status:  "failed",
			Message: "Lead 调度派工失败：" + err.Error(),
		}
	}
	lines := []string{
		"已进入 Lead 调度模式，本轮未直接执行 shell 命令。",
		fmt.Sprintf("结果：已入队 %d 条，已派发 %d 条。", enqueued, len(decisions)),
		buildRuntimeExecDecisionContextLine(plan),
		"证据：" + queueFile,
		nextStepLine(fmt.Sprintf("Worker 执行后我会继续汇总；如需查看详情，可查 `%s`。", dispatchFile)),
	}
	lines = trimRuntimeExecMessageLines(lines)
	if strings.TrimSpace(metaPath) != "" {
		lines = append(lines, "调度模式来源："+metaPath)
	}
	return runtimeExecApplyResult{
		Applied:  true,
		Status:   "applied",
		Message:  strings.Join(lines, "\n"),
		Executed: 0,
		Failed:   0,
		Total:    enqueued,
	}
}

func performRuntimeExecStep(
	ctx context.Context,
	runtime agentRuntime,
	conversationID string,
	execCWD string,
	cmdline string,
	planReason string,
	stepReason string,
	decisionMode string,
	decisionApplySource string,
	decisionLockSource string,
	fallbackSource string,
) (string, error, string) {
	cmdline = rewriteCommandForPreferredVenv(execCWD, cmdline)
	started := time.Now()
	out, runErr := runRuntimeExecCommand(ctx, cmdline, execCWD, 120*time.Second)
	if runErr == nil {
		if softErr := detectSoftExecutionFailure(cmdline, out); softErr != nil {
			runErr = softErr
		}
	}
	duration := time.Since(started).Milliseconds()
	execID := newRuntimeExecID()
	exitCode := 0
	if runErr != nil {
		exitCode = 1
	}
	decisionMode = strings.TrimSpace(decisionMode)
	decisionApplySource = strings.TrimSpace(decisionApplySource)
	decisionLockSource = strings.TrimSpace(decisionLockSource)
	if decisionMode != "" && decisionLockSource == "" {
		_, lockSource := resolveRuntimeExecDecisionLockState(conversationID)
		decisionLockSource = strings.TrimSpace(lockSource)
	}
	record := runtimeExecAttestationRecord{
		ExecID:                            execID,
		ConversationID:                    strings.TrimSpace(conversationID),
		AgentID:                           strings.TrimSpace(runtime.agentID),
		CWD:                               execCWD,
		Command:                           cmdline,
		PlanReason:                        strings.TrimSpace(planReason),
		StepReason:                        strings.TrimSpace(stepReason),
		RuntimeExecDecisionMode:           decisionMode,
		RuntimeExecDecisionSource:         decisionLockSource,
		RuntimeExecDecisionApplySource:    decisionApplySource,
		RuntimeExecDecisionLockSource:     decisionLockSource,
		RuntimeExecDecisionFallbackSource: strings.TrimSpace(fallbackSource),
		ExitCode:                          exitCode,
		DurationMS:                        duration,
		Success:                           runErr == nil,
		OutputDigest:                      digestOutput(out),
		OutputPreview:                     summarizeText(out, 180),
	}
	if runErr != nil {
		record.ErrorSummary = summarizeText(runErr.Error(), 180)
	}
	_ = appendRuntimeExecAttestation(record)
	return out, runErr, execID
}

func detectSoftExecutionFailure(cmdline, out string) error {
	lowerCmd := strings.ToLower(strings.TrimSpace(cmdline))
	lowerOut := strings.ToLower(strings.TrimSpace(out))
	if strings.Contains(lowerOut, "\"post ") && strings.Contains(lowerOut, "\" 500") {
		return fmt.Errorf("http request returned 500")
	}
	if !strings.Contains(lowerCmd, "curl ") {
		return nil
	}
	switch {
	case strings.Contains(lowerOut, "http/1.1 500"), strings.Contains(lowerOut, "http/2 500"), strings.Contains(lowerOut, " status=500"), strings.Contains(lowerOut, "\"status\":500"):
		return fmt.Errorf("http probe returned 500")
	case strings.Contains(lowerOut, "http/1.1 405"), strings.Contains(lowerOut, "http/2 405"), strings.Contains(lowerOut, " status=405"), strings.Contains(lowerOut, "\"status\":405"):
		return fmt.Errorf("http probe returned 405")
	case strings.Contains(lowerOut, "internal server error"):
		return fmt.Errorf("http probe returned 500 (internal server error)")
	default:
		return nil
	}
}

func rewriteCommandForPreferredVenv(execCWD, cmdline string) string {
	trimmed := strings.TrimSpace(cmdline)
	if trimmed == "" {
		return cmdline
	}
	if !strings.Contains(trimmed, "source .venv/bin/activate") {
		return cmdline
	}
	venv311Activate := filepath.Join(strings.TrimSpace(execCWD), ".venv311", "bin", "activate")
	if _, err := os.Stat(venv311Activate); err != nil {
		return cmdline
	}
	return strings.Replace(cmdline, "source .venv/bin/activate", "source .venv311/bin/activate", 1)
}

func tryAutoRepairLoop(
	ctx context.Context,
	runtime agentRuntime,
	conversationID string,
	fallbackCWD string,
	firstReason string,
	executed int,
	failed int,
	execIDs []string,
) (int, int, []string, string, int, bool, string) {
	reason := strings.ToLower(strings.TrimSpace(firstReason))
	if reason == "" {
		return executed, failed, execIDs, firstReason, 0, false, ""
	}
	execCWD := strings.TrimSpace(fallbackCWD)
	if execCWD == "" {
		execCWD = "."
	}
	if err := runtime.cfgSnapshot.ValidateWorkingDirectory(execCWD); err != nil {
		return executed, failed, execIDs, firstReason, 0, true, "需要调整可执行目录权限范围"
	}
	rounds := maxAutoRepairRounds()
	if rounds <= 0 {
		return executed, failed, execIDs, firstReason, 0, false, ""
	}
	repairClass := classifyAutoRepairClass(reason)
	if repairClass == "blocked" {
		return executed, failed, execIDs, firstReason, 0, true, "检测到高风险命令，我先暂停自动执行。你可以回复“替代命令: <命令>”继续，或回复“暂停”保持停止。"
	}
	if repairClass == "permission" {
		return executed, failed, execIDs, firstReason, 0, true, "需要你确认权限范围或 allowed roots"
	}
	if repairClass == "auth" {
		return executed, failed, execIDs, firstReason, 0, true, "需要你提供或确认凭据后再继续"
	}
	if tool, required := detectGenericVersionMismatch(reason); tool != "" {
		ok, detail := checkRuntimeVersionSatisfied(tool, required)
		if !ok {
			return executed, failed, execIDs, firstReason, 0, true, detail
		}
	}
	if missing := detectMissingRuntimeTool(reason); missing != "" {
		detail := "当前环境缺少运行时：" + missing + "。请先安装后我再继续自动修复。"
		return executed, failed, execIDs, firstReason, 0, true, detail
	}
	if repairClass == "build" {
		probeCmd := "source .venv/bin/activate && pip install -e . -vvv 2>&1 | grep -Ei 'requires a different python|requires-python' | tail -n 5 || true"
		out := ""
		if suppressed, suppressedCmd := shouldSuppressDuplicateRuntimeExec(conversationID, execCWD, probeCmd); suppressed {
			firstReason = "重复失败命令已抑制：" + suppressedCmd
		} else {
			probeOut, _, execID := performRuntimeExecStep(ctx, runtime, conversationID, execCWD, probeCmd, "runtime_auto_repair", "runtime_auto_repair_build_probe", "", "", "", "")
			out = probeOut
			execIDs = append(execIDs, execID)
		}
		if isPythonVersionMismatch(out) && !hasPython311InPath() {
			executed, failed, execIDs, firstReason, resolved := tryResolvePythonVersionMismatch(ctx, runtime, conversationID, execCWD, executed, failed, execIDs)
			if resolved {
				return executed, failed, execIDs, firstReason, 0, false, ""
			}
			return executed, failed, execIDs, firstReason, 0, true, "我已尝试自动安装/切换 python3.11 但失败，请你安装 python3.11（或将 requires-python 调整为 >=3.10）后继续。"
		}
	}
	if isPythonVersionMismatch(reason) {
		compatSteps := []string{
			"command -v python3.11 >/dev/null 2>&1 && python3.11 -m venv .venv311 && source .venv311/bin/activate && python -m pip install -U pip setuptools wheel && pip install -e . --no-build-isolation",
			"source .venv/bin/activate && pip install -e . --no-build-isolation",
		}
		for _, cmdline := range compatSteps {
			if suppressed, suppressedCmd := shouldSuppressDuplicateRuntimeExec(conversationID, execCWD, cmdline); suppressed {
				failed++
				firstReason = "重复失败命令已抑制：" + suppressedCmd
				continue
			}
			out, runErr, execID := performRuntimeExecStep(ctx, runtime, conversationID, execCWD, cmdline, "runtime_auto_repair", "runtime_auto_repair_python_compat", "", "", "", "")
			execIDs = append(execIDs, execID)
			if runErr != nil {
				failed++
				firstReason = "自动修复后仍失败：" + extractFailureEvidence(out, runErr.Error())
				continue
			}
			executed++
			return executed, failed, execIDs, "Python 版本兼容修复成功", 0, false, ""
		}
	}
	attemptedRounds := 0
	for round := 1; round <= rounds; round++ {
		beforeReason := firstReason
		steps := repairStepsForClass(repairClass, round)
		if len(steps) == 0 {
			break
		}
		for _, cmdline := range steps {
			if suppressed, suppressedCmd := shouldSuppressDuplicateRuntimeExec(conversationID, execCWD, cmdline); suppressed {
				failed++
				firstReason = "重复失败命令已抑制：" + suppressedCmd
				continue
			}
			stepReason := fmt.Sprintf("runtime_auto_repair_%s_round_%d", strings.TrimSpace(repairClass), round)
			out, runErr, execID := performRuntimeExecStep(ctx, runtime, conversationID, execCWD, cmdline, "runtime_auto_repair", stepReason, "", "", "", "")
			execIDs = append(execIDs, execID)
			if runErr != nil {
				failed++
				firstReason = "自动修复后仍失败：" + extractFailureEvidence(out, runErr.Error())
				continue
			}
			executed++
		}
		if shouldCountRepairRound(beforeReason, firstReason) {
			attemptedRounds++
		}
		if !strings.Contains(strings.ToLower(firstReason), "自动修复后仍失败") {
			break
		}
	}
	if attemptedRounds >= rounds && strings.Contains(strings.ToLower(firstReason), "自动修复后仍失败") {
		executed, failed, execIDs, firstReason = runPostLimitDiagnostics(ctx, runtime, conversationID, execCWD, repairClass, executed, failed, execIDs, firstReason)
		return executed, failed, execIDs, firstReason, attemptedRounds, false, ""
	}
	return executed, failed, execIDs, firstReason, attemptedRounds, false, ""
}

func runPostLimitDiagnostics(
	ctx context.Context,
	runtime agentRuntime,
	conversationID string,
	execCWD string,
	repairClass string,
	executed int,
	failed int,
	execIDs []string,
	firstReason string,
) (int, int, []string, string) {
	commands := make([]string, 0, 2)
	switch repairClass {
	case "build":
		commands = append(commands,
			"source .venv/bin/activate && pip install -e . -vvv 2>&1 | grep -Ei 'error|failed|exception|traceback|no matching|module' | tail -n 60",
			"test -f pyproject.toml && sed -n '1,200p' pyproject.toml || echo 'pyproject.toml not found'",
		)
	case "network":
		commands = append(commands,
			"getent hosts pypi.org || true && curl -I --max-time 8 https://pypi.org/simple || true",
		)
	default:
		commands = append(commands,
			"source .venv/bin/activate && pip --version",
		)
	}
	for _, cmdline := range commands {
		if suppressed, suppressedCmd := shouldSuppressDuplicateRuntimeExec(conversationID, execCWD, cmdline); suppressed {
			failed++
			firstReason = "深度诊断命令重复失败，已抑制：" + suppressedCmd
			continue
		}
		stepReason := "runtime_auto_repair_post_limit"
		if strings.TrimSpace(repairClass) != "" {
			stepReason = "runtime_auto_repair_post_limit_" + strings.TrimSpace(repairClass)
		}
		out, runErr, execID := performRuntimeExecStep(ctx, runtime, conversationID, execCWD, cmdline, "runtime_auto_repair", stepReason, "", "", "", "")
		execIDs = append(execIDs, execID)
		if runErr != nil {
			failed++
			if maybeFailureSummary(out) != "" {
				firstReason = maybeFailureSummary(out)
			} else {
				firstReason = "深度诊断仍失败：" + summarizeText(out, 120)
			}
			continue
		}
		executed++
		if summary := maybeFailureSummary(out); summary != "" {
			firstReason = summary
		}
	}
	return executed, failed, execIDs, firstReason
}

func renderRuntimeExecUserMessage(status string, total, executed, failed int, firstReason, failedCmd string, successInfo []string, failureEvidence []string, logPath, execIDLine string, autoRepairRounds int, decisionNeeded bool, decisionReason string, decisionContextLine string) string {
	firstReason = strings.TrimSpace(firstReason)
	if firstReason == "" {
		firstReason = "无"
	}
	decisionReason = strings.TrimSpace(decisionReason)
	decisionContextLine = strings.TrimSpace(decisionContextLine)
	execProof := renderExecIDsForUser(execIDLine)
	problem := describeRuntimeExecProblem(firstReason)
	recoveryStrategy := runtimeExecRecoveryStrategy(firstReason, autoRepairRounds, decisionNeeded)
	decisionOptions := runtimeExecDecisionOptions(firstReason, decisionNeeded)
	decisionBackground := summarizeRuntimeExecDecisionReason(decisionReason)
	keyEvidence := buildFailureEvidenceLine(firstReason, failureEvidence)
	nextStep := suggestRuntimeExecNextStep(firstReason, autoRepairRounds, decisionNeeded, decisionReason)
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "applied":
		if total == 1 && len(successInfo) <= 1 {
			lines := []string{
				"结论：已执行完成。",
				"完成状态：已完成。",
			}
			if len(successInfo) == 1 && strings.TrimSpace(successInfo[0]) != "" {
				lines = append(lines, "产物："+strings.TrimSpace(successInfo[0]))
			}
			lines = append(lines,
				decisionContextLine,
				"证据：" + execProof,
			)
			lines = trimRuntimeExecMessageLines(lines)
			if next := suggestSuccessNextStep(successInfo); next != "" {
				lines = append(lines, nextStepLine(next))
			}
			return strings.Join(lines, "\n")
		}
		lines := []string{
			fmt.Sprintf("结论：已完成，执行 %d 步，全部成功。", total),
			"完成状态：已完成。",
		}
		if len(successInfo) > 0 {
			lines = append(lines, "", "产物：")
			for _, item := range successInfo {
				item = strings.TrimSpace(item)
				if item == "" {
					continue
				}
				lines = append(lines, "- "+item)
			}
		}
		if next := suggestSuccessNextStep(successInfo); next != "" {
			lines = append(lines, "", nextStepLine(next))
		}
		if decisionContextLine != "" {
			lines = append(lines, "", decisionContextLine)
		}
		lines = append(lines, "", "证据："+execProof)
		return strings.Join(lines, "\n")
	case "partial":
		cmdLine := ""
		if strings.TrimSpace(failedCmd) != "" {
			cmdLine = "失败命令：" + strings.TrimSpace(failedCmd)
		}
		lines := []string{
			fmt.Sprintf("结论：已执行 %d 步，成功 %d 步、失败 %d 步。", total, executed, failed),
			"完成状态：部分失败。",
			"",
			"问题：" + problem,
			decisionContextLine,
		}
		lines = trimRuntimeExecMessageLines(lines)
		if recoveryStrategy != "" {
			lines = append(lines, "恢复策略："+recoveryStrategy)
		}
		if decisionBackground != "" {
			lines = append(lines, "决策背景："+decisionBackground)
		}
		if decisionOptions != "" {
			lines = append(lines, decisionOptions)
		}
		if cmdLine != "" {
			lines = append(lines, cmdLine)
		}
		lines = append(lines,
			"关键证据："+keyEvidence,
			"",
			nextStepLine(nextStep),
			"",
			"证据："+"证明文件="+logPath+"；"+execProof,
		)
		return strings.Join(lines, "\n")
	default:
		cmdLine := ""
		if strings.TrimSpace(failedCmd) != "" {
			cmdLine = "失败命令：" + strings.TrimSpace(failedCmd)
		}
		lines := []string{
			fmt.Sprintf("结论：尝试了 %d 步命令，但都失败了。", total),
			"完成状态：失败。",
			"",
			"问题：" + problem,
			decisionContextLine,
		}
		lines = trimRuntimeExecMessageLines(lines)
		if recoveryStrategy != "" {
			lines = append(lines, "恢复策略："+recoveryStrategy)
		}
		if decisionBackground != "" {
			lines = append(lines, "决策背景："+decisionBackground)
		}
		if decisionOptions != "" {
			lines = append(lines, decisionOptions)
		}
		if cmdLine != "" {
			lines = append(lines, cmdLine)
		}
		lines = append(lines,
			"关键证据："+keyEvidence,
			"",
			nextStepLine(nextStep),
			"",
			"证据："+"证明文件="+logPath+"；"+execProof,
		)
		return strings.Join(lines, "\n")
	}
}

func buildRuntimeExecDecisionContextLine(plan runtimeExecPlan) string {
	mode := normalizeRuntimeExecDecisionMode(plan.RuntimeExecDecisionMode)
	applySource := strings.TrimSpace(plan.RuntimeExecDecisionApplySource)
	lockSource := strings.TrimSpace(plan.RuntimeExecDecisionLockSource)
	fallbackSources := collectRuntimeExecFallbackSources(plan.Commands)

	if mode == "" && applySource == "" && lockSource == "" && len(fallbackSources) == 0 {
		return ""
	}
	if mode == "" {
		mode = "none"
	}
	if applySource == "" {
		applySource = "none"
	}
	if lockSource == "" {
		lockSource = "none"
	}
	line := fmt.Sprintf("执行来源：mode=%s apply_source=%s lock_source=%s", mode, applySource, lockSource)
	if len(fallbackSources) > 0 {
		line += " fallback_source=" + strings.Join(fallbackSources, ",")
	}
	return line
}

func collectRuntimeExecFallbackSources(commands []runtimeExecCommand) []string {
	if len(commands) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(commands))
	out := make([]string, 0, len(commands))
	for _, cmd := range commands {
		source := strings.TrimSpace(strings.ToLower(cmd.FallbackSource))
		if source == "" {
			continue
		}
		if _, ok := seen[source]; ok {
			continue
		}
		seen[source] = struct{}{}
		out = append(out, source)
	}
	sort.Strings(out)
	return out
}

func trimRuntimeExecMessageLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(out) > 0 && out[len(out)-1] == "" {
				continue
			}
			out = append(out, "")
			continue
		}
		out = append(out, line)
	}
	for len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

func runtimeExecRecoveryStrategy(firstReason string, autoRepairRounds int, decisionNeeded bool) string {
	reason := strings.ToLower(strings.TrimSpace(firstReason))
	cls := classifyAutoRepairClass(reason)
	if decisionNeeded {
		switch cls {
		case "blocked":
			return "检测到高风险命令，已停止自治执行，等待你提供替代命令。"
		case "permission":
			return "当前被权限或 allowed roots 限制，我先暂停重试，等你调整后继续。"
		case "auth":
			return "当前缺少有效凭据，我先暂停重试，等你补齐后继续。"
		case "service":
			return "服务类故障已进入人工决策。你可以回复“继续深修”或“仅重试”。"
		case "build":
			return "构建类故障已进入人工决策。你可以回复“继续构建修复”或“切换依赖源”。"
		default:
			return "需要你确认下一步策略后我再继续执行。"
		}
	}
	if autoRepairRounds > 0 {
		switch cls {
		case "service", "build", "network":
			return fmt.Sprintf("已执行 %d 轮自治修复，将继续按同类问题策略重试。", autoRepairRounds)
		default:
			return fmt.Sprintf("已执行 %d 轮自治修复，下一步将先补齐诊断证据再重试。", autoRepairRounds)
		}
	}
	switch cls {
	case "service":
		return "服务类故障，先补日志证据再定向修复。"
	case "build":
		return "构建类故障，先修复依赖/构建参数后重试。"
	case "network":
		return "网络类故障，先切换源并重试连通性。"
	default:
		return "先补齐诊断证据后再执行下一轮。"
	}
}

func runtimeExecDecisionOptions(firstReason string, decisionNeeded bool) string {
	if !decisionNeeded {
		return ""
	}
	cls := classifyAutoRepairClass(strings.ToLower(strings.TrimSpace(firstReason)))
	switch cls {
	case "service":
		return "你可以直接回复：继续深修 / 仅重试 / 暂停。"
	case "build":
		return "你可以直接回复：继续构建修复 / 切换依赖源 / 暂停。"
	case "permission", "network", "auth":
		return "你可以直接回复：继续 / 暂停。"
	case "blocked":
		return "你可以直接回复：替代命令: <命令> / 暂停。"
	default:
		return "你可以直接回复：继续 / 暂停。"
	}
}

func summarizeRuntimeExecDecisionReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	cutMarkers := []string{
		"你可以直接回复：",
		"你可以回复",
		"也可以直接回复",
		"决策选项：",
		"最小决策：",
	}
	for _, marker := range cutMarkers {
		if idx := strings.Index(reason, marker); idx > 0 {
			reason = strings.TrimSpace(reason[:idx])
			break
		}
	}
	return summarizeText(reason, 220)
}

func summarizeSuccessEvidence(step runtimeExecCommand, execCWD string, out string) string {
	cmd := strings.TrimSpace(step.Cmd)
	reason := strings.TrimSpace(step.Reason)
	if artifacts := detectCommandArtifactPaths(cmd, execCWD); len(artifacts) > 0 {
		if len(artifacts) == 1 {
			return "已生成文件 " + artifacts[0]
		}
		return "已更新文件 " + strings.Join(artifacts, ", ")
	}
	if looksLikeProbeCommand(cmd) {
		if code := extractHTTPStatus(out); code != "" {
			return "探针返回 HTTP " + code
		}
	}
	if reason != "" {
		return reason + "（已完成）"
	}
	signal := summarizeTailText(out, 2, 120)
	if signal != "" && signal != "(no output)" {
		return signal
	}
	if intent := inferCommandIntent(cmd); intent != "" {
		return intent + "（已完成）"
	}
	return "命令执行成功"
}

func inferCommandIntent(cmdline string) string {
	lower := strings.ToLower(strings.TrimSpace(cmdline))
	switch {
	case strings.Contains(lower, "pip install"):
		return "依赖安装"
	case strings.Contains(lower, "uvicorn"):
		return "服务启动/探针检查"
	case strings.Contains(lower, "alembic upgrade"):
		return "数据库迁移"
	case strings.Contains(lower, "mkdir -p"), strings.Contains(lower, "cat >"), strings.Contains(lower, "cat >>"):
		return "目录/文件更新"
	case strings.Contains(lower, "curl "):
		return "接口请求"
	default:
		return ""
	}
}

func detectCommandArtifactPaths(cmdline string, execCWD string) []string {
	trimmed := strings.TrimSpace(cmdline)
	if trimmed == "" {
		return nil
	}

	// Capture common file-write forms.
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bcat\s*>>?\s*([^\s<]+)`),
		regexp.MustCompile(`(?i)\btee\s+([^\s|]+)`),
		regexp.MustCompile(`(?i)\btouch\s+([^\s]+)`),
		regexp.MustCompile(`>>?\s*([^\s]+)`),
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, 2)
	for _, re := range patterns {
		matches := re.FindAllStringSubmatch(trimmed, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			path := normalizeArtifactPath(m[1], execCWD)
			if path == "" {
				continue
			}
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			result = append(result, path)
		}
	}
	return result
}

func normalizeArtifactPath(raw string, execCWD string) string {
	token := strings.Trim(strings.TrimSpace(raw), `"'`)
	if token == "" {
		return ""
	}
	if strings.HasPrefix(token, "&") || strings.HasPrefix(token, "2>&") {
		return ""
	}
	if strings.HasPrefix(token, "/") {
		return filepath.Clean(token)
	}
	cwd := strings.TrimSpace(execCWD)
	if cwd == "" {
		return filepath.Clean(token)
	}
	return filepath.Clean(filepath.Join(cwd, token))
}

func suggestSuccessNextStep(successInfo []string) string {
	for _, item := range successInfo {
		line := strings.ToLower(strings.TrimSpace(item))
		if line == "" {
			continue
		}
		if strings.Contains(line, "/docs/plan.md") || strings.Contains(line, "spec_") {
			return "我可以继续把文档拆成可执行任务并同步到 TASK_PLAN.md/TASK_EXECUTION.md。"
		}
	}
	return "如果要我继续，回复“继续按计划执行”即可。"
}

func looksLikeProbeCommand(cmdline string) bool {
	lower := strings.ToLower(strings.TrimSpace(cmdline))
	return strings.Contains(lower, "/health") ||
		strings.Contains(lower, "healthz") ||
		strings.Contains(lower, "curl -i") ||
		strings.Contains(lower, "curl -s") ||
		strings.Contains(lower, "curl ")
}

func extractHTTPStatus(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "http/1.1 ") || strings.HasPrefix(lower, "http/2 ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

func describeRuntimeExecProblem(firstReason string) string {
	reason := strings.ToLower(strings.TrimSpace(firstReason))
	switch {
	case strings.Contains(reason, "curl: (22)"), strings.Contains(reason, "returned error: 405"):
		return "HTTP 请求方法或路由不匹配（例如把 POST 接口用 GET 调用）。"
	case strings.Contains(reason, "returned error: 500"):
		return "目标服务返回 500，服务已响应但内部处理失败。"
	case strings.Contains(reason, "internal server error"), strings.Contains(reason, "http probe returned 500"):
		return "本地服务已启动，但应用内部报错导致接口返回 500。"
	case strings.Contains(reason, "exception in asgi application"), strings.Contains(reason, "asgi application"):
		return "服务进程已拉起，但 ASGI 应用内部抛出异常，导致请求处理失败。"
	case strings.Contains(reason, "build dependencies"), strings.Contains(reason, "build backend"), strings.Contains(reason, "build_editable"):
		return "pip 在可编辑安装阶段失败，卡在构建依赖/构建后端检查。"
	case strings.Contains(reason, "no such host"), strings.Contains(reason, "name or service not known"), strings.Contains(reason, "temporary failure"), strings.Contains(reason, "network"), strings.Contains(reason, "timeout"):
		return "网络或 DNS 访问异常，导致依赖下载阶段失败。"
	case strings.Contains(reason, "permission"), strings.Contains(reason, "outside allowed roots"):
		return "权限或工作目录范围受限，命令无法按当前路径执行。"
	default:
		return firstReason
	}
}

func suggestRuntimeExecNextStep(firstReason string, autoRepairRounds int, decisionNeeded bool, decisionReason string) string {
	reason := strings.ToLower(strings.TrimSpace(firstReason))
	cls := classifyAutoRepairClass(reason)
	if decisionNeeded {
		if strings.TrimSpace(decisionReason) != "" {
			decisionLower := strings.ToLower(strings.TrimSpace(decisionReason))
			if strings.Contains(decisionLower, "高风险命令") || strings.Contains(decisionLower, "替代命令") {
				return decisionReason
			}
			return "请从上面的选项里回复一个指令，我会立即继续执行。"
		}
		if strings.Contains(reason, "dangerous command") || strings.Contains(reason, "高风险命令") {
			return "检测到高风险命令，请先回复“替代命令: <命令>”或“暂停”。"
		}
		return "当前需要你做决策后我才能继续。"
	}
	if isPythonVersionMismatch(reason) {
		return "我会优先尝试匹配可用 Python 版本（例如 python3.11）并重建虚拟环境后继续安装。"
	}
	if autoRepairRounds > 0 {
		switch cls {
		case "service":
			return "已执行服务异常自动修复链（日志诊断、迁移/修复尝试、重启与健康回归）。我会继续沿最新 traceback 自动修复，除非触发权限/凭据决策。"
		case "build":
			return "我会继续按构建错误关键词做定向修复（补齐缺失依赖/调整构建参数）并再次执行安装。"
		case "network":
			return "我会继续做连通性切换与重试（官方源 + 镜像源）；如果都不可达，再只向你确认网络决策。"
		default:
			return "我会先补齐诊断证据（失败命令完整输出、关键日志、退出点），再执行定向修复，不会停在泛化重试。"
		}
	}
	switch {
	case strings.Contains(reason, "exception in asgi application"), strings.Contains(reason, "asgi application"):
		return "我会先抓取完整 traceback（uvicorn/app 日志）并定位异常模块，再按异常点自动修复并重启回归。"
	case strings.Contains(reason, "returned error: 405"):
		return "我会先校对接口方法与路径（GET/POST、query/body），修正后重试该请求。"
	case strings.Contains(reason, "returned error: 500"):
		return "我会先定位服务端错误日志与请求参数，再修复后重试同一请求。"
	case strings.Contains(reason, "internal server error"), strings.Contains(reason, "http probe returned 500"):
		return "我会先抓取 uvicorn/应用 traceback 和最近请求参数，定位 500 根因后再自动修复并回归探针。"
	case strings.Contains(reason, "build dependencies"), strings.Contains(reason, "build backend"), strings.Contains(reason, "build_editable"):
		return "我将先升级构建工具（pip/setuptools/wheel），再用 --no-build-isolation 重试 editable 安装；若仍失败，提取最后 120 行构建日志定位具体依赖。"
	case strings.Contains(reason, "no such host"), strings.Contains(reason, "name or service not known"), strings.Contains(reason, "temporary failure"), strings.Contains(reason, "network"), strings.Contains(reason, "timeout"):
		return "我将先做 DNS/连通性检查，再按官方源→镜像源回退重试；若都失败，再向你确认是否切换内网源或离线包。"
	case strings.Contains(reason, "permission"), strings.Contains(reason, "outside allowed roots"):
		return "我将先切回允许目录并重试；若仍受限，再向你确认是否调整权限范围。"
	default:
		return "我将先抓取完整错误上下文并执行一轮定向修复；若仍失败，再把最小决策项发给你。"
	}
}

func maybeFailureSummary(out string) string {
	lines := strings.Split(out, "\n")
	keywords := []string{"error", "failed", "exception", "traceback", "no matching distribution", "module not found", "build backend"}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				return summarizeText(trimmed, 140)
			}
		}
	}
	return ""
}

func extractReasonEvidenceLine(firstReason string) string {
	line := strings.TrimSpace(firstReason)
	if line == "" || strings.EqualFold(line, "无") {
		return "无"
	}
	if idx := strings.Index(line, "："); idx >= 0 && idx+1 < len(line) {
		line = strings.TrimSpace(line[idx+1:])
	}
	return summarizeText(line, 140)
}

func buildFailureEvidenceLine(firstReason string, failureEvidence []string) string {
	items := make([]string, 0, len(failureEvidence)+1)
	seen := make(map[string]struct{}, len(failureEvidence)+1)
	addItem := func(raw string) {
		item := strings.TrimSpace(raw)
		if item == "" || strings.EqualFold(item, "无") {
			return
		}
		key := failureEvidenceKey(item)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		items = append(items, item)
	}

	addItem(extractReasonEvidenceLine(firstReason))
	for _, evidence := range failureEvidence {
		addItem(summarizeText(evidence, 140))
	}
	if len(items) == 0 {
		return "无"
	}
	return strings.Join(items, "；")
}

func collectUniqueFailureEvidence(execIDs []string, limit int) []string {
	if limit <= 0 {
		limit = 3
	}
	records := listRuntimeExecAttestationsByIDs(execIDs)
	if len(records) == 0 {
		return nil
	}
	out := make([]string, 0, limit)
	seen := make(map[string]struct{}, limit*2)
	for _, record := range records {
		if record.Success {
			continue
		}
		evidence := strings.TrimSpace(record.ErrorSummary)
		if evidence == "" {
			evidence = strings.TrimSpace(record.OutputPreview)
		}
		if evidence == "" {
			evidence = strings.TrimSpace(record.OutputDigest)
		}
		evidence = summarizeText(evidence, 140)
		key := failureEvidenceKey(evidence + "|" + strings.TrimSpace(record.OutputDigest))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, evidence)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func shouldSuppressDuplicateRuntimeExec(conversationID, execCWD, cmdline string) (bool, string) {
	if isEnvTrue("CLAWX_RUNTIME_EXEC_DEDUPE_DISABLE") {
		return false, ""
	}
	conversationID = strings.TrimSpace(conversationID)
	signature := runtimeExecCommandSignature(execCWD, cmdline)
	if conversationID == "" || signature == "" {
		return false, ""
	}
	recent := listRecentRuntimeExecAttestations(conversationID, 12)
	if len(recent) < 2 {
		return false, ""
	}
	latest := recent[len(recent)-1]
	if runtimeExecCommandSignature(latest.CWD, latest.Command) != signature {
		return false, ""
	}
	if latest.Success {
		return false, ""
	}

	matchedFailures := make([]runtimeExecAttestationRecord, 0, 4)
	for i := len(recent) - 1; i >= 0; i-- {
		record := recent[i]
		if runtimeExecCommandSignature(record.CWD, record.Command) != signature {
			continue
		}
		if record.Success {
			return false, ""
		}
		matchedFailures = append(matchedFailures, record)
		if len(matchedFailures) >= 4 {
			break
		}
	}
	if len(matchedFailures) < 2 {
		return false, ""
	}

	evidenceMarkers := make(map[string]struct{}, len(matchedFailures))
	for _, record := range matchedFailures {
		marker := strings.TrimSpace(record.OutputDigest)
		if marker == "" {
			marker = failureEvidenceKey(strings.TrimSpace(record.ErrorSummary) + "|" + strings.TrimSpace(record.OutputPreview))
		}
		if marker == "" {
			marker = failureSignature(strings.TrimSpace(record.ErrorSummary) + " " + strings.TrimSpace(record.OutputPreview))
		}
		if marker == "" {
			continue
		}
		evidenceMarkers[marker] = struct{}{}
	}
	if len(evidenceMarkers) <= 1 || len(matchedFailures) >= 3 {
		return true, summarizeCommand(cmdline)
	}
	return false, ""
}

func runtimeExecCommandSignature(execCWD, cmdline string) string {
	cwd := strings.TrimSpace(execCWD)
	if cwd != "" {
		cwd = filepath.Clean(cwd)
	}
	command := normalizeCommandWhitespace(strings.ToLower(strings.TrimSpace(cmdline)))
	if command == "" {
		return ""
	}
	return cwd + "|" + command
}

func normalizeCommandWhitespace(text string) string {
	parts := strings.Fields(strings.TrimSpace(text))
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

func failureEvidenceKey(text string) string {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return ""
	}
	return normalizeCommandWhitespace(normalized)
}

func extractFailureEvidence(out string, errText string) string {
	candidates := []string{
		"requires a different python",
		"requires-python",
		"not in '>=",
		"could not resolve host",
		"name or service not known",
		"temporary failure in name resolution",
		"permission denied",
		"outside allowed roots",
		"no matching distribution found",
		"module not found",
		"command not found",
		"error:",
		"failed",
		"exception",
		"traceback",
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		for _, kw := range candidates {
			if strings.Contains(lower, kw) {
				return summarizeText(line, 180)
			}
		}
	}
	if tail := summarizeTailText(out, 4, 180); tail != "" {
		return tail
	}
	return summarizeText(strings.TrimSpace(errText), 180)
}

func summarizeTailText(text string, lineCount int, limit int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	nonEmpty := make([]string, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	if lineCount <= 0 || lineCount > len(nonEmpty) {
		lineCount = len(nonEmpty)
	}
	chosen := nonEmpty[len(nonEmpty)-lineCount:]
	return summarizeText(strings.Join(chosen, " | "), limit)
}

func shouldCountRepairRound(beforeReason, afterReason string) bool {
	before := strings.ToLower(strings.TrimSpace(beforeReason))
	after := strings.ToLower(strings.TrimSpace(afterReason))
	if isPythonVersionMismatch(before) || isPythonVersionMismatch(after) {
		return false
	}
	if after == "" {
		return false
	}
	if strings.Contains(after, "permission") || strings.Contains(after, "outside allowed roots") || strings.Contains(after, "unauthorized") {
		return true
	}
	return failureSignature(before) == failureSignature(after)
}

func augmentDecisionFromFailureHistory(conversationID, firstReason string, decisionNeeded bool, decisionReason string) (bool, string) {
	if decisionNeeded {
		return decisionNeeded, decisionReason
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return decisionNeeded, decisionReason
	}
	signature := failureSignature(firstReason)
	if signature == "" || signature == "无" {
		globalRuntimeExecFailureTracker.clear(conversationID)
		return decisionNeeded, decisionReason
	}
	count := globalRuntimeExecFailureTracker.record(conversationID, signature)
	cls := classifyAutoRepairClass(strings.ToLower(strings.TrimSpace(firstReason)))
	threshold := runtimeExecEscalationThreshold(cls)
	if count < threshold {
		return decisionNeeded, decisionReason
	}
	if reason := runtimeExecEscalationReason(cls, count); strings.TrimSpace(reason) != "" {
		return true, reason
	}
	return decisionNeeded, decisionReason
}

func runtimeExecEscalationThreshold(class string) int {
	switch strings.TrimSpace(strings.ToLower(class)) {
	case "permission", "network", "auth":
		return 3
	case "service", "build":
		return 4
	default:
		return 5
	}
}

func runtimeExecEscalationReason(class string, count int) string {
	switch strings.TrimSpace(strings.ToLower(class)) {
	case "permission":
		return fmt.Sprintf("同一权限问题连续出现（%d 次）。请先调整目录权限或 allowed roots。处理后回复“继续”，我会从当前进度接着执行。", count)
	case "network":
		return fmt.Sprintf("同一网络问题连续出现（%d 次）。请先确认当前机器到依赖源可达。处理后回复“继续”，我会从当前进度接着执行。", count)
	case "auth":
		return fmt.Sprintf("同一鉴权问题连续出现（%d 次）。请先更新凭据或登录态。处理后回复“继续”，我会从当前进度接着执行。", count)
	case "service":
		return fmt.Sprintf("同一服务类问题连续出现（%d 次）。我已完成日志诊断与重启修复。你可以回复“继续深修”（允许迁移/数据修复）、“仅重试”（只做重启+健康检查）或“暂停”。", count)
	case "build":
		return fmt.Sprintf("同一构建类问题连续出现（%d 次）。我已完成多轮构建链修复。你可以回复“继续构建修复”（继续自动修复）、“切换依赖源”（镜像/离线包路径）或“暂停”。", count)
	default:
		return fmt.Sprintf("同类问题连续出现（%d 次）。请确认下一步策略；也可以直接回复“继续”，我会从当前进度接着执行。", count)
	}
}

func (t *runtimeExecFailureTracker) record(conversationID, signature string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	state, ok := t.byConversation[conversationID]
	if !ok || state.signature != signature || now.Sub(state.updatedAt) > 30*time.Minute {
		t.byConversation[conversationID] = runtimeExecFailureState{
			signature: signature,
			count:     1,
			updatedAt: now,
		}
		return 1
	}
	state.count++
	state.updatedAt = now
	t.byConversation[conversationID] = state
	return state.count
}

func (t *runtimeExecFailureTracker) clear(conversationID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.byConversation, conversationID)
}

func failureSignature(reason string) string {
	reason = strings.ToLower(strings.TrimSpace(reason))
	reason = strings.ReplaceAll(reason, "自动修复后仍失败：", "")
	reason = strings.ReplaceAll(reason, "深度诊断仍失败：", "")
	reason = strings.ReplaceAll(reason, "深度诊断定位：", "")
	return summarizeText(reason, 80)
}

func isPythonVersionMismatch(reason string) bool {
	reason = strings.ToLower(strings.TrimSpace(reason))
	return strings.Contains(reason, "requires a different python") ||
		strings.Contains(reason, "not in '>=") ||
		strings.Contains(reason, "requires-python") ||
		strings.Contains(reason, "python 3.10") ||
		strings.Contains(reason, "python 3.11")
}

func hasPython311InPath() bool {
	_, err := exec.LookPath("python3.11")
	return err == nil
}

func tryResolvePythonVersionMismatch(
	ctx context.Context,
	runtime agentRuntime,
	conversationID string,
	execCWD string,
	executed int,
	failed int,
	execIDs []string,
) (int, int, []string, string, bool) {
	steps := []string{
		"command -v python3.11 >/dev/null 2>&1 && python3.11 --version",
		"command -v apt-get >/dev/null 2>&1 && command -v sudo >/dev/null 2>&1 && sudo -n apt-get update && sudo -n apt-get install -y python3.11 python3.11-venv",
		"command -v python3.11 >/dev/null 2>&1 && python3.11 -m venv .venv311 && source .venv311/bin/activate && python -m pip install -U pip setuptools wheel && pip install -e . --no-build-isolation",
	}
	lastReason := "项目要求 Python >=3.11，但当前环境只有 Python 3.10"
	for idx, cmdline := range steps {
		if suppressed, suppressedCmd := shouldSuppressDuplicateRuntimeExec(conversationID, execCWD, cmdline); suppressed {
			failed++
			lastReason = fmt.Sprintf("Python 版本修复步骤 %d 被抑制：重复失败命令 %s", idx+1, suppressedCmd)
			continue
		}
		stepReason := fmt.Sprintf("runtime_auto_repair_python_mismatch_step_%d", idx+1)
		out, runErr, execID := performRuntimeExecStep(ctx, runtime, conversationID, execCWD, cmdline, "runtime_auto_repair", stepReason, "", "", "", "")
		execIDs = append(execIDs, execID)
		if runErr != nil {
			failed++
			lastReason = fmt.Sprintf("Python 版本修复步骤 %d 失败：%s", idx+1, summarizeText(out, 120))
			continue
		}
		executed++
		if idx == 0 {
			// python3.11 already present, continue to venv/install step.
			continue
		}
		if idx == 1 {
			// install step passed, let next step create venv + install.
			continue
		}
		return executed, failed, execIDs, "Python 版本兼容修复成功", true
	}
	return executed, failed, execIDs, lastReason, false
}

func detectGenericVersionMismatch(reason string) (tool string, required string) {
	lower := strings.ToLower(strings.TrimSpace(reason))
	if strings.Contains(lower, "requires a different python") || strings.Contains(lower, "requires-python") {
		return "python", extractRequiredVersion(lower)
	}
	if strings.Contains(lower, "requires go") || strings.Contains(lower, "go.mod") {
		return "go", extractRequiredVersion(lower)
	}
	if strings.Contains(lower, "unsupportedclassversionerror") || strings.Contains(lower, "class file version") || strings.Contains(lower, "java version") {
		return "java", extractRequiredVersion(lower)
	}
	if strings.Contains(lower, "ebadengine") || strings.Contains(lower, "unsupported engine") || strings.Contains(lower, "node version") {
		return "node", extractRequiredVersion(lower)
	}
	return "", ""
}

func detectMissingRuntimeTool(reason string) string {
	lower := strings.ToLower(strings.TrimSpace(reason))
	switch {
	case strings.Contains(lower, "python: command not found"), strings.Contains(lower, "python3: command not found"):
		return "python"
	case strings.Contains(lower, "go: command not found"):
		return "go"
	case strings.Contains(lower, "java: command not found"):
		return "java"
	case strings.Contains(lower, "node: command not found"), strings.Contains(lower, "npm: command not found"):
		return "node"
	default:
		return ""
	}
}

func extractRequiredVersion(reason string) string {
	re := regexp.MustCompile(`>=\s*([0-9]+\.[0-9]+)`)
	if m := re.FindStringSubmatch(reason); len(m) >= 2 {
		return m[1]
	}
	re2 := regexp.MustCompile(`([0-9]+\.[0-9]+)`)
	if m := re2.FindStringSubmatch(reason); len(m) >= 2 {
		return m[1]
	}
	return ""
}

func checkRuntimeVersionSatisfied(tool string, required string) (bool, string) {
	tool = strings.TrimSpace(strings.ToLower(tool))
	switch tool {
	case "python":
		if hasPython311InPath() {
			return true, ""
		}
		return false, "项目运行时与当前 Python 版本不兼容。请安装 python3.11（或调整项目 requires-python）后我再继续。"
	case "go":
		_, err := exec.LookPath("go")
		if err != nil {
			return false, "当前环境缺少 Go 运行时。请安装 Go 后我再继续。"
		}
		return false, "项目要求更高 Go 版本。请升级本机 Go（或调整 go.mod 版本约束）后我再继续。"
	case "java":
		_, err := exec.LookPath("java")
		if err != nil {
			return false, "当前环境缺少 Java 运行时。请安装对应 JDK 后我再继续。"
		}
		return false, "项目要求的 Java 版本与当前环境不匹配。请切换/升级 JDK 后我再继续。"
	case "node":
		_, err := exec.LookPath("node")
		if err != nil {
			return false, "当前环境缺少 Node.js 运行时。请安装 Node.js 后我再继续。"
		}
		return false, "项目要求的 Node.js 版本与当前环境不匹配。请切换 Node 版本后我再继续。"
	default:
		_ = required
		return true, ""
	}
}

func classifyAutoRepairClass(reason string) string {
	switch {
	case strings.Contains(reason, "命令被拒绝"), strings.Contains(reason, "dangerous command pattern"):
		return "blocked"
	case strings.Contains(reason, "permission"), strings.Contains(reason, "outside allowed roots"), strings.Contains(reason, "not allowed"):
		return "permission"
	case strings.Contains(reason, "auth"), strings.Contains(reason, "unauthorized"), strings.Contains(reason, "forbidden"), strings.Contains(reason, "token"):
		return "auth"
	case strings.Contains(reason, "build dependen"), strings.Contains(reason, "build backend"), strings.Contains(reason, "build_editable"):
		return "build"
	case strings.Contains(reason, "internal server error"),
		strings.Contains(reason, "http probe returned 500"),
		strings.Contains(reason, "http request returned 500"),
		strings.Contains(reason, "returned error: 500"),
		strings.Contains(reason, "exception in asgi application"),
		strings.Contains(reason, "asgi application"),
		strings.Contains(reason, "uvicorn running on"),
		strings.Contains(reason, "\"post "),
		strings.Contains(reason, "\"get "),
		strings.Contains(reason, "sqlalchemy.exc"),
		strings.Contains(reason, "sqlite3.operationalerror"):
		return "service"
	case strings.Contains(reason, "no such host"), strings.Contains(reason, "name or service not known"), strings.Contains(reason, "network"), strings.Contains(reason, "timeout"):
		return "network"
	default:
		return "unknown"
	}
}

func repairStepsForClass(class string, round int) []string {
	switch class {
	case "service":
		if round == 1 {
			return []string{
				"test -f /tmp/bidall_api.log && tail -n 120 /tmp/bidall_api.log || true",
				"curl -sS -i --max-time 8 http://127.0.0.1:8000/api/health || true",
			}
		}
		if round == 2 {
			return []string{
				"test -f /tmp/bidall_api.log && grep -Ein 'traceback|error|exception|asgi application|sqlalchemy|sqlite|operationalerror|integrityerror|programmingerror' /tmp/bidall_api.log | tail -n 120 || true",
				"source .venv311/bin/activate && (test -f alembic.ini && alembic upgrade head || true)",
				"source .venv311/bin/activate && (test -f scripts/dev_fix_sqlite_schema.py && python scripts/dev_fix_sqlite_schema.py || true)",
				"source .venv311/bin/activate && (test -f scripts/seed_sources.py && python scripts/seed_sources.py || true)",
			}
		}
		return []string{
			"pkill -f 'uvicorn app.main:app --host 127.0.0.1 --port 8000' || true",
			"source .venv311/bin/activate && bash -lc 'nohup uvicorn app.main:app --host 127.0.0.1 --port 8000 >/tmp/bidall_api.log 2>&1 & sleep 2; curl -sS -i --max-time 8 http://127.0.0.1:8000/api/health || true'",
			"test -f /tmp/bidall_api.log && tail -n 80 /tmp/bidall_api.log || true",
		}
	case "build":
		if round == 1 {
			return []string{
				"source .venv/bin/activate && python3 -m pip install -U pip setuptools wheel",
				"source .venv/bin/activate && pip install -e . --no-build-isolation",
			}
		}
		return []string{
			"source .venv/bin/activate && pip install -e . -v 2>&1 | tail -n 120",
		}
	case "network":
		if round == 1 {
			return []string{
				"getent hosts pypi.org || true && curl -I --max-time 8 https://pypi.org/simple || true",
				"source .venv/bin/activate && pip install -e . --index-url https://pypi.org/simple --retries 1 --timeout 8",
			}
		}
		return []string{
			"source .venv/bin/activate && pip install -e . --index-url https://pypi.tuna.tsinghua.edu.cn/simple --retries 1 --timeout 8",
			"source .venv/bin/activate && pip install -e . --index-url https://mirrors.aliyun.com/pypi/simple --retries 1 --timeout 8",
		}
	default:
		if round == 1 {
			return []string{
				"source .venv/bin/activate && python3 -m pip --version",
				"source .venv/bin/activate && pip install -e . --retries 1 --timeout 8",
			}
		}
		return nil
	}
}

func maxAutoRepairRounds() int {
	raw := strings.TrimSpace(os.Getenv("CLAWX_RUNTIME_EXEC_AUTO_REPAIR_ROUNDS"))
	if raw == "" {
		return 3
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 3
	}
	return value
}

func renderExecIDsForUser(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" || strings.HasSuffix(value, "(none)") {
		return "执行凭证：无"
	}
	items := strings.Split(strings.TrimPrefix(value, "exec_ids: "), ",")
	ids := make([]string, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item)
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return "执行凭证：无"
	}
	if len(ids) <= 2 {
		return "执行凭证：" + strings.Join(ids, ", ")
	}
	return fmt.Sprintf("执行凭证：%s, %s 等 %d 条", ids[0], ids[1], len(ids))
}

func validateRuntimeExecCommand(cmdline string) error {
	trimmed := strings.TrimSpace(cmdline)
	if trimmed == "" {
		return fmt.Errorf("empty command")
	}
	lower := strings.ToLower(trimmed)
	denyPhrases := []string{
		"rm -rf /", "mkfs", "shutdown", "reboot", "poweroff", "dd if=", ":(){",
	}
	for _, phrase := range denyPhrases {
		if strings.Contains(lower, phrase) {
			return fmt.Errorf("dangerous command pattern")
		}
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return fmt.Errorf("invalid command")
	}
	return nil
}

func runRuntimeExecCommand(ctx context.Context, cmdline string, cwd string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "bash", "-lc", cmdline)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func summarizeCommand(cmdline string) string {
	return summarizeText(strings.Join(strings.Fields(strings.TrimSpace(cmdline)), " "), 120)
}

func summarizeText(text string, limit int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return "(no output)"
	}
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit] + "..."
}
