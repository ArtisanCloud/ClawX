package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"clawx/internal/application/runtimeorchestrator"
	"clawx/internal/application/service"
	stdlogging "clawx/internal/infrastructure/logging"
)

const (
	defaultLeadWorkerTickInterval             = 2 * time.Second
	defaultLeadWorkerStaleAfter               = 30 * time.Minute
	defaultLeadWorkerDispatchCap              = 8
	defaultLeadWorkerExecTimeout              = 120 * time.Second
	defaultLeadWorkerSelfHealTick             = 30 * time.Second
	defaultLeadWorkerSelfHealAlertMinInterval = 2 * time.Minute
	defaultLeadWorkerSelfHealAlertMergeWindow = 5 * time.Minute
)

type leadWorkerLoop struct {
	runtime        agentRuntime
	root           string
	queue          *runtimeorchestrator.Queue
	registry       *runtimeorchestrator.Registry
	dispatcher     *runtimeorchestrator.Dispatcher
	recovery       *runtimeorchestrator.RecoveryEngine
	recorder       *stdlogging.RuntimeOrchestratorRecorder
	lastSelfHealAt time.Time
}

func startLeadWorkerExecutionLoops(ctx context.Context, runtimes map[string]agentRuntime) {
	if len(runtimes) == 0 {
		return
	}
	roots := make([]string, 0, len(runtimes))
	byRoot := make(map[string]agentRuntime, len(runtimes))
	for _, runtime := range runtimes {
		root := strings.TrimSpace(resolveTaskTrackingRoot(runtime, ""))
		if root == "" {
			continue
		}
		root = filepath.Clean(root)
		if _, exists := byRoot[root]; exists {
			continue
		}
		byRoot[root] = runtime
		roots = append(roots, root)
	}
	sort.Strings(roots)
	for _, root := range roots {
		runtime := byRoot[root]
		loop, err := newLeadWorkerLoop(runtime, root)
		if err != nil {
			log.Printf("lead worker loop skipped: agent=%s workspace=%s err=%v", strings.TrimSpace(runtime.agentID), root, err)
			continue
		}
		log.Printf("lead worker loop started: agent=%s workspace=%s", strings.TrimSpace(runtime.agentID), root)
		go loop.run(ctx)
	}
}

func newLeadWorkerLoop(runtime agentRuntime, workspaceRoot string) (*leadWorkerLoop, error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return nil, fmt.Errorf("workspace root is required")
	}
	runtimeDir := filepath.Join(root, ".clawx", "runtime")
	queue := runtimeorchestrator.NewQueue(filepath.Join(runtimeDir, "tasks.jsonl"))
	registry := runtimeorchestrator.NewRegistry(filepath.Join(runtimeDir, "worker_states.json"), filepath.Join(runtimeDir, "heartbeats.json"))
	dispatcher := runtimeorchestrator.NewDispatcher(queue, registry, filepath.Join(runtimeDir, "dispatch_state.json"))
	recorder, err := newLeadWorkerRecorder(runtimeDir)
	if err != nil {
		log.Printf("lead worker recorder disabled: workspace=%s err=%v", root, err)
	}
	recovery := runtimeorchestrator.NewRecoveryEngine(queue, registry, dispatcher, recorder, strings.TrimSpace(runtime.agentID))
	return &leadWorkerLoop{
		runtime:    runtime,
		root:       root,
		queue:      queue,
		registry:   registry,
		dispatcher: dispatcher,
		recovery:   recovery,
		recorder:   recorder,
	}, nil
}

func newLeadWorkerRecorder(runtimeDir string) (*stdlogging.RuntimeOrchestratorRecorder, error) {
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(runtimeDir, "runtime_orchestrator.jsonl")
	maxMB := parsePositiveIntEnv("CLAWX_RUNTIME_ORCHESTRATOR_LOG_MAX_MB", 20)
	maxBackups := parsePositiveIntEnv("CLAWX_RUNTIME_ORCHESTRATOR_LOG_MAX_BACKUPS", 5)
	return stdlogging.NewRuntimeOrchestratorRecorder(path, int64(maxMB)*1024*1024, maxBackups)
}

func (l *leadWorkerLoop) close() {
	if l == nil || l.recorder == nil {
		return
	}
	_ = l.recorder.Close()
}

func (l *leadWorkerLoop) run(ctx context.Context) {
	defer l.close()
	ticker := time.NewTicker(resolveLeadWorkerTickInterval())
	defer ticker.Stop()
	for {
		if err := l.runCycle(ctx, time.Now().UTC()); err != nil {
			log.Printf("lead worker cycle error: agent=%s workspace=%s err=%v", strings.TrimSpace(l.runtime.agentID), l.root, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (l *leadWorkerLoop) runCycle(ctx context.Context, now time.Time) error {
	if l == nil {
		return nil
	}
	leadMode, _, err := loadLeadModeFromRuntimeMeta(l.root)
	if err != nil {
		return fmt.Errorf("load lead mode: %w", err)
	}
	if !leadMode {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if _, err := l.recovery.Recover(now.UTC(), resolveLeadWorkerStaleAfter(), resolveLeadWorkerDispatchLimit()); err != nil {
		log.Printf("lead worker recovery warning: agent=%s workspace=%s err=%v", strings.TrimSpace(l.runtime.agentID), l.root, err)
	}
	if _, err := l.dispatcher.DispatchQueued(now.UTC(), resolveLeadWorkerDispatchLimit()); err != nil {
		return fmt.Errorf("dispatch queued: %w", err)
	}
	if err := l.executeRunningTasks(ctx); err != nil {
		return fmt.Errorf("execute running tasks: %w", err)
	}
	if err := l.runTaskControlSelfHealCycle(ctx, now.UTC()); err != nil {
		log.Printf("lead worker self-heal warning: agent=%s workspace=%s err=%v", strings.TrimSpace(l.runtime.agentID), l.root, err)
	}
	return nil
}

func (l *leadWorkerLoop) runTaskControlSelfHealCycle(ctx context.Context, now time.Time) error {
	if l == nil {
		return nil
	}
	if !resolveLeadWorkerSelfHealEnabled() {
		return nil
	}
	interval := resolveLeadWorkerSelfHealTick()
	if !l.lastSelfHealAt.IsZero() && interval > 0 && now.Sub(l.lastSelfHealAt) < interval {
		return nil
	}
	l.lastSelfHealAt = now

	agentID := strings.TrimSpace(l.runtime.agentID)
	if agentID == "" {
		agentID = "main"
	}
	serviceName := inferManagedServiceNameForAgent(agentID)
	scopeKey := "lead_worker:" + agentID
	conversationID := "lead-worker-self-heal:" + agentID
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: conversationID,
	}
	note, err := applyActionPlanRuntimeTaskControl(ctx, l.runtime, decision, actionPlanItem{
		Kind:           "runtime.task.control",
		AgentID:        agentID,
		Service:        serviceName,
		Operation:      "status",
		Mode:           "self_heal",
		Scope:          "user",
		TimeoutSec:     resolveLeadWorkerSelfHealTimeoutSeconds(),
		ConversationID: conversationID,
	}, scopeKey, l.root, map[string]agentRuntime{agentID: l.runtime}, agentID)
	if err != nil {
		return err
	}
	status := "ok"
	if strings.Contains(note, "auto_recovery=applied") {
		status = "applied"
	} else if strings.Contains(note, "auto_recovery=failed") {
		status = "failed"
	} else if strings.Contains(note, "auto_recovery=throttled") {
		status = "throttled"
	}
	l.audit("task_control_status", "", "", status, summarizeText(note, 180))

	alertMessage := ""
	if alert, ok := l.resolveTaskControlSelfHealAlert(serviceName, now.UTC()); ok {
		previousLevel, previousState, previousReason, previousNotifiedAt := lookupTaskControlSelfHealAlertNotify(l.runtime, l.root, serviceName)
		currentReason := normalizeTaskControlAutoRecoveryReason(extractTaskControlSummaryToken(note, "auto_recovery_reason"))
		currentHealthURL := extractTaskControlSummaryToken(note, "health_url")
		alertMessage = buildLeadWorkerSelfHealAlertTransitionMessage(
			agentID,
			serviceName,
			alert,
			previousLevel,
			previousState,
			previousReason,
			currentReason,
			currentHealthURL,
		)
		if alertMessage != "" && shouldSuppressLeadWorkerSelfHealAlertTransition(
			alert,
			previousLevel,
			previousState,
			previousReason,
			currentReason,
			previousNotifiedAt,
			now.UTC(),
			resolveLeadWorkerSelfHealAlertMinInterval(),
			resolveLeadWorkerSelfHealAlertMergeWindow(),
		) {
			alertMessage = ""
		}
		if alertMessage != "" {
			persistTaskControlSelfHealAlertNotify(l.runtime, l.root, serviceName, alert, currentReason, now.UTC())
		}
	}

	progressMessage := ""
	if status == "applied" && alertMessage == "" {
		progressMessage = buildLeadWorkerSelfHealProgressMessage(agentID, serviceName, status, note)
	}
	businessConversationIDs := l.resolveTaskControlSelfHealConversationIDs(serviceName, conversationID)
	if alertMessage != "" {
		alertTargets := resolveLeadWorkerSelfHealAlertTargets(alertMessage, businessConversationIDs)
		for _, notifyConversationID := range alertTargets {
			emitConversationProgressNotification(notifyConversationID, alertMessage)
		}
	}
	if progressMessage != "" {
		for _, notifyConversationID := range businessConversationIDs {
			emitConversationProgressNotification(notifyConversationID, progressMessage)
		}
	}
	return nil
}

func resolveLeadWorkerSelfHealAlertTargets(alertMessage string, businessConversationIDs []string) []string {
	currentLevel := normalizeTaskControlSelfHealAlertLevel(extractTaskControlSummaryToken(alertMessage, "self_heal_alert_level"))
	opsConversationIDs := resolveLeadWorkerSelfHealAlertConversationIDs()
	if len(opsConversationIDs) == 0 {
		return dedupeConversationIDs(businessConversationIDs)
	}
	switch currentLevel {
	case "warning", "critical":
		return dedupeConversationIDs(opsConversationIDs)
	case "ok":
		return dedupeConversationIDs(append(append([]string{}, opsConversationIDs...), businessConversationIDs...))
	default:
		return dedupeConversationIDs(opsConversationIDs)
	}
}

func dedupeConversationIDs(input []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(input))
	for _, raw := range input {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func (l *leadWorkerLoop) resolveTaskControlSelfHealAlert(serviceName string, now time.Time) (taskControlSelfHealAlert, bool) {
	if l == nil {
		return taskControlSelfHealAlert{}, false
	}
	root := resolveTaskTrackingRoot(l.runtime, l.root)
	if strings.TrimSpace(root) == "" {
		return taskControlSelfHealAlert{}, false
	}
	state, ok, err := loadWorkspaceTaskTrackingState(root)
	if err != nil || !ok {
		return taskControlSelfHealAlert{}, false
	}
	snapshot := state.TaskTrackingSnapshot
	if len(snapshot) == 0 {
		return taskControlSelfHealAlert{}, false
	}
	normalizedService := strings.TrimSpace(strings.ToLower(strings.TrimSuffix(serviceName, ".service")))
	lastSelfHealService := strings.TrimSpace(strings.ToLower(strings.TrimSuffix(readTaskTrackingString(snapshot, "last_task_control_self_heal_service"), ".service")))
	if normalizedService != "" && lastSelfHealService != "" && lastSelfHealService != normalizedService {
		return taskControlSelfHealAlert{}, false
	}
	lastSelfHealState := strings.TrimSpace(strings.ToLower(readTaskTrackingString(snapshot, "last_task_control_self_heal_state")))
	lastSelfHealAt := strings.TrimSpace(readTaskTrackingString(snapshot, "last_task_control_self_heal_at"))
	if lastSelfHealState == "" || lastSelfHealAt == "" {
		return taskControlSelfHealAlert{}, false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	alert := evaluateTaskControlSelfHealAlert(taskControlSelfHealMetric{
		AgentID: strings.TrimSpace(l.runtime.agentID),
		Service: fallbackValue(lastSelfHealService, normalizedService),
		State:   lastSelfHealState,
		At:      lastSelfHealAt,
	}, now.UTC(), resolveTaskControlSelfHealAlertWindow())
	return alert, true
}

func lookupTaskControlSelfHealAlertNotify(runtime agentRuntime, fallbackCWD string, serviceName string) (string, string, string, time.Time) {
	root := resolveTaskTrackingRoot(runtime, fallbackCWD)
	if strings.TrimSpace(root) == "" {
		return "", "", "", time.Time{}
	}
	state, ok, err := loadWorkspaceTaskTrackingState(root)
	if err != nil || !ok {
		return "", "", "", time.Time{}
	}
	snapshot := state.TaskTrackingSnapshot
	if len(snapshot) == 0 {
		return "", "", "", time.Time{}
	}
	serviceName = strings.TrimSpace(strings.ToLower(strings.TrimSuffix(serviceName, ".service")))
	if serviceName == "" {
		return "", "", "", time.Time{}
	}
	level := strings.TrimSpace(strings.ToLower(readTaskTrackingStringMap(snapshot, "service_self_heal_alert_levels")[serviceName]))
	alertState := strings.TrimSpace(strings.ToLower(readTaskTrackingStringMap(snapshot, "service_self_heal_alert_states")[serviceName]))
	alertReason := normalizeTaskControlAutoRecoveryReason(readTaskTrackingStringMap(snapshot, "service_self_heal_alert_reasons")[serviceName])
	notifiedAtRaw := strings.TrimSpace(readTaskTrackingStringMap(snapshot, "service_self_heal_alert_notified_at")[serviceName])
	if level == "" {
		lastService := strings.TrimSpace(strings.ToLower(strings.TrimSuffix(readTaskTrackingString(snapshot, "last_task_control_self_heal_alert_service"), ".service")))
		if lastService == "" || lastService == serviceName {
			level = strings.TrimSpace(strings.ToLower(readTaskTrackingString(snapshot, "last_task_control_self_heal_alert_level")))
			alertState = strings.TrimSpace(strings.ToLower(readTaskTrackingString(snapshot, "last_task_control_self_heal_alert_state")))
			alertReason = normalizeTaskControlAutoRecoveryReason(readTaskTrackingString(snapshot, "last_task_control_self_heal_alert_reason"))
			if strings.TrimSpace(notifiedAtRaw) == "" {
				notifiedAtRaw = strings.TrimSpace(readTaskTrackingString(snapshot, "last_task_control_self_heal_alert_at"))
			}
		}
	}
	notifiedAt, _ := parseTaskControlHintTime(notifiedAtRaw)
	return normalizeTaskControlSelfHealAlertLevel(level), alertState, alertReason, notifiedAt
}

func persistTaskControlSelfHealAlertNotify(runtime agentRuntime, fallbackCWD string, serviceName string, alert taskControlSelfHealAlert, currentReason string, now time.Time) {
	serviceName = strings.TrimSpace(strings.ToLower(strings.TrimSuffix(serviceName, ".service")))
	alertLevel := normalizeTaskControlSelfHealAlertLevel(alert.Level)
	alertState := strings.TrimSpace(strings.ToLower(alert.State))
	currentReason = normalizeTaskControlAutoRecoveryReason(currentReason)
	if serviceName == "" || alertLevel == "" {
		return
	}
	root := resolveTaskTrackingRoot(runtime, fallbackCWD)
	if strings.TrimSpace(root) == "" {
		return
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err := ensureTaskTrackingScaffold(root, now); err != nil {
		return
	}
	alertLevels := map[string]string{}
	alertStates := map[string]string{}
	alertReasons := map[string]string{}
	alertNotifiedAt := map[string]string{}
	if state, ok, err := loadWorkspaceTaskTrackingState(root); err == nil && ok {
		for key, value := range readTaskTrackingStringMap(state.TaskTrackingSnapshot, "service_self_heal_alert_levels") {
			alertLevels[key] = value
		}
		for key, value := range readTaskTrackingStringMap(state.TaskTrackingSnapshot, "service_self_heal_alert_states") {
			alertStates[key] = value
		}
		for key, value := range readTaskTrackingStringMap(state.TaskTrackingSnapshot, "service_self_heal_alert_reasons") {
			alertReasons[key] = value
		}
		for key, value := range readTaskTrackingStringMap(state.TaskTrackingSnapshot, "service_self_heal_alert_notified_at") {
			alertNotifiedAt[key] = value
		}
	}
	alertLevels[serviceName] = alertLevel
	alertNotifiedAt[serviceName] = now.Format(time.RFC3339)
	if alertState != "" {
		alertStates[serviceName] = alertState
	} else {
		delete(alertStates, serviceName)
	}
	if currentReason != "" {
		alertReasons[serviceName] = currentReason
	} else {
		delete(alertReasons, serviceName)
	}

	fields := map[string]interface{}{
		"service_self_heal_alert_levels":            alertLevels,
		"service_self_heal_alert_states":            alertStates,
		"service_self_heal_alert_reasons":           alertReasons,
		"service_self_heal_alert_notified_at":       alertNotifiedAt,
		"last_task_control_self_heal_alert_service": serviceName,
		"last_task_control_self_heal_alert_level":   alertLevel,
		"last_task_control_self_heal_alert_at":      now.Format(time.RFC3339),
		"last_task_control_self_heal_alert_reason":  currentReason,
	}
	if alertState != "" {
		fields["last_task_control_self_heal_alert_state"] = alertState
	}
	_ = updateWorkspaceTaskState(root, now, fields)
}

func normalizeTaskControlSelfHealAlertLevel(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "ok":
		return "ok"
	case "warning":
		return "warning"
	case "critical":
		return "critical"
	default:
		return ""
	}
}

func buildLeadWorkerSelfHealAlertTransitionMessage(
	agentID string,
	serviceName string,
	alert taskControlSelfHealAlert,
	previousLevel string,
	previousState string,
	previousReason string,
	currentReason string,
	currentHealthURL string,
) string {
	currentLevel := normalizeTaskControlSelfHealAlertLevel(alert.Level)
	previousLevel = normalizeTaskControlSelfHealAlertLevel(previousLevel)
	currentState := strings.TrimSpace(strings.ToLower(alert.State))
	previousState = strings.TrimSpace(strings.ToLower(previousState))
	previousReason = normalizeTaskControlAutoRecoveryReason(previousReason)
	currentReason = normalizeTaskControlAutoRecoveryReason(currentReason)
	currentHealthURL = sanitizeManagedHealthURL(currentHealthURL)
	if currentLevel == "" {
		return ""
	}
	agentID = fallbackValue(strings.TrimSpace(agentID), "main")
	serviceName = strings.TrimSpace(strings.TrimSuffix(serviceName, ".service"))
	if serviceName == "" {
		serviceName = inferManagedServiceNameForAgent(agentID)
	}

	headline := ""
	switch currentLevel {
	case "critical":
		if previousLevel == "critical" && previousState == currentState && previousReason == currentReason {
			return ""
		}
		if previousLevel == "warning" {
			headline = "自治自愈告警升级：已从 warning 升级为 critical，请立即人工介入。"
		} else {
			headline = "自治自愈告警：已进入 critical，请立即人工介入。"
		}
	case "warning":
		if previousLevel == "warning" && previousState == currentState && previousReason == currentReason {
			return ""
		}
		if previousLevel == "critical" {
			headline = "自治自愈告警降级：已从 critical 降级为 warning，仍需尽快处理。"
		} else {
			headline = "自治自愈告警：已进入 warning，请尽快检查恢复链路。"
		}
	case "ok":
		if previousLevel != "warning" && previousLevel != "critical" {
			return ""
		}
		headline = "自治自愈恢复：告警已恢复为 ok，自动恢复链路回到稳定状态。"
	default:
		return ""
	}

	lines := []string{
		headline,
		fmt.Sprintf("agent=%s service=%s self_heal_alert_level=%s", agentID, serviceName, currentLevel),
	}
	if currentState != "" {
		lines = append(lines, "self_heal_alert_state="+currentState)
	}
	if previousLevel != "" {
		lines = append(lines, "previous_alert_level="+previousLevel)
	}
	if previousState != "" {
		lines = append(lines, "previous_alert_state="+previousState)
	}
	if previousReason != "" {
		lines = append(lines, "previous_alert_reason="+previousReason)
		if reasonLabel := strings.TrimSpace(mapTaskControlAutoRecoveryReasonLabel(previousReason)); reasonLabel != "" {
			lines = append(lines, "previous_alert_reason_label="+reasonLabel)
		}
	}
	if currentReason != "" {
		lines = append(lines, "self_heal_alert_reason="+currentReason)
		if reasonLabel := strings.TrimSpace(mapTaskControlAutoRecoveryReasonLabel(currentReason)); reasonLabel != "" {
			lines = append(lines, "self_heal_alert_reason_label="+reasonLabel)
		}
	}
	lines = append(lines, "self_heal_alert_window="+resolveTaskControlSelfHealAlertWindow().String())
	playbook := buildLeadWorkerSelfHealAlertPlaybook(serviceName, currentLevel, currentState, currentReason, currentHealthURL)
	for idx, step := range playbook {
		lines = append(lines, fmt.Sprintf("playbook_%d=%s", idx+1, step))
	}
	return strings.Join(lines, "\n")
}

func buildLeadWorkerSelfHealAlertPlaybook(serviceName string, currentLevel string, currentState string, currentReason string, currentHealthURL string) []string {
	serviceName = strings.TrimSpace(strings.TrimSuffix(serviceName, ".service"))
	currentLevel = normalizeTaskControlSelfHealAlertLevel(currentLevel)
	currentState = strings.TrimSpace(strings.ToLower(currentState))
	currentReason = normalizeTaskControlAutoRecoveryReason(currentReason)
	currentHealthURL = sanitizeManagedHealthURL(currentHealthURL)
	if serviceName == "" {
		serviceName = "clawx"
	}
	unit := serviceName + ".service"
	statusCommand := "systemctl --user status " + unit + " --no-pager"
	logCommand := "journalctl --user-unit " + unit + " -n 120 --no-pager"
	healthCommand := ""
	if currentHealthURL != "" {
		healthCommand = "curl -fsS -m 5 " + currentHealthURL
	}
	releaseStatusCommand := "runtime.release.status operation=current service=" + serviceName
	switch currentLevel {
	case "critical":
		switch currentReason {
		case "health_down":
			steps := []string{}
			if healthCommand != "" {
				steps = append(steps, "先复测健康探针："+healthCommand)
			}
			steps = append(steps,
				"立即确认服务状态："+statusCommand,
				"查看最近错误日志："+logCommand,
				"执行自治恢复：runtime.task.control operation=ensure_running service="+serviceName,
			)
			return steps
		case "release_changed":
			return []string{
				"先核对当前发布版本：" + releaseStatusCommand,
				"立即确认服务状态：" + statusCommand,
				"执行自治恢复：runtime.task.control operation=ensure_running service=" + serviceName,
			}
		case "service_not_active":
			return []string{
				"立即确认服务状态：" + statusCommand,
				"查看最近错误日志：" + logCommand,
				"执行自治恢复：runtime.task.control operation=ensure_running service=" + serviceName,
			}
		default:
			return []string{
				"立即确认服务状态：" + statusCommand,
				"查看最近错误日志：" + logCommand,
				"执行自治恢复：runtime.task.control operation=ensure_running service=" + serviceName,
			}
		}
	case "warning":
		switch currentReason {
		case "health_down":
			steps := []string{}
			if healthCommand != "" {
				steps = append(steps, "先复测健康探针："+healthCommand)
			}
			steps = append(steps,
				"再检查服务状态："+statusCommand,
				"查看最近日志定位失败原因："+logCommand,
			)
			return steps
		case "release_changed":
			return []string{
				"先核对当前发布版本：" + releaseStatusCommand,
				"再检查服务状态：" + statusCommand,
			}
		case "service_not_active":
			return []string{
				"先检查服务状态：" + statusCommand,
				"查看最近日志定位失败原因：" + logCommand,
			}
		default:
			return []string{
				"先检查服务状态：" + statusCommand,
				"查看最近日志定位失败原因：" + logCommand,
			}
		}
	case "ok":
		if currentReason == "release_changed" {
			return []string{
				"当前已恢复；建议确认发布状态：" + releaseStatusCommand,
			}
		}
		if currentState == "not_needed" || currentState == "" {
			return []string{
				"当前已恢复，无需人工操作；若再次触发告警可先执行：" + statusCommand,
			}
		}
		return []string{
			"当前已恢复，无需人工操作；建议留意后续波动并按需执行：" + statusCommand,
		}
	default:
		return []string{
			"检查服务状态：" + statusCommand,
			"查看日志：" + logCommand,
		}
	}
}

func normalizeTaskControlAutoRecoveryReason(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "health_down":
		return "health_down"
	case "service_not_active":
		return "service_not_active"
	case "release_changed":
		return "release_changed"
	default:
		return ""
	}
}

func shouldSuppressLeadWorkerSelfHealAlertTransition(
	alert taskControlSelfHealAlert,
	previousLevel string,
	previousState string,
	previousReason string,
	currentReason string,
	previousNotifiedAt time.Time,
	now time.Time,
	minInterval time.Duration,
	mergeWindow time.Duration,
) bool {
	currentLevel := normalizeTaskControlSelfHealAlertLevel(alert.Level)
	previousLevel = normalizeTaskControlSelfHealAlertLevel(previousLevel)
	currentState := strings.TrimSpace(strings.ToLower(alert.State))
	previousState = strings.TrimSpace(strings.ToLower(previousState))
	currentReason = normalizeTaskControlAutoRecoveryReason(currentReason)
	previousReason = normalizeTaskControlAutoRecoveryReason(previousReason)
	if currentLevel == "" || now.IsZero() || previousNotifiedAt.IsZero() {
		return false
	}
	if currentLevel == "ok" {
		return false
	}
	// Always report escalation to critical even inside suppression windows.
	if currentLevel == "critical" && previousLevel == "warning" {
		return false
	}
	// Report any level/state/reason transition so operators can see root-cause changes.
	if previousLevel != currentLevel || previousState != currentState || previousReason != currentReason {
		return false
	}
	elapsed := now.Sub(previousNotifiedAt)
	if minInterval > 0 && elapsed < minInterval {
		return true
	}
	if mergeWindow > 0 && previousLevel == currentLevel && elapsed < mergeWindow {
		return true
	}
	return false
}

func (l *leadWorkerLoop) resolveTaskControlSelfHealConversationIDs(serviceName string, fallbackConversationID string) []string {
	seen := map[string]struct{}{}
	conversationIDs := make([]string, 0, 2)
	addConversationID := func(raw string) {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			return
		}
		if _, exists := seen[candidate]; exists {
			return
		}
		seen[candidate] = struct{}{}
		conversationIDs = append(conversationIDs, candidate)
	}

	addConversationID(fallbackConversationID)
	if l == nil {
		return conversationIDs
	}
	root := resolveTaskTrackingRoot(l.runtime, l.root)
	if strings.TrimSpace(root) == "" {
		return conversationIDs
	}
	state, ok, err := loadWorkspaceTaskTrackingState(root)
	if err != nil || !ok {
		return conversationIDs
	}
	snapshot := state.TaskTrackingSnapshot
	if len(snapshot) == 0 {
		return conversationIDs
	}

	if hintConversationID := pickLatestTaskControlHintConversationID(snapshot, serviceName); hintConversationID != "" {
		addConversationID(hintConversationID)
	}
	latestConversationID := strings.TrimSpace(readTaskTrackingString(snapshot, "conversation_id"))
	if latestConversationID != "" && !strings.HasPrefix(strings.ToLower(latestConversationID), "lead-worker-self-heal:") {
		recordedService := strings.TrimSpace(strings.ToLower(readTaskTrackingString(snapshot, "last_task_control_service")))
		normalizedService := strings.TrimSpace(strings.ToLower(strings.TrimSuffix(serviceName, ".service")))
		if normalizedService == "" || recordedService == "" || recordedService == normalizedService {
			addConversationID(latestConversationID)
		}
	}
	return conversationIDs
}

func pickLatestTaskControlHintConversationID(snapshot map[string]interface{}, serviceName string) string {
	if len(snapshot) == 0 {
		return ""
	}
	routeHints := readTaskTrackingRouteHintMap(snapshot, "task_control_route_hints")
	if len(routeHints) == 0 {
		return ""
	}
	normalizedService := strings.TrimSpace(strings.ToLower(strings.TrimSuffix(serviceName, ".service")))
	bestConversationID := ""
	bestUpdatedAt := time.Time{}
	bestUpdatedAtSet := false
	for _, record := range routeHints {
		hintService := strings.TrimSpace(strings.ToLower(strings.TrimSuffix(record.Service, ".service")))
		if normalizedService != "" && hintService != "" && hintService != normalizedService {
			continue
		}
		conversationID := strings.TrimSpace(record.ConversationID)
		if conversationID == "" || strings.HasPrefix(strings.ToLower(conversationID), "lead-worker-self-heal:") {
			continue
		}
		updatedAt, hasUpdatedAt := parseTaskControlHintTime(record.UpdatedAt)
		switch {
		case bestConversationID == "":
			bestConversationID = conversationID
			bestUpdatedAt = updatedAt
			bestUpdatedAtSet = hasUpdatedAt
		case hasUpdatedAt && (!bestUpdatedAtSet || updatedAt.After(bestUpdatedAt)):
			bestConversationID = conversationID
			bestUpdatedAt = updatedAt
			bestUpdatedAtSet = true
		}
	}
	return bestConversationID
}

func buildLeadWorkerSelfHealProgressMessage(agentID string, serviceName string, status string, note string) string {
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" || status == "ok" {
		return ""
	}
	agentID = fallbackValue(strings.TrimSpace(agentID), "main")
	serviceName = strings.TrimSpace(strings.TrimSuffix(serviceName, ".service"))
	if serviceName == "" {
		serviceName = inferManagedServiceNameForAgent(agentID)
	}

	headline := ""
	switch status {
	case "applied":
		headline = "自治自愈进展：检测到运行异常，已自动恢复服务。"
	case "failed":
		headline = "自治自愈进展：检测到运行异常，但自动恢复失败，需要人工处理。"
	case "throttled":
		headline = "自治自愈进展：检测到运行异常，但因连续失败进入冷却窗口。"
	default:
		return ""
	}

	lines := []string{
		headline,
		fmt.Sprintf("agent=%s service=%s auto_recovery=%s", agentID, serviceName, status),
	}
	if reason := extractTaskControlSummaryToken(note, "auto_recovery_reason"); reason != "" {
		lines = append(lines, "auto_recovery_reason="+reason)
		if reasonLabel := strings.TrimSpace(mapTaskControlAutoRecoveryReasonLabel(reason)); reasonLabel != "" {
			lines = append(lines, "auto_recovery_reason_label="+reasonLabel)
		}
	}
	if failures := extractTaskControlSummaryToken(note, "auto_recovery_failures"); failures != "" {
		lines = append(lines, "auto_recovery_failures="+failures)
	}
	if cooldownUntil := extractTaskControlSummaryToken(note, "auto_recovery_cooldown_until"); cooldownUntil != "" {
		lines = append(lines, "auto_recovery_cooldown_until="+cooldownUntil)
	}
	if evidence := strings.TrimSpace(summarizeText(oneLine(note), 200)); evidence != "" {
		lines = append(lines, "evidence="+evidence)
	}
	return strings.Join(lines, "\n")
}

func extractTaskControlSummaryToken(note string, key string) string {
	note = strings.TrimSpace(note)
	key = strings.TrimSpace(strings.ToLower(key))
	if note == "" || key == "" {
		return ""
	}
	prefix := key + "="
	for _, token := range strings.FieldsFunc(note, func(r rune) bool {
		switch r {
		case ' ', '\n', '\t', '\r', ';', '；', ',', '，':
			return true
		default:
			return false
		}
	}) {
		candidate := strings.TrimSpace(strings.Trim(token, "\"'`[]()"))
		if candidate == "" {
			continue
		}
		lower := strings.ToLower(candidate)
		if !strings.HasPrefix(lower, prefix) {
			continue
		}
		value := strings.TrimSpace(candidate[len(prefix):])
		return strings.Trim(value, "\"'`[]()。.,，;；")
	}
	return ""
}

func (l *leadWorkerLoop) executeRunningTasks(ctx context.Context) error {
	tasks, err := l.queue.List()
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if task.Status != runtimeorchestrator.TaskRunning {
			continue
		}
		if strings.TrimSpace(task.AssignedWorkerID) == "" {
			continue
		}
		if err := l.executeAssignedTask(ctx, task); err != nil {
			log.Printf("lead worker execute warning: agent=%s workspace=%s task_id=%s err=%v", strings.TrimSpace(l.runtime.agentID), l.root, strings.TrimSpace(task.TaskID), err)
		}
	}
	return nil
}

func (l *leadWorkerLoop) executeAssignedTask(ctx context.Context, task runtimeorchestrator.RuntimeTask) error {
	workerID := strings.TrimSpace(task.AssignedWorkerID)
	if workerID == "" {
		return nil
	}
	cmdline := readRuntimeTaskPayloadString(task.Payload, "cmd")
	execCWD := readRuntimeTaskPayloadString(task.Payload, "cwd")
	if execCWD == "" {
		execCWD = l.root
	}
	conversationID := readRuntimeTaskPayloadString(task.Payload, "conversation")
	parentTaskID := readRuntimeTaskPayloadString(task.Payload, "parent_task_id")
	reason := readRuntimeTaskPayloadString(task.Payload, "reason")
	fallbackSource := readRuntimeTaskPayloadString(task.Payload, "fallback_source")
	decisionMode := readRuntimeTaskPayloadString(task.Payload, "runtime_exec_decision_mode")
	decisionApplySource := readRuntimeTaskPayloadString(task.Payload, "runtime_exec_decision_apply_source")
	decisionLockSource := readRuntimeTaskPayloadString(task.Payload, "runtime_exec_decision_lock_source")
	role := l.resolveWorkerRole(workerID)

	if strings.TrimSpace(cmdline) == "" {
		return l.failAssignedTask(workerID, role, task, conversationID, "(no output)", "task payload missing cmd", "")
	}
	if err := l.runtime.cfgSnapshot.ValidateWorkingDirectory(execCWD); err != nil {
		return l.failAssignedTask(workerID, role, task, conversationID, "(no output)", "task cwd invalid: "+err.Error(), "")
	}

	now := time.Now().UTC()
	_ = l.registry.ReportHeartbeat(workerID, now)
	_ = l.registry.UpdateWorkerState(workerID, "busy", strings.TrimSpace(task.TaskID), role, now)
	l.audit("task_execute_start", strings.TrimSpace(task.TaskID), workerID, "running", summarizeCommand(cmdline))

	runCtx, cancel := context.WithTimeout(ctx, resolveLeadWorkerExecTimeout())
	defer cancel()
	stepReason := strings.TrimSpace(reason)
	if stepReason == "" {
		stepReason = "runtime_lead_worker_task"
	}
	out, runErr, execID := performRuntimeExecStep(
		runCtx,
		l.runtime,
		conversationID,
		execCWD,
		cmdline,
		"runtime_lead_worker_dispatch",
		stepReason,
		strings.TrimSpace(decisionMode),
		strings.TrimSpace(decisionApplySource),
		strings.TrimSpace(decisionLockSource),
		strings.TrimSpace(fallbackSource),
	)
	if runErr != nil {
		failure := extractFailureEvidence(out, runErr.Error())
		if strings.TrimSpace(failure) == "" {
			failure = summarizeText(runErr.Error(), 180)
		}
		return l.failAssignedTask(workerID, role, task, conversationID, out, failure, execID)
	}

	if _, _, err := l.queue.UpdateStatus(strings.TrimSpace(task.TaskID), runtimeorchestrator.TaskSucceeded); err != nil {
		return err
	}
	doneAt := time.Now().UTC()
	_ = l.registry.UpdateWorkerState(workerID, "idle", "", role, doneAt)
	_ = l.registry.ReportHeartbeat(workerID, doneAt)

	successSummary := summarizeSuccessEvidence(runtimeExecCommand{Cmd: cmdline, Reason: reason}, execCWD, out)
	if strings.TrimSpace(successSummary) == "" {
		successSummary = "命令执行成功"
	}
	successPayloadPatch := map[string]interface{}{
		"last_status":         "succeeded",
		"last_result":         summarizeText(successSummary, 180),
		"last_error":          nil,
		"last_error_preview":  nil,
		"last_output_preview": summarizeText(strings.TrimSpace(out), 180),
		"last_exec_id":        strings.TrimSpace(execID),
		"last_worker_id":      workerID,
		"last_finished_at":    doneAt.Format(time.RFC3339),
	}
	if _, _, err := l.queue.MergePayload(strings.TrimSpace(task.TaskID), successPayloadPatch); err != nil {
		log.Printf("lead worker payload merge warning: agent=%s workspace=%s task_id=%s err=%v", strings.TrimSpace(l.runtime.agentID), l.root, strings.TrimSpace(task.TaskID), err)
	}
	l.audit("task_execute_done", strings.TrimSpace(task.TaskID), workerID, "succeeded", summarizeText(successSummary, 180))
	appendTrackingExecution(l.runtime, execCWD, conversationID, runtimeExecApplyResult{
		Applied:     true,
		Status:      "applied",
		Message:     successSummary,
		Executed:    1,
		Failed:      0,
		Total:       1,
		FirstReason: successSummary,
		SuccessInfo: []string{successSummary},
		ExecIDs:     []string{strings.TrimSpace(execID)},
	})
	if strings.TrimSpace(conversationID) != "" {
		setExecutionGoalState(conversationID, executionGoalState{
			AgentID:    strings.TrimSpace(l.runtime.agentID),
			Status:     "running",
			LastResult: summarizeText(successSummary, 220),
			NextAction: "继续推进后续步骤",
		})
		updateExecutionGoalWithDelegateProgress(l.queue, conversationID, parentTaskID, strings.TrimSpace(l.runtime.agentID))
	}
	return nil
}

func (l *leadWorkerLoop) failAssignedTask(workerID, role string, task runtimeorchestrator.RuntimeTask, conversationID string, out string, reason string, execID string) error {
	taskID := strings.TrimSpace(task.TaskID)
	if _, _, err := l.queue.UpdateStatus(taskID, runtimeorchestrator.TaskFailed); err != nil {
		return err
	}
	doneAt := time.Now().UTC()
	_ = l.registry.UpdateWorkerState(workerID, "idle", "", role, doneAt)
	_ = l.registry.ReportHeartbeat(workerID, doneAt)

	reason = summarizeText(strings.TrimSpace(reason), 180)
	failurePayloadPatch := map[string]interface{}{
		"last_status":         "failed",
		"last_error":          reason,
		"last_error_preview":  summarizeText(strings.TrimSpace(out), 180),
		"last_output_preview": summarizeText(strings.TrimSpace(out), 180),
		"last_exec_id":        strings.TrimSpace(execID),
		"last_worker_id":      workerID,
		"last_finished_at":    doneAt.Format(time.RFC3339),
	}
	if _, _, err := l.queue.MergePayload(taskID, failurePayloadPatch); err != nil {
		log.Printf("lead worker payload merge warning: agent=%s workspace=%s task_id=%s err=%v", strings.TrimSpace(l.runtime.agentID), l.root, taskID, err)
	}
	l.audit("task_execute_done", taskID, workerID, "failed", reason)
	execIDs := []string{}
	if strings.TrimSpace(execID) != "" {
		execIDs = append(execIDs, strings.TrimSpace(execID))
	}
	appendTrackingExecution(l.runtime, readRuntimeTaskPayloadString(task.Payload, "cwd"), conversationID, runtimeExecApplyResult{
		Applied:     false,
		Status:      "failed",
		Message:     reason,
		Executed:    0,
		Failed:      1,
		Total:       1,
		FirstReason: reason,
		FailedCmd:   summarizeCommand(readRuntimeTaskPayloadString(task.Payload, "cmd")),
		ExecIDs:     execIDs,
	})
	if strings.TrimSpace(conversationID) != "" {
		setExecutionGoalState(conversationID, executionGoalState{
			AgentID:    strings.TrimSpace(l.runtime.agentID),
			Status:     "blocked",
			LastResult: summarizeText(reason, 220),
			NextAction: "修复失败原因后继续执行",
		})
		updateExecutionGoalWithDelegateProgress(l.queue, conversationID, readRuntimeTaskPayloadString(task.Payload, "parent_task_id"), strings.TrimSpace(l.runtime.agentID))
	}
	return nil
}

func (l *leadWorkerLoop) resolveWorkerRole(workerID string) string {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return "executor"
	}
	workers, err := l.registry.Workers()
	if err != nil {
		return "executor"
	}
	for _, item := range workers {
		if strings.TrimSpace(item.WorkerID) != workerID {
			continue
		}
		role := strings.TrimSpace(strings.ToLower(item.Role))
		if role == "" {
			return "executor"
		}
		return role
	}
	return "executor"
}

func (l *leadWorkerLoop) audit(event, taskID, workerID, status, reason string) {
	if l == nil || l.recorder == nil {
		return
	}
	_ = l.recorder.Append(stdlogging.RuntimeOrchestratorAuditRecord{
		AgentID:  strings.TrimSpace(l.runtime.agentID),
		Event:    strings.TrimSpace(event),
		TaskID:   strings.TrimSpace(taskID),
		WorkerID: strings.TrimSpace(workerID),
		Status:   strings.TrimSpace(status),
		Reason:   strings.TrimSpace(reason),
	})
}

func readRuntimeTaskPayloadString(payload map[string]interface{}, key string) string {
	if len(payload) == 0 {
		return ""
	}
	raw, ok := payload[strings.TrimSpace(key)]
	if !ok {
		return ""
	}
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func resolveLeadWorkerTickInterval() time.Duration {
	return parsePositiveDurationEnv("CLAWX_RUNTIME_WORKER_TICK", defaultLeadWorkerTickInterval)
}

func resolveLeadWorkerStaleAfter() time.Duration {
	return parsePositiveDurationEnv("CLAWX_RUNTIME_WORKER_STALE_AFTER", defaultLeadWorkerStaleAfter)
}

func resolveLeadWorkerExecTimeout() time.Duration {
	return parsePositiveDurationEnv("CLAWX_RUNTIME_WORKER_EXEC_TIMEOUT", defaultLeadWorkerExecTimeout)
}

func resolveLeadWorkerDispatchLimit() int {
	return parsePositiveIntEnv("CLAWX_RUNTIME_WORKER_DISPATCH_LIMIT", defaultLeadWorkerDispatchCap)
}

func resolveLeadWorkerSelfHealAlertConversationIDs() []string {
	raw := strings.TrimSpace(os.Getenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_ALERT_CONVERSATIONS"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_ALERT_CONVERSATION"))
	}
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\r', '\t':
			return true
		default:
			return false
		}
	})
	return dedupeConversationIDs(parts)
}

func resolveLeadWorkerSelfHealAlertMinInterval() time.Duration {
	return parseNonNegativeDurationEnv("CLAWX_RUNTIME_LEAD_SELF_HEAL_ALERT_MIN_INTERVAL", defaultLeadWorkerSelfHealAlertMinInterval)
}

func resolveLeadWorkerSelfHealAlertMergeWindow() time.Duration {
	return parseNonNegativeDurationEnv("CLAWX_RUNTIME_LEAD_SELF_HEAL_ALERT_MERGE_WINDOW", defaultLeadWorkerSelfHealAlertMergeWindow)
}

func resolveLeadWorkerSelfHealEnabled() bool {
	return parseManagedBool(os.Getenv("CLAWX_RUNTIME_LEAD_SELF_HEAL_ENABLED"))
}

func resolveLeadWorkerSelfHealTick() time.Duration {
	return parsePositiveDurationEnv("CLAWX_RUNTIME_LEAD_SELF_HEAL_TICK", defaultLeadWorkerSelfHealTick)
}

func resolveLeadWorkerSelfHealTimeoutSeconds() int {
	timeout := parsePositiveDurationEnv("CLAWX_RUNTIME_LEAD_SELF_HEAL_TIMEOUT", defaultManagedServiceTimeout)
	seconds := int(timeout / time.Second)
	if seconds <= 0 {
		return int(defaultManagedServiceTimeout / time.Second)
	}
	if seconds > int(maxManagedServiceTimeout/time.Second) {
		return int(maxManagedServiceTimeout / time.Second)
	}
	return seconds
}

func parsePositiveDurationEnv(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseNonNegativeDurationEnv(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	if parsed, err := time.ParseDuration(raw); err == nil && parsed >= 0 {
		return parsed
	}
	if seconds, err := strconv.Atoi(raw); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}
