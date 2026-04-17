package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"clawx/internal/application/runtimeorchestrator"
	"clawx/internal/application/service"
	chatiface "clawx/internal/interfaces/chat"
)

var actionPlanBlockPattern = regexp.MustCompile("(?is)```(?:json)?\\s*(\\{.*?\"type\"\\s*:\\s*\"action_plan\".*?\\})\\s*```")
var releaseServiceFromTextPattern = regexp.MustCompile(`(?i)\bclawx-[a-z0-9_.@-]+\b`)
var healthURLFromTextPattern = regexp.MustCompile(`https?://[^\s'"<>]+`)
var localhostPortFromTextPattern = regexp.MustCompile(`(?:127\.0\.0\.1|localhost)\s*[:：]\s*([0-9]{2,5})`)

type actionPlan struct {
	Type    string           `json:"type"`
	Mode    string           `json:"mode"`
	Reason  string           `json:"reason,omitempty"`
	Actions []actionPlanItem `json:"actions"`
}

type actionPlanItem struct {
	Kind           string   `json:"kind"`
	Cmd            string   `json:"cmd,omitempty"`
	CWD            string   `json:"cwd,omitempty"`
	Reason         string   `json:"reason,omitempty"`
	FallbackSource string   `json:"fallback_source,omitempty"`
	Command        string   `json:"command,omitempty"`
	AgentID        string   `json:"agent_id,omitempty"`
	DelegatedAgent string   `json:"delegated_agent,omitempty"`
	Requirement    string   `json:"requirement,omitempty"`
	Mode           string   `json:"mode,omitempty"`
	WorkerRoles    []string `json:"worker_roles,omitempty"`
	Service        string   `json:"service,omitempty"`
	Operation      string   `json:"operation,omitempty"`
	Scope          string   `json:"scope,omitempty"`
	HealthURL      string   `json:"health_url,omitempty"`
	TimeoutSec     int      `json:"timeout_seconds,omitempty"`
	MaxRetry       int      `json:"max_retry,omitempty"`
	Script         string   `json:"script,omitempty"`
	RollbackScript string   `json:"rollback_script,omitempty"`
	ArtifactPath   string   `json:"artifact_path,omitempty"`
	ApprovalToken  string   `json:"approval_token,omitempty"`
	UnitFile       string   `json:"unit_file,omitempty"`
	ReleaseVersion string   `json:"release_version,omitempty"`
	Limit          int      `json:"limit,omitempty"`
	ConversationID string   `json:"conversation_id,omitempty"`
	ParentTaskID   string   `json:"parent_task_id,omitempty"`
	TaskID         string   `json:"task_id,omitempty"`
	ResourceKey    string   `json:"resource_key,omitempty"`
	TaskTitle      string   `json:"task_title,omitempty"`
	TaskSummary    string   `json:"task_summary,omitempty"`
}

var managedServiceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.@-]+$`)

const (
	defaultManagedServiceTimeout               = 45 * time.Second
	maxManagedServiceTimeout                   = 180 * time.Second
	defaultManagedServiceLockWait              = 30 * time.Second
	staleManagedServiceLockDuration            = 30 * time.Minute
	defaultTaskControlHintTTL                  = 72 * time.Hour
	defaultTaskControlHintMax                  = 64
	defaultTaskControlSelfHealFailureThreshold = 2
	defaultTaskControlSelfHealCooldown         = 5 * time.Minute
	maxTaskControlSelfHealFailureThreshold     = 10
	maxTaskControlSelfHealCooldown             = 2 * time.Hour
)

type managedServiceLock struct {
	path string
}

type managedReleaseRecord struct {
	ReleaseID         string `json:"release_id"`
	Service           string `json:"service"`
	Version           string `json:"version"`
	Status            string `json:"status"`
	StartedAt         string `json:"started_at"`
	FinishedAt        string `json:"finished_at"`
	Script            string `json:"script,omitempty"`
	RollbackScript    string `json:"rollback_script,omitempty"`
	HealthURL         string `json:"health_url,omitempty"`
	ArtifactPath      string `json:"artifact_path,omitempty"`
	ArtifactSHA256    string `json:"artifact_sha256,omitempty"`
	ArtifactSizeBytes int64  `json:"artifact_size_bytes,omitempty"`
	TargetBinary      string `json:"target_binary,omitempty"`
	BackupBinary      string `json:"backup_binary,omitempty"`
	SourceBinary      string `json:"source_binary,omitempty"`
	DeployOutput      string `json:"deploy_output,omitempty"`
	Error             string `json:"error,omitempty"`
	RollbackNote      string `json:"rollback_note,omitempty"`
}

func parseActionPlanFromText(text string) (actionPlan, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return actionPlan{}, false
	}
	for _, payload := range extractStructuredPayloadMaps(trimmed) {
		if strings.TrimSpace(strings.ToLower(toString(payload["type"]))) != "action_plan" {
			continue
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			continue
		}
		var plan actionPlan
		if err := json.Unmarshal(encoded, &plan); err != nil {
			continue
		}
		plan.Type = strings.TrimSpace(strings.ToLower(plan.Type))
		plan.Mode = strings.TrimSpace(strings.ToLower(plan.Mode))
		if plan.Type != "action_plan" {
			continue
		}
		return plan, true
	}
	return actionPlan{}, false
}

func stripActionPlanPayload(output string) string {
	text := strings.TrimSpace(output)
	if text == "" {
		return text
	}
	if _, ok := parseActionPlanFromText(text); !ok {
		return output
	}
	if strings.HasPrefix(text, "{") && strings.HasSuffix(text, "}") {
		return ""
	}
	cleaned := actionPlanBlockPattern.ReplaceAllString(text, "")
	for _, snippet := range extractJSONObjectSnippets(cleaned) {
		var payload map[string]any
		if err := json.Unmarshal([]byte(snippet), &payload); err != nil {
			continue
		}
		if strings.TrimSpace(strings.ToLower(toString(payload["type"]))) != "action_plan" {
			continue
		}
		cleaned = strings.Replace(cleaned, snippet, "", 1)
		break
	}
	return strings.TrimSpace(cleaned)
}

type actionPlanApplyResult struct {
	Applied         bool
	Status          string
	Message         string
	ResponseAgentID string
	ReadOnlyQuery   bool
}

var specKitDocPathPattern = regexp.MustCompile(`(?i)(/[^,\s]+/(SPEC|PLAN|TASKS|ANALYZE)\.md)`)
var specKitDirPathPattern = regexp.MustCompile(`(?i)(/[^,\s'"]+/docs/spec-kit/[^/\s'"]+)`)
var specKitDirRelPattern = regexp.MustCompile(`(?i)docs/spec-kit/([^/\s'"]+)`)

type actionPlanNoteCategory int

const (
	actionPlanNoteCategoryUnknown actionPlanNoteCategory = iota
	actionPlanNoteCategoryQuery
	actionPlanNoteCategoryMutation
	actionPlanNoteCategoryError
)

const (
	nextStepContinueFromFailures      = "直接发送下一条消息（如“继续”），我会从失败项开始续跑并回报结果。"
	nextStepRetryFromRecovery         = "直接发送下一条消息（如“继续”），我会按恢复动作从当前进度重试。"
	nextStepContinueByRemaining       = "继续按剩余步骤推进，并回报关键证据。"
	nextStepReleaseInitPublish        = "如需上线，直接告诉我发布版本，我会继续执行 runtime.release publish。"
	nextStepReleaseDecideByCurrent    = "如需变更版本，可继续执行 runtime.release publish 或 rollback。"
	nextStepReleaseInspectHistory     = "可继续查询 history，或指定 release_id 执行回滚。"
	nextStepReleaseObserveMetrics     = "确认服务运行状态并继续观察业务指标。"
	nextStepServiceDecideByStatus     = "根据状态结果决定是否需要 start/restart。"
	nextStepServiceRecheckStatus      = "建议执行 runtime.service status 复核服务状态。"
	nextStepServiceOnlineProceed      = "服务在线，可继续推进任务。"
	nextStepServiceRecover            = "可执行 restart/ensure_running 以恢复服务。"
	nextStepSupervisorDecideByStatus  = "根据 supervisor 状态决定是否需要 ensure/disable。"
	nextStepSupervisorRecheckStatus   = "建议执行 runtime.supervisor status 复核 unit 状态。"
	nextStepSupervisorOnlineProceed   = "supervisor 在线，可继续推进任务。"
	nextStepSupervisorRecover         = "可执行 runtime.supervisor ensure 或 runtime.service restart。"
	nextStepConfigApply               = "确认后执行 `/config apply`。"
	nextStepConfigPlanFirst           = "先发送 `/config plan ...` 生成配置计划。"
	nextStepConfigShow                = "可用 `/config show` 检查当前配置。"
	nextStepConfigApplyPersist        = "如需落盘生效，继续执行 `/config apply`。"
	nextStepDelegatesPrioritizeFailed = "优先处理 failed 子任务，再继续推进。"
	nextStepDelegateWait              = "等待子任务执行后再汇总。"
	nextStepRetryWait                 = "等待重试执行完成后再查看汇总状态。"
	nextStepCancelDecide              = "确认是否需要重试或重新委派。"
	nextStepTaskControlCheckStatus    = "可执行 runtime.task.status 查看任务推进状态。"
	nextStepWorkerOnline              = "worker 服务在线，可继续推进任务。"
	nextStepWorkerRecover             = "建议先执行 runtime.task.control ensure_running 或 restart。"
	nextStepAutonomyProgressDefault   = "按剩余步骤继续推进并补充关键证据。"
	nextStepDelegateRawRetry          = "优先处理 failed 子任务后重试。"
	nextStepDelegateRawWait           = "等待运行中/排队子任务完成后再次查询。"
	nextStepDelegateRawDone           = "子任务已完成，可汇总结果并回复用户。"
)

type renderedActionPlanNote struct {
	Text     string
	Category actionPlanNoteCategory
}

func formatActionPlanApplyResult(result actionPlanApplyResult) string {
	if msg := strings.TrimSpace(result.Message); msg != "" {
		return msg
	}
	return fmt.Sprintf("action_plan status=%s", strings.TrimSpace(result.Status))
}

func buildRuntimeExecDecisionStatusLine(conversationID string) string {
	mode := "none"
	source := "none"
	lockMode, lockSource := resolveRuntimeExecDecisionLockState(conversationID)
	if lockMode != "" {
		mode = lockMode
		source = fallbackValue(strings.TrimSpace(lockSource), "locked_state")
	}
	lockLabel := mapRuntimeExecDecisionLockLabel(mode)
	return strings.Join([]string{
		fmt.Sprintf("执行策略锁定：%s。", lockLabel),
		fmt.Sprintf("runtime_exec_decision 当前锁定：mode=%s source=%s（%s）", mode, source, lockLabel),
	}, "\n")
}

func mapRuntimeExecDecisionLockLabel(mode string) string {
	switch normalizeRuntimeExecDecisionMode(mode) {
	case "", "none":
		return "未锁定"
	case "paused":
		return "已暂停自动执行"
	case "blocked.command_override":
		return "替代命令覆盖模式"
	case "service.retry_only":
		return "仅重试模式"
	case "build.switch_source":
		return "切换依赖源模式"
	case "service.deep_repair":
		return "服务深修模式"
	case "build.continue":
		return "构建持续修复模式"
	default:
		return "已锁定"
	}
}

func resolveRuntimeExecDecisionLockState(conversationID string) (string, string) {
	if goal, ok := getExecutionGoalState(conversationID); ok {
		mode := normalizeRuntimeExecDecisionMode(goal.RuntimeExecDecisionMode)
		if mode == "" {
			return "", ""
		}
		source := strings.TrimSpace(goal.RuntimeExecDecisionSource)
		if source == "" || strings.EqualFold(source, "__clear__") {
			source = "locked_state"
		}
		return mode, source
	}
	return "", ""
}

func formatRuntimeExecDecisionApplySource(decisionSource string, conversationID string) string {
	decisionSource = fallbackValue(strings.TrimSpace(decisionSource), "user_phrase")
	if !strings.EqualFold(decisionSource, "persisted_state") {
		return decisionSource
	}
	_, lockSource := resolveRuntimeExecDecisionLockState(conversationID)
	lockSource = strings.TrimSpace(lockSource)
	if lockSource == "" || strings.EqualFold(lockSource, "persisted_state") {
		return decisionSource
	}
	return fmt.Sprintf("%s lock_source=%s", decisionSource, lockSource)
}

func prependRuntimeExecDecisionStatusLine(message string, line string) string {
	message = strings.TrimSpace(message)
	line = strings.TrimSpace(line)
	if line == "" {
		return message
	}
	if message == "" {
		return line
	}
	if strings.HasPrefix(message, line) {
		return message
	}
	return line + "\n" + message
}

func renderActionPlanUserMessage(status string, notes []string, appliedCount, failedCount int) string {
	status = strings.TrimSpace(strings.ToLower(status))
	normalized := make([]renderedActionPlanNote, 0, len(notes))
	allQueryLike := true
	queryCount := 0
	mutationCount := 0
	errorCount := 0
	hasNextStep := false
	for _, note := range notes {
		note = normalizeActionPlanNote(note)
		if note == "" {
			continue
		}
		note = humanizeActionPlanNote(note)
		if actionPlanNoteContainsNextStep(note) {
			hasNextStep = true
		}
		category := classifyActionPlanNote(note)
		normalized = append(normalized, renderedActionPlanNote{
			Text:     note,
			Category: category,
		})
		switch category {
		case actionPlanNoteCategoryQuery:
			queryCount++
		case actionPlanNoteCategoryMutation:
			mutationCount++
		case actionPlanNoteCategoryError:
			errorCount++
		}
		if !isQueryLikeActionPlanNote(note) {
			allQueryLike = false
		}
	}

	if len(normalized) == 1 {
		entry := normalized[0]
		shouldRenderTemplate := (status == "partial" || status == "failed") &&
			(entry.Category == actionPlanNoteCategoryError || !hasNextStep)
		if !shouldRenderTemplate {
			return entry.Text
		}
	}
	if len(normalized) > 0 && failedCount == 0 && allQueryLike {
		lines := mergeQueryOnlyActionPlanNotes(normalized)
		if overview := buildQueryOnlyOverviewLine(lines); overview != "" {
			lines = append([]string{overview}, lines...)
		}
		return strings.TrimSpace(strings.Join(lines, "\n"))
	}

	lines := make([]string, 0, len(normalized)+2)
	switch status {
	case "applied":
		lines = append(lines, fmt.Sprintf("本轮执行完成：成功 %d 项。", appliedCount))
		lines = append(lines, "完成状态：已完成。")
	case "partial":
		lines = append(lines, fmt.Sprintf("本轮执行部分完成：成功 %d 项，失败 %d 项。", appliedCount, failedCount))
		lines = append(lines, "完成状态：部分失败。")
	case "failed":
		lines = append(lines, fmt.Sprintf("本轮执行失败：失败 %d 项。", failedCount))
		lines = append(lines, "完成状态：失败。")
	default:
		lines = append(lines, "本轮没有识别到可执行动作。")
		lines = append(lines, "完成状态：未执行。")
	}
	if summary := formatActionPlanNotesOverview(queryCount, mutationCount, errorCount); summary != "" {
		lines = append(lines, summary)
	}
	grouped := groupActionPlanNotesByCategory(normalized)
	lines = appendActionPlanNoteGroup(lines, "执行动作：", grouped[actionPlanNoteCategoryMutation])
	lines = appendActionPlanNoteGroup(lines, "查询结果：", grouped[actionPlanNoteCategoryQuery])
	lines = appendActionPlanNoteGroup(lines, "异常记录：", grouped[actionPlanNoteCategoryError])
	lines = appendActionPlanNoteGroup(lines, "补充信息：", grouped[actionPlanNoteCategoryUnknown])
	if !hasNextStep {
		if status == "partial" || status == "failed" {
			lines = append(lines, nextStepLine(nextStepContinueFromFailures))
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func mergeQueryOnlyActionPlanNotes(notes []renderedActionPlanNote) []string {
	lines := make([]string, 0, len(notes)*6)
	seenExecSource := map[string]struct{}{}
	seenConclusion := map[string]struct{}{}
	seenStatus := map[string]struct{}{}
	evidenceItems := make([]string, 0, len(notes)*3)
	seenEvidence := map[string]struct{}{}
	lastNextStep := ""
	for _, note := range notes {
		text := strings.ReplaceAll(note.Text, "\r\n", "\n")
		inEvidence := false
		for _, rawLine := range strings.Split(text, "\n") {
			line := strings.TrimSpace(rawLine)
			if line == "" {
				continue
			}
			lower := strings.ToLower(line)
			if strings.HasPrefix(lower, "关键证据：") || strings.HasPrefix(lower, "关键证据:") {
				evidenceValue := strings.TrimSpace(strings.TrimPrefix(line, "关键证据："))
				evidenceValue = strings.TrimSpace(strings.TrimPrefix(evidenceValue, "关键证据:"))
				if evidenceValue != "" {
					appendQueryOnlyEvidenceItem(&evidenceItems, seenEvidence, evidenceValue)
				}
				inEvidence = true
				continue
			}
			if inEvidence {
				if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "• ") {
					evidenceValue := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "- "), "• "))
					appendQueryOnlyEvidenceItem(&evidenceItems, seenEvidence, evidenceValue)
					continue
				}
				inEvidence = false
			}
			if strings.HasPrefix(lower, "下一步：") || strings.HasPrefix(lower, "下一步:") {
				if normalized := nextStepLine(line); normalized != "" {
					lastNextStep = normalized
				}
				continue
			}
			if strings.HasPrefix(lower, "next step:") {
				if normalized := nextStepLine(strings.TrimSpace(strings.TrimPrefix(line, "next step:"))); normalized != "" {
					lastNextStep = normalized
				}
				continue
			}
			if strings.HasPrefix(line, "执行来源：") {
				if _, exists := seenExecSource[line]; exists {
					continue
				}
				seenExecSource[line] = struct{}{}
			}
			if strings.HasPrefix(lower, "结论：") || strings.HasPrefix(lower, "结论:") {
				if _, exists := seenConclusion[line]; exists {
					continue
				}
				seenConclusion[line] = struct{}{}
			}
			if strings.HasPrefix(lower, "完成状态：") || strings.HasPrefix(lower, "完成状态:") {
				if _, exists := seenStatus[line]; exists {
					continue
				}
				seenStatus[line] = struct{}{}
			}
			lines = append(lines, line)
		}
	}
	lines = dedupeConsecutiveActionPlanLines(lines)
	if len(evidenceItems) > 0 {
		lines = append(lines, "关键证据：")
		for _, item := range evidenceItems {
			lines = append(lines, "- "+item)
		}
	}
	if lastNextStep != "" {
		if len(lines) == 0 || lines[len(lines)-1] != lastNextStep {
			lines = append(lines, lastNextStep)
		}
	}
	return lines
}

func appendQueryOnlyEvidenceItem(items *[]string, seen map[string]struct{}, item string) {
	item = strings.TrimSpace(item)
	if item == "" {
		return
	}
	if _, exists := seen[item]; exists {
		return
	}
	seen[item] = struct{}{}
	*items = append(*items, item)
}

// Query overview selects the highest-confidence source when the same field appears multiple times.
const (
	queryOverviewPriorityHumanizedTask          = 20
	queryOverviewPriorityHumanizedRelease       = 20
	queryOverviewPriorityGenericToken           = 30
	queryOverviewPriorityRuntimeDelegates       = 40
	queryOverviewPriorityServiceStatusNarrative = 40
	queryOverviewPriorityRuntimeTaskControl     = 50
	queryOverviewPriorityRuntimeTaskStatus      = 60
	queryOverviewPriorityRuntimeServiceStatus   = 60
	queryOverviewPriorityRuntimeReleaseStatus   = 70
)

func buildQueryOnlyOverviewLine(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	queryCount := 0
	firstConclusion := ""
	overviewFields := collectQueryOnlyOverviewFields(lines)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "查询结果：") ||
			strings.HasPrefix(lower, "查询结果:") ||
			strings.HasPrefix(lower, "query result:") {
			queryCount++
			continue
		}
		if firstConclusion == "" && (strings.HasPrefix(lower, "结论：") || strings.HasPrefix(lower, "结论:")) {
			firstConclusion = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmed, "结论："), "结论:"))
		}
	}
	if queryCount <= 1 {
		return ""
	}
	if len(overviewFields) > 0 {
		return "查询摘要：" + strings.Join(overviewFields, " ")
	}
	if firstConclusion != "" {
		return "查询摘要：" + firstConclusion
	}
	return fmt.Sprintf("查询摘要：已返回 %d 项查询结果。", queryCount)
}

func collectQueryOnlyOverviewFields(lines []string) []string {
	type fieldCandidate struct {
		Value string
		Score int
	}
	var (
		agentCandidate         fieldCandidate
		serviceCandidate       fieldCandidate
		versionCandidate       fieldCandidate
		taskStatusCandidate    fieldCandidate
		releaseStatusCandidate fieldCandidate
		serviceActiveCandidate fieldCandidate
	)
	setCandidate := func(candidate *fieldCandidate, value string, score int) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if candidate.Value == "" || score >= candidate.Score {
			candidate.Value = value
			candidate.Score = score
		}
	}
	setAgent := func(raw string, score int) {
		setCandidate(&agentCandidate, normalizeQueryOverviewToken(raw), score)
	}
	setService := func(raw string, score int) {
		setCandidate(&serviceCandidate, normalizeQueryOverviewToken(raw), score)
	}
	setVersion := func(raw string, score int) {
		setCandidate(&versionCandidate, normalizeQueryOverviewToken(raw), score)
	}
	setTaskStatus := func(raw string, score int) {
		status := normalizeTaskStatusValue(normalizeQueryOverviewToken(raw))
		setCandidate(&taskStatusCandidate, status, score)
	}
	setReleaseStatus := func(raw string, score int) {
		setCandidate(&releaseStatusCandidate, normalizeReleaseStatusValue(normalizeQueryOverviewToken(raw)), score)
	}
	setServiceActive := func(raw string, score int) {
		active := normalizeManagedServiceStateToken(normalizeQueryOverviewToken(raw))
		setCandidate(&serviceActiveCandidate, active, score)
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		// Generic token extraction as baseline; higher-priority sources can override.
		setAgent(extractActionPlanTokenValueExact(line, "agent"), queryOverviewPriorityGenericToken)
		setService(extractActionPlanTokenValueExact(line, "service"), queryOverviewPriorityGenericToken)
		setVersion(extractActionPlanTokenValueExact(line, "version"), queryOverviewPriorityGenericToken)
		setTaskStatus(extractActionPlanTokenValueExact(line, "task_status"), queryOverviewPriorityGenericToken)
		setServiceActive(extractActionPlanTokenValueExact(line, "active"), queryOverviewPriorityGenericToken)

		if strings.HasPrefix(lower, "查询结果：runtime.task.status") {
			setAgent(extractActionPlanTokenValueExact(line, "agent"), queryOverviewPriorityRuntimeTaskStatus)
			setTaskStatus(extractActionPlanTokenValueExact(line, "status"), queryOverviewPriorityRuntimeTaskStatus)
		}
		if strings.HasPrefix(lower, "查询结果：runtime.task.delegates") {
			setAgent(extractActionPlanTokenValueExact(line, "agent"), queryOverviewPriorityRuntimeDelegates)
		}
		if strings.HasPrefix(lower, "查询结果：runtime.task.control 状态") {
			setAgent(extractActionPlanTokenValueExact(line, "agent"), queryOverviewPriorityRuntimeTaskControl)
			setService(extractActionPlanTokenValueExact(line, "service"), queryOverviewPriorityRuntimeTaskControl)
			setTaskStatus(extractActionPlanTokenValueExact(line, "task_status"), queryOverviewPriorityRuntimeTaskControl)
			setServiceActive(extractActionPlanTokenValueExact(line, "active"), queryOverviewPriorityRuntimeTaskControl)
		}
		if strings.HasPrefix(lower, "runtime.service 状态：") ||
			strings.HasPrefix(lower, "runtime.service status:") ||
			strings.HasPrefix(lower, "查询结果：runtime.service 状态：") ||
			strings.HasPrefix(lower, "query result:runtime.service status:") ||
			strings.HasPrefix(lower, "query result: runtime.service status:") {
			setService(extractActionPlanTokenValueExact(line, "service"), queryOverviewPriorityRuntimeServiceStatus)
			setServiceActive(extractActionPlanTokenValueExact(line, "active"), queryOverviewPriorityRuntimeServiceStatus)
		}
		if strings.HasPrefix(lower, "查询结果：runtime.release.status") {
			setService(extractActionPlanTokenValueExact(line, "service"), queryOverviewPriorityRuntimeReleaseStatus)
			setVersion(extractActionPlanTokenValueExact(line, "version"), queryOverviewPriorityRuntimeReleaseStatus)
			setReleaseStatus(extractActionPlanTokenValueExact(line, "status"), queryOverviewPriorityRuntimeReleaseStatus)
		}

		if agent, status, ok := parseHumanizedTaskStatusQueryLine(line); ok {
			setAgent(agent, queryOverviewPriorityHumanizedTask)
			setTaskStatus(status, queryOverviewPriorityHumanizedTask)
		}
		if service, version, ok := parseHumanizedReleaseCurrentQueryLine(line); ok {
			setService(service, queryOverviewPriorityHumanizedRelease)
			setVersion(version, queryOverviewPriorityHumanizedRelease)
		}
		if service, status, ok := parseHumanizedReleaseStatusQueryLine(line); ok {
			setService(service, queryOverviewPriorityHumanizedRelease)
			setReleaseStatus(status, queryOverviewPriorityHumanizedRelease)
		}
		if service, ok := parseHumanizedReleaseNoneQueryLine(line); ok {
			setService(service, queryOverviewPriorityHumanizedRelease)
			setReleaseStatus("none", queryOverviewPriorityHumanizedRelease)
		}
		if strings.Contains(lower, "服务状态：active=") {
			setServiceActive(extractActionPlanTokenValue(line, "active="), queryOverviewPriorityServiceStatusNarrative)
		}
		if strings.Contains(lower, "当前版本是") {
			setReleaseStatus(extractActionPlanTokenValueExact(line, "status"), queryOverviewPriorityHumanizedRelease)
		}
	}
	if strings.TrimSpace(agentCandidate.Value) == "" &&
		strings.TrimSpace(serviceCandidate.Value) == "" &&
		strings.TrimSpace(versionCandidate.Value) == "" &&
		strings.TrimSpace(taskStatusCandidate.Value) == "" &&
		strings.TrimSpace(releaseStatusCandidate.Value) == "" &&
		strings.TrimSpace(serviceActiveCandidate.Value) == "" {
		return nil
	}
	return []string{
		"agent=" + fallbackValue(agentCandidate.Value, "-"),
		"service=" + fallbackValue(serviceCandidate.Value, "-"),
		"version=" + fallbackValue(versionCandidate.Value, "-"),
		"task_status=" + fallbackValue(taskStatusCandidate.Value, "-"),
		"release_status=" + fallbackValue(releaseStatusCandidate.Value, "-"),
		"service_active=" + fallbackValue(serviceActiveCandidate.Value, "-"),
	}
}

func normalizeQueryOverviewToken(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, "`'\"，,。.!！?？;；()（）[]【】")
	return strings.TrimSpace(raw)
}

func parseHumanizedTaskStatusQueryLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	prefix := "查询结果："
	needle := " 当前任务状态为 "
	if !strings.HasPrefix(line, prefix) {
		return "", "", false
	}
	start := len(prefix)
	idx := strings.Index(line[start:], needle)
	if idx < 0 {
		return "", "", false
	}
	agent := strings.TrimSpace(line[start : start+idx])
	rest := strings.TrimSpace(line[start+idx+len(needle):])
	if agent == "" || rest == "" {
		return "", "", false
	}
	cut := len(rest)
	for _, marker := range []string{"。", "；", "(", "（", " "} {
		if p := strings.Index(rest, marker); p >= 0 && p < cut {
			cut = p
		}
	}
	status := strings.TrimSpace(rest[:cut])
	if status == "" {
		return "", "", false
	}
	return agent, status, true
}

func parseHumanizedReleaseCurrentQueryLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	prefix := "查询结果："
	needle := " 当前版本是 "
	if !strings.HasPrefix(line, prefix) {
		return "", "", false
	}
	start := len(prefix)
	idx := strings.Index(line[start:], needle)
	if idx < 0 {
		return "", "", false
	}
	service := strings.TrimSpace(line[start : start+idx])
	rest := strings.TrimSpace(line[start+idx+len(needle):])
	if service == "" || rest == "" {
		return "", "", false
	}
	cut := len(rest)
	for _, marker := range []string{"。", "；", "(", "（", " "} {
		if p := strings.Index(rest, marker); p >= 0 && p < cut {
			cut = p
		}
	}
	version := strings.TrimSpace(rest[:cut])
	if version == "" {
		return "", "", false
	}
	return service, version, true
}

func parseHumanizedReleaseStatusQueryLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	prefix := "查询结果："
	needle := " 当前版本是 "
	if !strings.HasPrefix(line, prefix) {
		return "", "", false
	}
	start := len(prefix)
	idx := strings.Index(line[start:], needle)
	if idx < 0 {
		return "", "", false
	}
	service := strings.TrimSpace(line[start : start+idx])
	rest := strings.TrimSpace(line[start+idx+len(needle):])
	if service == "" || rest == "" {
		return "", "", false
	}
	if status := strings.TrimSpace(extractActionPlanTokenValueExact(rest, "status")); status != "" {
		return service, status, true
	}
	anchorIdx := strings.Index(rest, "状态：")
	anchorLen := len("状态：")
	if anchorIdx < 0 {
		anchorIdx = strings.Index(strings.ToLower(rest), "status:")
		anchorLen = len("status:")
	}
	if anchorIdx < 0 {
		return "", "", false
	}
	statusPart := strings.TrimSpace(rest[anchorIdx+anchorLen:])
	if statusPart == "" {
		return "", "", false
	}
	cut := len(statusPart)
	for _, marker := range []string{"，", ",", "）", ")", "。", "；", ";", " "} {
		if p := strings.Index(statusPart, marker); p >= 0 && p < cut {
			cut = p
		}
	}
	status := strings.TrimSpace(statusPart[:cut])
	if status == "" {
		return "", "", false
	}
	return service, status, true
}

func parseHumanizedReleaseNoneQueryLine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	prefix := "查询结果："
	needle := " 当前还没有发布版本记录"
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	start := len(prefix)
	idx := strings.Index(line[start:], needle)
	if idx < 0 {
		return "", false
	}
	service := strings.TrimSpace(line[start : start+idx])
	if service == "" {
		return "", false
	}
	return service, true
}

func dedupeConsecutiveActionPlanLines(lines []string) []string {
	if len(lines) <= 1 {
		return lines
	}
	out := make([]string, 0, len(lines))
	last := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == last {
			continue
		}
		out = append(out, line)
		last = line
	}
	return out
}

func groupActionPlanNotesByCategory(notes []renderedActionPlanNote) map[actionPlanNoteCategory][]string {
	grouped := map[actionPlanNoteCategory][]string{
		actionPlanNoteCategoryMutation: make([]string, 0, len(notes)),
		actionPlanNoteCategoryQuery:    make([]string, 0, len(notes)),
		actionPlanNoteCategoryError:    make([]string, 0, len(notes)),
		actionPlanNoteCategoryUnknown:  make([]string, 0, len(notes)),
	}
	for _, note := range notes {
		text := strings.TrimSpace(note.Text)
		if text == "" {
			continue
		}
		category := note.Category
		if category != actionPlanNoteCategoryQuery &&
			category != actionPlanNoteCategoryMutation &&
			category != actionPlanNoteCategoryError {
			category = actionPlanNoteCategoryUnknown
		}
		grouped[category] = append(grouped[category], text)
	}
	return grouped
}

func appendActionPlanNoteGroup(lines []string, title string, notes []string) []string {
	if len(notes) == 0 {
		return lines
	}
	lines = append(lines, title)
	for _, note := range notes {
		note = strings.TrimSpace(note)
		if note == "" {
			continue
		}
		lines = append(lines, "- "+note)
	}
	return lines
}

func appendRuntimeSummaryLine(lines []string, summary string) []string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return lines
	}
	return append(lines, "摘要："+summary)
}

func appendRuntimeEvidenceSection(lines []string, items []string) []string {
	if len(items) == 0 {
		return lines
	}
	cleaned := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item = normalizeActionPlanNote(item)
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		cleaned = append(cleaned, item)
	}
	if len(cleaned) == 0 {
		return lines
	}
	lines = append(lines, "关键证据：")
	for _, item := range cleaned {
		lines = append(lines, "- "+item)
	}
	return lines
}

func nextStepLine(detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return ""
	}
	detail = strings.TrimPrefix(detail, "下一步：")
	detail = strings.TrimPrefix(detail, "下一步:")
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return ""
	}
	return "下一步：" + detail
}

func actionPlanNoteContainsNextStep(note string) bool {
	lower := strings.ToLower(strings.TrimSpace(note))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "下一步：") ||
		strings.Contains(lower, "下一步:") ||
		strings.Contains(lower, "next step:")
}

func humanizeActionPlanNote(note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return ""
	}
	if humanized, ok := humanizeSingleActionPlanNote(note); ok {
		return humanized
	}
	return note
}

func isQueryLikeActionPlanNote(note string) bool {
	note = strings.TrimSpace(strings.ToLower(note))
	if note == "" {
		return false
	}
	return strings.HasPrefix(note, "查询结果：") ||
		strings.HasPrefix(note, "查询结果:") ||
		strings.HasPrefix(note, "query result:")
}

func classifyActionPlanNote(note string) actionPlanNoteCategory {
	note = strings.TrimSpace(note)
	if note == "" {
		return actionPlanNoteCategoryUnknown
	}
	if isQueryLikeActionPlanNote(note) {
		return actionPlanNoteCategoryQuery
	}
	lower := strings.ToLower(note)
	if strings.Contains(lower, "执行失败") ||
		strings.HasPrefix(lower, "spec_kit.gate：") ||
		strings.HasPrefix(lower, "spec_kit.gate:") ||
		strings.HasPrefix(lower, "执行异常：") ||
		strings.HasPrefix(lower, "失败摘要：") ||
		strings.HasPrefix(lower, "未支持的 action.kind") ||
		strings.HasPrefix(lower, "action_plan 未包含 actions") ||
		(strings.Contains(lower, "缺少") && strings.Contains(lower, "已跳过")) {
		return actionPlanNoteCategoryError
	}
	if strings.HasPrefix(lower, "runtime.exec 结果：") ||
		strings.HasPrefix(lower, "runtime_exec_decision 已生效：") ||
		strings.HasPrefix(lower, "runtime_exec_decision 深修兜底：") ||
		strings.HasPrefix(lower, "runtime_exec_decision 构建兜底：") ||
		strings.HasPrefix(lower, "runtime 初始化完成：") ||
		strings.HasPrefix(lower, "config.exec 完成：") ||
		strings.Contains(lower, " 完成：") {
		return actionPlanNoteCategoryMutation
	}
	return actionPlanNoteCategoryUnknown
}

func formatActionPlanNotesOverview(queryCount, mutationCount, errorCount int) string {
	parts := make([]string, 0, 3)
	if mutationCount > 0 {
		parts = append(parts, fmt.Sprintf("变更 %d 项", mutationCount))
	}
	if queryCount > 0 {
		parts = append(parts, fmt.Sprintf("查询 %d 项", queryCount))
	}
	if errorCount > 0 {
		parts = append(parts, fmt.Sprintf("异常 %d 项", errorCount))
	}
	if len(parts) == 0 {
		return ""
	}
	return "结果概览：" + strings.Join(parts, "，") + "。"
}

func normalizeActionPlanNote(note string) string {
	note = strings.TrimSpace(note)
	note = strings.TrimPrefix(note, "- ")
	note = strings.TrimPrefix(note, "• ")
	return strings.TrimSpace(note)
}

func isSyntheticQueryPlanReason(reason string) bool {
	reason = strings.TrimSpace(strings.ToLower(reason))
	if reason == "" {
		return false
	}
	return strings.HasPrefix(reason, "auto_release_status_query") ||
		strings.HasPrefix(reason, "auto_task_delegates_query") ||
		strings.HasPrefix(reason, "auto_task_status_query") ||
		strings.HasPrefix(reason, "auto_status_truth_query") ||
		strings.HasPrefix(reason, "auto_task_control_query") ||
		strings.HasPrefix(reason, "auto_task_control_execute")
}

func humanizeSingleActionPlanNote(note string) (string, bool) {
	note = strings.TrimSpace(note)
	if note == "" {
		return "", false
	}
	lower := strings.ToLower(note)
	switch {
	case strings.HasPrefix(note, "spec_kit.gate："), strings.HasPrefix(lower, "spec_kit.gate:"):
		return humanizeSpecKitGateNote(note), true
	case strings.HasPrefix(note, "runtime_exec_decision 已生效："),
		strings.HasPrefix(note, "runtime_exec_decision 深修兜底："),
		strings.HasPrefix(note, "runtime_exec_decision 构建兜底："):
		return humanizeRuntimeExecDecisionNote(note), true
	case strings.HasPrefix(note, "runtime.release.status："), strings.HasPrefix(lower, "runtime.release.status:"):
		return humanizeRuntimeReleaseStatusNote(note), true
	case strings.HasPrefix(note, "runtime.release 完成："):
		return humanizeRuntimeReleaseMutationNote(note), true
	case looksLikeRuntimeExecResponseNote(note):
		return humanizeRuntimeExecNote(note), true
	case strings.HasPrefix(note, "runtime.service 完成："):
		return humanizeRuntimeServiceMutationNote(note), true
	case strings.HasPrefix(note, "runtime.service 状态："):
		return humanizeRuntimeServiceStatusNote(note), true
	case strings.HasPrefix(note, "runtime.supervisor 完成："):
		return humanizeRuntimeSupervisorMutationNote(note), true
	case strings.HasPrefix(note, "runtime.supervisor 状态："):
		return humanizeRuntimeSupervisorStatusNote(note), true
	case looksLikeConfigExecResponseNote(note):
		return humanizeConfigExecNote(note), true
	case strings.HasPrefix(note, "查询结果：runtime.task.delegates"):
		return humanizeRuntimeTaskDelegatesQueryNote(note), true
	case strings.HasPrefix(note, "查询结果：runtime.task.control 状态"):
		return humanizeRuntimeTaskControlStatusNote(note), true
	case strings.HasPrefix(note, "查询结果：") && strings.Contains(note, " 当前任务状态为 "):
		return humanizeRuntimeTaskStatusQueryNote(note), true
	case strings.HasPrefix(note, "runtime.task.delegate 完成："):
		return humanizeRuntimeTaskDelegateMutationNote(note), true
	case strings.HasPrefix(note, "runtime.task.retry 完成："):
		return humanizeRuntimeTaskRetryMutationNote(note), true
	case strings.HasPrefix(note, "runtime.task.cancel 完成："):
		return humanizeRuntimeTaskCancelMutationNote(note), true
	case strings.HasPrefix(note, "runtime.task.control 完成："):
		return humanizeRuntimeTaskControlMutationNote(note), true
	case looksLikeActionPlanErrorNote(note):
		return humanizeActionPlanErrorNote(note), true
	default:
		return "", false
	}
}

func looksLikeActionPlanErrorNote(note string) bool {
	note = strings.TrimSpace(note)
	if note == "" {
		return false
	}
	lower := strings.ToLower(note)
	return strings.Contains(lower, "执行失败:") ||
		strings.HasPrefix(lower, "未支持的 action.kind:") ||
		strings.HasPrefix(lower, "action_plan 未包含 actions") ||
		(strings.Contains(lower, "缺少") && strings.Contains(lower, "已跳过")) ||
		strings.Contains(lower, "未被处理:")
}

func humanizeActionPlanErrorNote(note string) string {
	note = normalizeActionPlanNote(note)
	source := extractActionPlanErrorSource(note)
	summary := extractActionPlanErrorSummary(note)
	execSource := extractActionPlanInlineExecutionSource(note)
	summary = strings.TrimSpace(strings.ReplaceAll(summary, execSource, ""))
	summary = strings.TrimSpace(strings.Trim(summary, "；;,\n"))
	recovery := deriveActionPlanErrorRecovery(source, note)
	if source == "" {
		source = "action_plan"
	}
	if summary == "" {
		summary = summarizeText(note, 220)
	}
	commandHint := deriveActionPlanErrorCommandHint(source)
	lines := []string{
		fmt.Sprintf("执行异常：%s。", source),
		"结论：" + buildActionPlanErrorConclusion(source),
		"完成状态：失败。",
		"失败摘要：" + summary,
	}
	if execSource != "" {
		lines = append(lines, execSource)
	}
	if commandHint != "" {
		lines = append(lines, "建议命令："+commandHint)
	}
	lines = appendRuntimeEvidenceSection(lines, []string{"原始回执：" + note})
	lines = append(lines, "恢复动作："+recovery)
	lines = append(lines, nextStepLine(nextStepRetryFromRecovery))
	return strings.Join(lines, "\n")
}

func extractActionPlanErrorSource(note string) string {
	note = strings.TrimSpace(note)
	switch {
	case strings.HasPrefix(note, "未支持的 action.kind:"):
		return "action.kind"
	case strings.HasPrefix(note, "action_plan 未包含 actions"):
		return "action_plan"
	}
	markers := []string{" 执行失败:", " 缺少", " 未被处理:"}
	for _, marker := range markers {
		if idx := strings.Index(note, marker); idx > 0 {
			return strings.TrimSpace(note[:idx])
		}
	}
	return ""
}

func extractActionPlanErrorSummary(note string) string {
	note = strings.TrimSpace(note)
	switch {
	case strings.HasPrefix(note, "未支持的 action.kind:"):
		return "未支持的 action.kind：" + strings.TrimSpace(strings.TrimPrefix(note, "未支持的 action.kind:"))
	case strings.HasPrefix(note, "action_plan 未包含 actions"):
		return "action_plan 缺少 actions，无法执行。"
	}
	if idx := strings.Index(note, ":"); idx >= 0 && idx+1 < len(note) {
		return strings.TrimSpace(note[idx+1:])
	}
	return summarizeText(note, 220)
}

func deriveActionPlanErrorRecovery(source string, note string) string {
	source = strings.TrimSpace(strings.ToLower(source))
	note = strings.TrimSpace(strings.ToLower(note))
	switch {
	case strings.HasPrefix(source, "runtime.bootstrap"):
		return "先确认 workspace 在 allowed_roots 范围内，再重试。"
	case strings.HasPrefix(source, "runtime.release.status"):
		return "先核对 service/operation 参数，再执行 runtime.release.status current 或 history。"
	case strings.HasPrefix(source, "runtime.task.control"):
		return "先执行 runtime.task.control status 查看任务与服务状态，再按结果执行 ensure_running 或 restart。"
	case strings.HasPrefix(source, "runtime.supervisor"):
		return "先执行 runtime.supervisor status 查看 unit 状态，再按结果执行 ensure/disable 或 runtime.service restart。"
	case strings.HasPrefix(source, "runtime.release"):
		return "先执行 runtime.release.status current 确认当前版本，再决定重试发布或回滚。"
	case strings.HasPrefix(source, "runtime.service"):
		return "先执行 runtime.service status 确认现状，再重试变更操作。"
	case strings.HasPrefix(source, "runtime.task.status"), strings.HasPrefix(source, "runtime.task.delegate"), strings.HasPrefix(source, "runtime.task.delegates"), strings.HasPrefix(source, "runtime.task.retry"), strings.HasPrefix(source, "runtime.task.cancel"):
		return "先核对 task_id/agent 参数并查询任务状态，再重试。"
	case strings.HasPrefix(source, "config.exec"):
		return "先用 `/config help` 或 `/config show` 核对配置状态，再重试。"
	case strings.HasPrefix(source, "agent.use"):
		return "先确认目标 agent_id 存在且可用。"
	case strings.HasPrefix(source, "requirement.sync"):
		return "先补全 requirement 内容，再指定目标 agent 重试。"
	case strings.HasPrefix(source, "action.kind"):
		return "先改为支持的 action.kind，再执行。"
	case strings.HasPrefix(source, "action_plan"):
		return "先补全 action_plan.actions 后再执行。"
	case strings.Contains(note, "缺少") && strings.Contains(note, "已跳过"):
		return "先补全缺失参数，再重试该动作。"
	default:
		return "先修复上述错误后再续跑。"
	}
}

func buildActionPlanErrorConclusion(source string) string {
	source = strings.TrimSpace(strings.ToLower(source))
	switch {
	case strings.HasPrefix(source, "runtime.task.control"):
		return "worker 控制动作失败，任务可能继续阻塞。"
	case strings.HasPrefix(source, "runtime.supervisor"):
		return "supervisor 操作失败，unit 状态可能异常。"
	case strings.HasPrefix(source, "runtime.release"):
		return "发布相关动作失败，当前版本状态可能未按预期变更。"
	case strings.HasPrefix(source, "runtime.service"):
		return "服务操作失败，服务状态可能未按预期变更。"
	case strings.HasPrefix(source, "runtime.task.status"),
		strings.HasPrefix(source, "runtime.task.delegate"),
		strings.HasPrefix(source, "runtime.task.delegates"),
		strings.HasPrefix(source, "runtime.task.retry"),
		strings.HasPrefix(source, "runtime.task.cancel"):
		return "任务中心动作失败，任务进度未推进。"
	case strings.HasPrefix(source, "config.exec"):
		return "配置命令执行失败，配置未生效。"
	case strings.HasPrefix(source, "requirement.sync"):
		return "需求同步失败，本轮需求变更未落盘。"
	case strings.HasPrefix(source, "agent.use"):
		return "agent 切换失败，仍在当前 agent 上下文。"
	case strings.HasPrefix(source, "action.kind"):
		return "action_plan 含不支持的动作类型，无法执行。"
	case strings.HasPrefix(source, "action_plan"):
		return "action_plan 结构不完整，无法继续执行。"
	default:
		return "本次动作执行失败，需先处理错误后再续跑。"
	}
}

func deriveActionPlanErrorCommandHint(source string) string {
	source = strings.TrimSpace(strings.ToLower(source))
	switch {
	case strings.HasPrefix(source, "runtime.task.control"):
		return "`runtime.task.control operation=status`"
	case strings.HasPrefix(source, "runtime.supervisor"):
		return "`runtime.supervisor operation=status`"
	case strings.HasPrefix(source, "runtime.release.status"):
		return "`runtime.release.status operation=current`"
	case strings.HasPrefix(source, "runtime.release"):
		return "`runtime.release.status operation=current`"
	case strings.HasPrefix(source, "runtime.service"):
		return "`runtime.service operation=status`"
	case strings.HasPrefix(source, "runtime.task.status"),
		strings.HasPrefix(source, "runtime.task.delegate"),
		strings.HasPrefix(source, "runtime.task.delegates"),
		strings.HasPrefix(source, "runtime.task.retry"),
		strings.HasPrefix(source, "runtime.task.cancel"):
		return "`runtime.task.status`"
	case strings.HasPrefix(source, "config.exec"):
		return "`/config show`"
	default:
		return ""
	}
}

func humanizeSpecKitGateNote(note string) string {
	note = strings.TrimSpace(note)
	missingRaw := strings.TrimSpace(extractActionPlanTokenValueExact(note, "missing"))
	specDir := strings.TrimSpace(extractActionPlanTokenValueExact(note, "dir"))

	missing := normalizeSpecKitMissingDocs(strings.Split(missingRaw, "|"))
	if len(missing) == 0 {
		if legacy := strings.TrimSpace(extractActionPlanSegmentValue(note, "缺少：")); legacy != "" {
			missing = normalizeSpecKitMissingDocs(strings.FieldsFunc(legacy, func(r rune) bool {
				return r == ',' || r == '，'
			}))
		}
	}
	if specDir == "" {
		if legacyDir := strings.TrimSpace(extractActionPlanSegmentValue(note, "目录：")); legacyDir != "" {
			specDir = legacyDir
		}
	}

	missingCSV := "未知"
	missingStep := "必需文档"
	missingContinue := "必需文档"
	if len(missing) > 0 {
		missingCSV = strings.Join(missing, ", ")
		missingStep = strings.Join(missing, "、")
		missingContinue = strings.Join(missing, " 和 ")
	}

	head := "Spec Kit 文档未完整，缺少：" + missingCSV
	if specDir != "" {
		head = fmt.Sprintf("Spec Kit 文档未完整（目录：%s），缺少：%s", specDir, missingCSV)
	}

	lines := []string{
		head,
		"结论：当前暂不进入实现阶段。",
		"完成状态：阻塞（待补文档）。",
	}
	lines = appendRuntimeSummaryLine(lines, "Spec Kit 文档未通过完整性检查。")
	evidence := []string{"缺少：" + missingCSV}
	if specDir != "" {
		evidence = append(evidence, "目录："+specDir)
	}
	evidence = append(evidence, "原始回执："+note)
	lines = appendRuntimeEvidenceSection(lines, evidence)
	lines = append(lines, nextStepLine(fmt.Sprintf("请先补齐 %s，再进入实现阶段。", missingStep)))
	lines = append(lines, fmt.Sprintf("你可以直接回复：继续补齐 %s", missingContinue))
	return strings.Join(lines, "\n")
}

func formatSpecKitGateNote(specDir string, missing []string) string {
	missing = normalizeSpecKitMissingDocs(missing)
	missingToken := strings.Join(missing, "|")
	dirToken := strings.TrimSpace(specDir)
	if dirToken == "" {
		return fmt.Sprintf("spec_kit.gate：missing=%s", missingToken)
	}
	return fmt.Sprintf("spec_kit.gate：missing=%s dir=%s", missingToken, dirToken)
}

func normalizeSpecKitMissingDocs(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item = strings.TrimSpace(strings.Trim(item, "。;；"))
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func humanizeRuntimeExecDecisionNote(note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return ""
	}
	mode := normalizeRuntimeExecDecisionMode(extractActionPlanTokenValueExact(note, "mode"))
	source := strings.TrimSpace(extractActionPlanTokenValueExact(note, "source"))
	lockSource := strings.TrimSpace(extractActionPlanTokenValueExact(note, "lock_source"))
	fallbackSource := strings.TrimSpace(extractActionPlanTokenValueExact(note, "fallback_source"))

	if strings.HasPrefix(note, "runtime_exec_decision 深修兜底：") ||
		strings.HasPrefix(note, "runtime_exec_decision 构建兜底：") {
		if mode == "" {
			if strings.HasPrefix(note, "runtime_exec_decision 深修兜底：") {
				mode = "service.deep_repair"
			} else {
				mode = "build.continue"
			}
		}
		lines := []string{
			note,
			"结论：已注入兜底诊断命令，用于补充定位上下文。",
			"完成状态：已注入兜底诊断。",
			"策略模式：" + mapRuntimeExecDecisionModeLabel(mode) + "。",
		}
		evidence := []string{"原始回执：" + note}
		if fallbackSource != "" {
			evidence = append(evidence, "fallback_source="+fallbackSource)
		}
		lines = appendRuntimeEvidenceSection(lines, evidence)
		lines = append(lines, nextStepLine("等待本轮执行完成后按证据继续处理。"))
		return strings.Join(lines, "\n")
	}

	lines := []string{
		note,
		"结论：" + buildRuntimeExecDecisionMutationConclusion(mode, note),
		"完成状态：" + mapRuntimeExecDecisionMutationStatus(mode, note) + "。",
		"策略模式：" + mapRuntimeExecDecisionModeLabel(mode) + "。",
	}
	if source != "" {
		sourceLine := source
		if lockSource != "" {
			sourceLine = sourceLine + " lock_source=" + lockSource
		}
		lines = append(lines, "决策来源："+sourceLine+"。")
	}
	evidence := []string{"原始回执：" + note}
	if mode != "" {
		evidence = append(evidence, "mode="+mode)
	}
	if source != "" {
		evidence = append(evidence, "source="+source)
	}
	lines = appendRuntimeEvidenceSection(lines, evidence)
	lines = append(lines, nextStepLine(buildRuntimeExecDecisionMutationNextStep(mode)))
	return strings.Join(lines, "\n")
}

func mapRuntimeExecDecisionModeLabel(mode string) string {
	switch normalizeRuntimeExecDecisionMode(mode) {
	case "paused":
		return "暂停自动执行"
	case "blocked.command_override":
		return "替代命令覆盖"
	case "service.retry_only":
		return "仅重试"
	case "build.switch_source":
		return "切换依赖源"
	case "service.deep_repair":
		return "服务深修"
	case "build.continue":
		return "构建持续修复"
	case "clear", "", "none":
		return "默认自治"
	default:
		return "策略已更新"
	}
}

func buildRuntimeExecDecisionMutationConclusion(mode string, note string) string {
	mode = normalizeRuntimeExecDecisionMode(mode)
	switch mode {
	case "clear":
		return "已清除策略锁定，后续按默认自治执行。"
	case "blocked.command_override":
		return "已按你的替代命令覆盖执行，本轮优先执行你指定的命令。"
	case "service.retry_only":
		if strings.Contains(note, "已跳过") {
			return "已收敛为仅重试策略，非重试动作已跳过。"
		}
		return "已启用仅重试策略，后续动作将优先执行重试链路。"
	case "build.switch_source":
		if strings.Contains(note, "未识别到可改写") {
			return "切换依赖源策略已生效，但本轮未匹配到可改写命令。"
		}
		return "切换依赖源策略已生效，后续构建命令将优先使用镜像源。"
	case "service.deep_repair":
		return "已切换为服务深修策略，后续将优先执行诊断与修复链路。"
	case "build.continue":
		return "已切换为构建持续修复策略，后续将继续推进构建链修复。"
	case "paused":
		return "自动执行已暂停，等待你确认后继续。"
	default:
		if strings.Contains(note, "已生效") {
			return "执行决策已更新，后续动作将按新策略继续。"
		}
		return "执行决策已更新。"
	}
}

func mapRuntimeExecDecisionMutationStatus(mode string, note string) string {
	mode = normalizeRuntimeExecDecisionMode(mode)
	switch mode {
	case "clear":
		return "已清除策略锁定"
	case "paused":
		return "已暂停（等待确认）"
	case "blocked.command_override":
		return "已启用替代命令"
	case "service.retry_only":
		return "已切换为仅重试模式"
	case "build.switch_source":
		if strings.Contains(note, "未识别到可改写") {
			return "策略已应用（无命令改写）"
		}
		return "已切换依赖源策略"
	case "service.deep_repair":
		return "已切换为服务深修模式"
	case "build.continue":
		return "已切换为构建持续修复模式"
	default:
		return "已应用"
	}
}

func buildRuntimeExecDecisionMutationNextStep(mode string) string {
	switch normalizeRuntimeExecDecisionMode(mode) {
	case "clear":
		return "继续按默认自治执行；如需限定策略可再次说明。"
	case "paused":
		return "回复“继续”恢复自治，或直接发送新的执行指令。"
	case "blocked.command_override":
		return "等待替代命令执行结果；如需恢复默认策略可回复“清除策略”。"
	case "service.retry_only":
		return "等待重试链路完成后，再复查服务与任务状态。"
	case "build.switch_source":
		return "继续执行构建命令并观察依赖拉取结果。"
	case "service.deep_repair":
		return "继续执行深修链路并关注健康探针。"
	case "build.continue":
		return "继续执行构建修复链路并观察失败项是否收敛。"
	default:
		return "继续执行并观察回执。"
	}
}

func looksLikeRuntimeExecResponseNote(note string) bool {
	note = strings.TrimSpace(note)
	if note == "" || !strings.HasPrefix(note, "结论：") {
		return false
	}
	return strings.Contains(note, "证据：") ||
		strings.Contains(note, "关键证据：") ||
		strings.Contains(note, "exec_ids:")
}

func humanizeRuntimeExecNote(note string) string {
	lines := splitNonEmptyLines(note)
	if len(lines) == 0 {
		return note
	}
	conclusion := ""
	problem := ""
	execSource := ""
	keyEvidence := ""
	evidence := ""
	nextStep := ""
	artifacts := make([]string, 0, 4)
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "结论："):
			conclusion = line
		case strings.HasPrefix(line, "问题："):
			problem = line
		case strings.HasPrefix(line, "执行来源："):
			execSource = line
		case strings.HasPrefix(line, "关键证据："):
			keyEvidence = line
		case strings.HasPrefix(line, "证据："):
			evidence = line
		case strings.HasPrefix(line, "下一步："):
			nextStep = line
		case strings.HasPrefix(line, "产物："), strings.HasPrefix(line, "- "):
			artifacts = append(artifacts, line)
		}
	}

	conclusionSummary := strings.TrimSpace(strings.TrimPrefix(conclusion, "结论："))
	problemSummary := strings.TrimSpace(strings.TrimPrefix(problem, "问题："))
	if runtimeExecNoteLooksFailure(conclusionSummary, problemSummary) {
		return renderRuntimeExecErrorCard(
			conclusionSummary,
			problemSummary,
			execSource,
			keyEvidence,
			evidence,
			artifacts,
			nextStep,
		)
	}

	out := []string{"runtime.exec 结果："}
	switch {
	case conclusion != "":
		out = appendRuntimeSummaryLine(out, strings.TrimSpace(strings.TrimPrefix(conclusion, "结论：")))
	case problem != "":
		out = appendRuntimeSummaryLine(out, "执行存在问题："+strings.TrimSpace(strings.TrimPrefix(problem, "问题：")))
	default:
		out = appendRuntimeSummaryLine(out, "已返回本轮执行结果。")
	}
	out = append(out, "完成状态：已完成。")
	if conclusion != "" {
		out = append(out, conclusion)
	}
	if problem != "" {
		out = append(out, problem)
	}
	if execSource != "" {
		out = append(out, execSource)
	}
	evidenceItems := make([]string, 0, len(artifacts)+2)
	if keyEvidence != "" {
		evidenceItems = append(evidenceItems, keyEvidence)
	}
	if evidence != "" {
		evidenceItems = append(evidenceItems, evidence)
	}
	for _, artifact := range artifacts {
		evidenceItems = append(evidenceItems, artifact)
	}
	out = appendRuntimeEvidenceSection(out, evidenceItems)
	if nextStep != "" {
		out = append(out, nextStepLine(nextStep))
	} else {
		out = append(out, nextStepLine(nextStepContinueByRemaining))
	}
	return strings.Join(out, "\n")
}

func runtimeExecNoteLooksFailure(conclusionSummary string, problemSummary string) bool {
	if strings.TrimSpace(problemSummary) != "" {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(conclusionSummary))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "失败") ||
		strings.Contains(lower, "错误") ||
		strings.Contains(lower, "未完成")
}

func renderRuntimeExecErrorCard(
	conclusionSummary string,
	problemSummary string,
	execSource string,
	keyEvidence string,
	evidence string,
	artifacts []string,
	nextStep string,
) string {
	failureSummary := strings.TrimSpace(problemSummary)
	if failureSummary == "" {
		failureSummary = fallbackValue(strings.TrimSpace(conclusionSummary), "runtime.exec 执行失败。")
	}
	lines := []string{
		"执行异常：runtime.exec。",
		"结论：" + buildRuntimeExecErrorConclusion(conclusionSummary, problemSummary),
		"完成状态：" + mapRuntimeExecErrorStatus(conclusionSummary, problemSummary) + "。",
		"失败摘要：" + failureSummary,
	}
	if execSource != "" {
		lines = append(lines, execSource)
	}
	commandHint := deriveRuntimeExecErrorCommandHint(strings.Join([]string{
		problemSummary,
		keyEvidence,
		evidence,
		strings.Join(artifacts, " "),
	}, " "))
	if commandHint != "" {
		lines = append(lines, "建议命令："+commandHint)
	}
	evidenceItems := make([]string, 0, len(artifacts)+2)
	if keyEvidence != "" {
		evidenceItems = append(evidenceItems, keyEvidence)
	}
	if evidence != "" {
		evidenceItems = append(evidenceItems, evidence)
	}
	for _, artifact := range artifacts {
		evidenceItems = append(evidenceItems, artifact)
	}
	lines = appendRuntimeEvidenceSection(lines, evidenceItems)
	lines = append(lines, "恢复动作："+deriveRuntimeExecErrorRecovery(problemSummary, nextStep))
	if nextStep != "" {
		lines = append(lines, nextStepLine(nextStep))
	} else {
		lines = append(lines, nextStepLine(nextStepRetryFromRecovery))
	}
	return strings.Join(lines, "\n")
}

func buildRuntimeExecErrorConclusion(conclusionSummary string, problemSummary string) string {
	conclusionLower := strings.ToLower(strings.TrimSpace(conclusionSummary))
	problemLower := strings.ToLower(strings.TrimSpace(problemSummary))
	switch {
	case strings.Contains(conclusionLower, "成功") && strings.Contains(conclusionLower, "失败"):
		return "本轮命令执行部分失败，目标未完全达成。"
	case strings.Contains(conclusionLower, "都失败"):
		return "本轮命令执行全部失败，未产生有效结果。"
	case strings.Contains(problemLower, "timeout"), strings.Contains(problemLower, "network"), strings.Contains(problemLower, "name or service"):
		return "命令执行受网络/超时影响而失败。"
	default:
		return "本轮命令执行失败，需要先处理问题后重试。"
	}
}

func mapRuntimeExecErrorStatus(conclusionSummary string, problemSummary string) string {
	conclusionLower := strings.ToLower(strings.TrimSpace(conclusionSummary))
	problemLower := strings.ToLower(strings.TrimSpace(problemSummary))
	if strings.Contains(conclusionLower, "成功") && strings.Contains(conclusionLower, "失败") {
		return "部分失败"
	}
	if strings.Contains(problemLower, "partial") {
		return "部分失败"
	}
	return "失败"
}

func deriveRuntimeExecErrorCommandHint(context string) string {
	lower := strings.ToLower(strings.TrimSpace(context))
	switch {
	case strings.Contains(lower, "pip"), strings.Contains(lower, "pypi"), strings.Contains(lower, "name or service"), strings.Contains(lower, "timeout"), strings.Contains(lower, "network"):
		return "`runtime.exec cmd=\"curl -I --max-time 8 https://pypi.org/simple\"`"
	case strings.Contains(lower, "permission"), strings.Contains(lower, "denied"), strings.Contains(lower, "权限"):
		return "`runtime.exec cmd=\"pwd && ls -la\"`"
	case strings.Contains(lower, "python"), strings.Contains(lower, "venv"):
		return "`runtime.exec cmd=\"python3 --version && pip --version\"`"
	default:
		return "`runtime.task.status`"
	}
}

func deriveRuntimeExecErrorRecovery(problemSummary string, nextStep string) string {
	next := strings.TrimSpace(nextStep)
	next = strings.TrimPrefix(next, "下一步：")
	next = strings.TrimPrefix(next, "下一步:")
	next = strings.TrimSpace(next)
	if next != "" {
		return summarizeText(next, 180)
	}
	lower := strings.ToLower(strings.TrimSpace(problemSummary))
	switch {
	case strings.Contains(lower, "timeout"), strings.Contains(lower, "network"), strings.Contains(lower, "name or service"), strings.Contains(lower, "pypi"):
		return "先确认网络和镜像源可达，再重试 runtime.exec。"
	case strings.Contains(lower, "permission"), strings.Contains(lower, "denied"), strings.Contains(lower, "权限"):
		return "先修复目录或权限问题，再重试 runtime.exec。"
	default:
		return "先按失败摘要处理环境/依赖问题，再重试 runtime.exec。"
	}
}

func humanizeRuntimeReleaseStatusNote(note string) string {
	lines := splitNonEmptyLines(note)
	if len(lines) == 0 {
		return strings.TrimSpace(note)
	}
	head := strings.TrimSpace(lines[0])
	execSource := ""
	historyLines := make([]string, 0, len(lines)-1)
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "执行来源：") {
			execSource = line
			continue
		}
		historyLines = append(historyLines, line)
	}
	if execSource == "" {
		execSource = extractActionPlanInlineExecutionSource(note)
	}
	renderQueryCard := func(lines []string, summary string, evidence []string, nextStep string) string {
		lines = append([]string{}, lines...)
		lines = appendRuntimeSummaryLine(lines, summary)
		if execSource != "" {
			lines = append(lines, execSource)
		}
		lines = appendRuntimeEvidenceSection(lines, evidence)
		next := nextStepLine(nextStep)
		if next == "" {
			return strings.TrimSpace(strings.Join(lines, "\n"))
		}
		lines = append(lines, next)
		return strings.TrimSpace(strings.Join(lines, "\n"))
	}
	service := extractActionPlanTokenValue(head, "service=")
	if service == "" {
		service = "该服务"
	}
	if strings.Contains(head, "current=none") {
		return renderQueryCard([]string{
			fmt.Sprintf("查询结果：%s 当前还没有发布版本记录（current=none）。", service),
			"结论：暂无发布版本。",
			"完成状态：未发布。",
			"说明：尚未检测到可用的 current 发布元数据，通常表示还未完成首次发布。",
		}, fmt.Sprintf("service=%s current=none", service), []string{
			"service=" + service,
			"current=none",
		}, nextStepReleaseInitPublish)
	}
	if strings.Contains(head, "history=empty") {
		return renderQueryCard([]string{
			fmt.Sprintf("查询结果：%s 暂无发布历史记录（history=empty）。", service),
			"结论：暂无发布历史。",
			"完成状态：历史为空。",
			"说明：当前没有可追溯的发布历史，建议先执行一次发布并生成基线记录。",
		}, fmt.Sprintf("service=%s history=empty", service), []string{
			"service=" + service,
			"history=empty",
		}, nextStepReleaseInitPublish)
	}
	if strings.Contains(head, "current version=") {
		version := fallbackValue(extractActionPlanTokenValue(head, "version="), "-")
		state := fallbackValue(extractActionPlanTokenValue(head, "status="), "-")
		finishedAt := fallbackValue(extractActionPlanTokenValue(head, "finished_at="), "-")
		releaseID := fallbackValue(extractActionPlanTokenValue(head, "release_id="), "-")
		artifactSHA := fallbackValue(extractActionPlanTokenValue(head, "artifact_sha256="), "-")
		return renderQueryCard([]string{
			fmt.Sprintf("查询结果：%s 当前版本是 %s（状态：%s，时间：%s）。", service, version, state, finishedAt),
			fmt.Sprintf("结论：当前版本为 %s。", version),
			fmt.Sprintf("完成状态：%s。", mapRuntimeReleaseCurrentStatusLabel(state)),
		}, fmt.Sprintf("service=%s version=%s status=%s release_id=%s finished_at=%s", service, version, state, releaseID, finishedAt), []string{
			"service=" + service,
			"version=" + version,
			"status=" + state,
			"release_id=" + releaseID,
			"finished_at=" + finishedAt,
			"artifact_sha256=" + artifactSHA,
		}, nextStepReleaseDecideByCurrent)
	}
	if strings.Contains(head, "history_count=") {
		count := fallbackValue(extractActionPlanTokenValue(head, "history_count="), "0")
		entries := make([]string, 0, len(historyLines))
		versions := make([]string, 0, len(historyLines))
		for _, line := range historyLines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			version := fallbackValue(extractActionPlanTokenValue(line, "version="), "-")
			state := fallbackValue(extractActionPlanTokenValue(line, "status="), "-")
			finishedAt := fallbackValue(extractActionPlanTokenValue(line, "finished_at="), "-")
			entries = append(entries, fmt.Sprintf("%s（%s，%s）", version, state, finishedAt))
			versions = append(versions, version)
		}
		if len(entries) == 0 {
			return renderQueryCard([]string{
				fmt.Sprintf("查询结果：%s 最近 %s 次发布记录已返回。", service, count),
				fmt.Sprintf("结论：已返回 %s 条发布记录。", count),
				"完成状态：历史已返回。",
			}, fmt.Sprintf("service=%s history_count=%s", service, count), []string{
				"service=" + service,
				"history_count=" + count,
			}, nextStepReleaseInspectHistory)
		}
		return renderQueryCard([]string{
			fmt.Sprintf("查询结果：%s 最近 %s 次发布记录：%s。", service, count, strings.Join(entries, "；")),
			fmt.Sprintf("结论：已返回 %s 条发布记录。", count),
			"完成状态：历史已返回。",
		}, fmt.Sprintf("service=%s history_count=%s history_versions=%s", service, count, strings.Join(versions, "|")), []string{
			"service=" + service,
			"history_count=" + count,
			"history_versions=" + strings.Join(versions, "|"),
		}, nextStepReleaseInspectHistory)
	}
	return renderQueryCard([]string{
		"查询结果：" + strings.TrimSpace(note),
		"结论：发布状态已返回。",
		"完成状态：已查询。",
	}, fmt.Sprintf("service=%s status=query_returned", service), []string{
		"service=" + service,
		"raw_head=" + summarizeText(head, 140),
	}, nextStepReleaseInspectHistory)
}

func mapRuntimeReleaseCurrentStatusLabel(state string) string {
	switch strings.TrimSpace(strings.ToLower(state)) {
	case "succeeded", "success":
		return "已发布"
	case "rolled_back", "rollback":
		return "已回滚"
	case "failed":
		return "发布失败"
	case "running", "pending":
		return "发布中"
	default:
		if strings.TrimSpace(state) == "" || strings.TrimSpace(state) == "-" {
			return "已记录"
		}
		return strings.TrimSpace(state)
	}
}

func humanizeRuntimeReleaseMutationNote(note string) string {
	service := fallbackValue(extractActionPlanTokenValueExact(note, "service"), "-")
	version := fallbackValue(extractActionPlanTokenValueExact(note, "version"), "-")
	releaseID := fallbackValue(extractActionPlanTokenValueExact(note, "release_id"), "-")
	script := fallbackValue(extractActionPlanTokenValueExact(note, "script"), "-")
	health := strings.TrimSpace(extractActionPlanTokenValueExact(note, "health"))
	output := strings.TrimSpace(extractActionPlanTokenValueExact(note, "output"))
	metadataWarning := strings.TrimSpace(extractActionPlanTokenValueExact(note, "metadata_warning"))
	execSource := extractActionPlanInlineExecutionSource(note)

	lines := []string{
		fmt.Sprintf("runtime.release 完成：已完成发布（service=%s version=%s release_id=%s）。", service, version, releaseID),
		"结论：发布动作执行成功，可进入运行观测阶段。",
		"完成状态：已发布。",
		"执行脚本：" + script + "。",
	}
	lines = appendRuntimeSummaryLine(lines, fmt.Sprintf("发布已完成（service=%s version=%s release_id=%s）。", service, version, releaseID))
	if output != "" {
		lines = append(lines, "附加输出："+summarizeText(output, 160))
	}
	if health != "" {
		lines = append(lines, "健康检查："+health+"。")
	}
	if metadataWarning != "" {
		lines = append(lines, "元数据告警："+summarizeText(metadataWarning, 160))
	}
	if execSource != "" {
		lines = append(lines, execSource)
	}
	evidenceItems := []string{
		"release_id=" + releaseID,
		"script=" + script,
		"version=" + version,
	}
	if health != "" {
		evidenceItems = append(evidenceItems, "health="+health)
	}
	if output != "" {
		evidenceItems = append(evidenceItems, "output="+summarizeText(output, 100))
	}
	if metadataWarning != "" {
		evidenceItems = append(evidenceItems, "metadata_warning="+summarizeText(metadataWarning, 100))
	}
	lines = appendRuntimeEvidenceSection(lines, evidenceItems)
	lines = append(lines, nextStepLine(nextStepReleaseObserveMetrics))
	return strings.Join(lines, "\n")
}

func humanizeRuntimeServiceMutationNote(note string) string {
	service := fallbackValue(extractActionPlanTokenValueExact(note, "service"), "-")
	operation := fallbackValue(extractActionPlanTokenValueExact(note, "operation"), "-")
	health := strings.TrimSpace(extractActionPlanTokenValueExact(note, "health"))
	output := strings.TrimSpace(extractActionPlanTokenValueExact(note, "output"))
	execSource := extractActionPlanInlineExecutionSource(note)

	lines := []string{
		fmt.Sprintf("runtime.service 完成：已执行服务操作（service=%s operation=%s）。", service, operation),
		"结论：" + buildRuntimeServiceMutationConclusion(operation),
		"完成状态：" + mapRuntimeServiceMutationStatus(operation) + "。",
		"动作说明：" + mapServiceOperationLabel(operation) + "。",
	}
	lines = appendRuntimeSummaryLine(lines, fmt.Sprintf("服务操作已完成（service=%s operation=%s）。", service, operation))
	if output != "" {
		lines = append(lines, "附加输出："+summarizeText(output, 160))
	}
	if health != "" {
		lines = append(lines, "健康检查："+health+"。")
	}
	if execSource != "" {
		lines = append(lines, execSource)
	}
	evidenceItems := []string{
		"service=" + service,
		"operation=" + operation,
	}
	if health != "" {
		evidenceItems = append(evidenceItems, "health="+health)
	}
	if output != "" {
		evidenceItems = append(evidenceItems, "output="+summarizeText(output, 100))
	}
	lines = appendRuntimeEvidenceSection(lines, evidenceItems)
	if strings.EqualFold(operation, "status") {
		lines = append(lines, nextStepLine(nextStepServiceDecideByStatus))
	} else {
		lines = append(lines, nextStepLine(nextStepServiceRecheckStatus))
	}
	return strings.Join(lines, "\n")
}

func humanizeRuntimeServiceStatusNote(note string) string {
	service := fallbackValue(extractActionPlanTokenValueExact(note, "service"), "-")
	enabled := fallbackValue(extractActionPlanTokenValueExact(note, "enabled"), "-")
	active := fallbackValue(extractActionPlanTokenValueExact(note, "active"), "-")
	execSource := extractActionPlanInlineExecutionSource(note)

	lines := []string{
		fmt.Sprintf("查询结果：runtime.service 状态：service=%s。", service),
		fmt.Sprintf("结论：%s", buildRuntimeServiceStatusConclusion(active, enabled)),
		fmt.Sprintf("完成状态：%s。", mapRuntimeServiceStatusLabel(active, enabled)),
		fmt.Sprintf("服务状态：active=%s enabled=%s。", active, enabled),
	}
	lines = appendRuntimeSummaryLine(lines, fmt.Sprintf("service=%s active=%s enabled=%s。", service, active, enabled))
	if execSource != "" {
		lines = append(lines, execSource)
	}
	lines = appendRuntimeEvidenceSection(lines, []string{
		fmt.Sprintf("service=%s", service),
		fmt.Sprintf("active=%s", active),
		fmt.Sprintf("enabled=%s", enabled),
	})
	if strings.EqualFold(active, "active") {
		lines = append(lines, nextStepLine(nextStepServiceOnlineProceed))
	} else {
		lines = append(lines, nextStepLine(nextStepServiceRecover))
	}
	return strings.Join(lines, "\n")
}

func humanizeRuntimeSupervisorMutationNote(note string) string {
	service := fallbackValue(extractActionPlanTokenValueExact(note, "service"), "-")
	operation := fallbackValue(extractActionPlanTokenValueExact(note, "operation"), "-")
	unit := strings.TrimSpace(extractActionPlanTokenValueExact(note, "unit"))
	unitStatus := strings.TrimSpace(extractActionPlanTokenValueExact(note, "unit_status"))
	output := strings.TrimSpace(extractActionPlanTokenValueExact(note, "output"))
	health := strings.TrimSpace(extractActionPlanTokenValueExact(note, "health"))
	execSource := extractActionPlanInlineExecutionSource(note)

	lines := []string{
		fmt.Sprintf("runtime.supervisor 完成：已执行 supervisor 操作（service=%s operation=%s）。", service, operation),
		"结论：" + buildRuntimeSupervisorMutationConclusion(operation),
		"完成状态：" + mapRuntimeSupervisorMutationStatus(operation) + "。",
		"动作说明：" + mapSupervisorOperationLabel(operation) + "。",
	}
	lines = appendRuntimeSummaryLine(lines, fmt.Sprintf("supervisor 操作已完成（service=%s operation=%s）。", service, operation))
	if unit != "" {
		lines = append(lines, "Unit 文件："+unit+"。")
	}
	if unitStatus != "" {
		lines = append(lines, "Unit 状态："+unitStatus+"。")
	}
	if output != "" {
		lines = append(lines, "附加输出："+summarizeText(output, 160))
	}
	if health != "" {
		lines = append(lines, "健康检查："+health+"。")
	}
	if execSource != "" {
		lines = append(lines, execSource)
	}
	evidenceItems := []string{
		"service=" + service,
		"operation=" + operation,
	}
	if unit != "" {
		evidenceItems = append(evidenceItems, "unit="+unit)
	}
	if unitStatus != "" {
		evidenceItems = append(evidenceItems, "unit_status="+unitStatus)
	}
	if output != "" {
		evidenceItems = append(evidenceItems, "output="+summarizeText(output, 100))
	}
	if health != "" {
		evidenceItems = append(evidenceItems, "health="+health)
	}
	lines = appendRuntimeEvidenceSection(lines, evidenceItems)
	if strings.EqualFold(operation, "status") {
		lines = append(lines, nextStepLine(nextStepSupervisorDecideByStatus))
	} else {
		lines = append(lines, nextStepLine(nextStepSupervisorRecheckStatus))
	}
	return strings.Join(lines, "\n")
}

func humanizeRuntimeSupervisorStatusNote(note string) string {
	service := fallbackValue(extractActionPlanTokenValueExact(note, "service"), "-")
	enabled := fallbackValue(extractActionPlanTokenValueExact(note, "enabled"), "-")
	active := fallbackValue(extractActionPlanTokenValueExact(note, "active"), "-")
	execSource := extractActionPlanInlineExecutionSource(note)

	lines := []string{
		fmt.Sprintf("查询结果：runtime.supervisor 状态：service=%s。", service),
		fmt.Sprintf("结论：%s", buildRuntimeSupervisorStatusConclusion(active, enabled)),
		fmt.Sprintf("完成状态：%s。", mapRuntimeSupervisorStatusLabel(active, enabled)),
		fmt.Sprintf("Supervisor 状态：active=%s enabled=%s。", active, enabled),
	}
	lines = appendRuntimeSummaryLine(lines, fmt.Sprintf("service=%s active=%s enabled=%s。", service, active, enabled))
	if execSource != "" {
		lines = append(lines, execSource)
	}
	lines = appendRuntimeEvidenceSection(lines, []string{
		fmt.Sprintf("service=%s", service),
		fmt.Sprintf("active=%s", active),
		fmt.Sprintf("enabled=%s", enabled),
	})
	if strings.EqualFold(active, "active") {
		lines = append(lines, nextStepLine(nextStepSupervisorOnlineProceed))
	} else {
		lines = append(lines, nextStepLine(nextStepSupervisorRecover))
	}
	return strings.Join(lines, "\n")
}

func buildRuntimeServiceStatusConclusion(active string, enabled string) string {
	active = strings.TrimSpace(strings.ToLower(active))
	enabled = strings.TrimSpace(strings.ToLower(enabled))
	if active == "active" && enabled == "enabled" {
		return "服务在线且开机自启已启用，可继续推进任务。"
	}
	if active == "active" {
		return "服务当前在线，但启用状态未确认，建议补充检查。"
	}
	return "服务当前不在线，建议执行 restart 或 ensure_running 恢复。"
}

func mapRuntimeServiceStatusLabel(active string, enabled string) string {
	active = strings.TrimSpace(strings.ToLower(active))
	enabled = strings.TrimSpace(strings.ToLower(enabled))
	if active == "active" && enabled == "enabled" {
		return "在线"
	}
	if active == "active" {
		return "在线（启用待确认）"
	}
	return "待恢复"
}

func buildRuntimeSupervisorStatusConclusion(active string, enabled string) string {
	active = strings.TrimSpace(strings.ToLower(active))
	enabled = strings.TrimSpace(strings.ToLower(enabled))
	if active == "active" && enabled == "enabled" {
		return "supervisor 在线且已启用，可继续推进任务。"
	}
	if active == "active" {
		return "supervisor 当前在线，但启用状态未确认，建议补充检查。"
	}
	return "supervisor 当前异常或未启用，建议先 ensure 或重启服务。"
}

func mapRuntimeSupervisorStatusLabel(active string, enabled string) string {
	active = strings.TrimSpace(strings.ToLower(active))
	enabled = strings.TrimSpace(strings.ToLower(enabled))
	if active == "active" && enabled == "enabled" {
		return "在线"
	}
	if active == "active" {
		return "在线（启用待确认）"
	}
	return "待恢复"
}

func mapRuntimeTaskControlStatusLabel(workerOnline bool, taskStatus string, alertLevel string) string {
	taskStatus = strings.TrimSpace(strings.ToLower(taskStatus))
	alertLevel = strings.TrimSpace(strings.ToLower(alertLevel))
	if !workerOnline {
		if alertLevel == "critical" {
			return "待恢复（高风险）"
		}
		return "待恢复"
	}
	if taskStatus == "blocked" || taskStatus == "failed" {
		return "在线（任务阻塞）"
	}
	return "在线"
}

func mapRuntimeTaskControlMutationStatus(operation string) string {
	switch strings.TrimSpace(strings.ToLower(operation)) {
	case "start":
		return "已启动"
	case "restart":
		return "已重启"
	case "ensure_running":
		return "已确保运行"
	case "ensure_running_skip":
		return "已跳过重启"
	case "stop":
		return "已停止"
	case "status":
		return "已查询"
	default:
		return "已执行"
	}
}

func buildRuntimeServiceMutationConclusion(operation string) string {
	switch strings.TrimSpace(strings.ToLower(operation)) {
	case "status":
		return "服务状态查询已完成，可按结果决定是否继续操作。"
	case "start", "restart", "ensure_running", "ensure_running_skip":
		return "服务拉起操作已执行，建议复核在线与健康状态。"
	case "stop", "disable":
		return "停止类操作已执行，若需恢复可再执行 start/restart。"
	default:
		return "服务操作已执行，建议复核运行状态。"
	}
}

func mapRuntimeServiceMutationStatus(operation string) string {
	switch strings.TrimSpace(strings.ToLower(operation)) {
	case "status":
		return "已查询"
	case "start":
		return "已启动"
	case "restart":
		return "已重启"
	case "ensure_running", "ensure_running_skip":
		return "已确保运行"
	case "stop", "disable":
		return "已停止"
	default:
		return "已执行"
	}
}

func buildRuntimeSupervisorMutationConclusion(operation string) string {
	switch strings.TrimSpace(strings.ToLower(operation)) {
	case "status":
		return "supervisor 状态查询已完成，可按结果决定 ensure/disable。"
	case "ensure":
		return "supervisor ensure 已执行，建议复核 unit 状态与健康检查。"
	case "disable":
		return "supervisor 停用已执行，若需恢复可再次 ensure。"
	default:
		return "supervisor 操作已执行，建议复核 unit 状态。"
	}
}

func mapRuntimeSupervisorMutationStatus(operation string) string {
	switch strings.TrimSpace(strings.ToLower(operation)) {
	case "status":
		return "已查询"
	case "ensure":
		return "已确保运行"
	case "disable":
		return "已停用"
	default:
		return "已执行"
	}
}

func mapConfigExecStatusLabel(text string) string {
	text = strings.TrimSpace(text)
	switch {
	case strings.Contains(text, "配置已应用"):
		return "已应用"
	case strings.Contains(text, "当前没有待确认"), strings.Contains(text, "已取消待确认计划"):
		return "无待应用"
	case strings.Contains(text, "已生成配置计划"), strings.Contains(text, "已更新待确认计划"), strings.Contains(text, "待确认计划"):
		return "待应用"
	default:
		return "已执行"
	}
}

func looksLikeConfigExecResponseNote(note string) bool {
	note = strings.TrimSpace(note)
	if note == "" {
		return false
	}
	return strings.HasPrefix(note, "已生成配置计划:") ||
		strings.HasPrefix(note, "已更新待确认计划") ||
		strings.HasPrefix(note, "待确认计划:") ||
		strings.HasPrefix(note, "当前没有待确认的配置计划") ||
		strings.HasPrefix(note, "已取消待确认计划") ||
		strings.HasPrefix(note, "配置已应用") ||
		strings.HasPrefix(note, "我理解你可能想改配置") ||
		containsConfigSlashCommandMention(note)
}

func containsConfigSlashCommandMention(note string) bool {
	text := strings.TrimSpace(strings.ToLower(note))
	if text == "" {
		return false
	}
	const token = "/config"
	for start := 0; start < len(text); {
		idx := strings.Index(text[start:], token)
		if idx < 0 {
			return false
		}
		idx += start
		end := idx + len(token)
		if hasConfigCommandBoundaryBefore(text, idx) && isConfigSlashCommandAt(text, end) {
			return true
		}
		start = idx + len(token)
	}
	return false
}

func hasConfigCommandBoundaryBefore(text string, idx int) bool {
	if idx == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(text[:idx])
	if unicode.IsSpace(r) {
		return true
	}
	return strings.ContainsRune("`\"'([{<（【「", r)
}

func isConfigSlashCommandAt(text string, idx int) bool {
	if idx >= len(text) {
		return true
	}
	r, size := utf8.DecodeRuneInString(text[idx:])
	if isConfigCommandTerminator(r) {
		return true
	}
	if !unicode.IsSpace(r) {
		return false
	}
	cursor := idx + size
	for cursor < len(text) {
		nextRune, nextSize := utf8.DecodeRuneInString(text[cursor:])
		if !unicode.IsSpace(nextRune) {
			break
		}
		cursor += nextSize
	}
	if cursor >= len(text) {
		return true
	}
	tokenStart := cursor
	for cursor < len(text) {
		nextRune, nextSize := utf8.DecodeRuneInString(text[cursor:])
		if unicode.IsSpace(nextRune) || isConfigCommandTerminator(nextRune) {
			break
		}
		cursor += nextSize
	}
	action := strings.TrimSpace(text[tokenStart:cursor])
	if action == "" {
		return true
	}
	switch action {
	case "help", "show", "plan", "apply", "cancel":
		return true
	default:
		return false
	}
}

func isConfigCommandTerminator(r rune) bool {
	return strings.ContainsRune("`\"')]}>，。,:：;；!?？！）】」", r)
}

func humanizeConfigExecNote(note string) string {
	text := strings.TrimSpace(note)
	execSource := extractActionPlanInlineExecutionSource(text)
	baseText := strings.TrimSpace(strings.ReplaceAll(text, execSource, ""))
	baseText = strings.TrimSpace(strings.Trim(baseText, "；;,\n"))
	if baseText == "" {
		baseText = text
	}
	lines := []string{
		"config.exec 完成：已执行配置指令。",
		"结论：" + buildConfigExecConclusion(baseText),
		"完成状态：" + mapConfigExecStatusLabel(baseText) + "。",
	}
	lines = appendRuntimeSummaryLine(lines, summarizeText(baseText, 220))
	if execSource != "" {
		lines = append(lines, execSource)
	}
	lines = appendRuntimeEvidenceSection(lines, extractConfigExecEvidence(baseText))
	switch {
	case strings.Contains(baseText, "已生成配置计划"), strings.Contains(baseText, "待确认计划"), strings.Contains(baseText, "已更新待确认计划"):
		lines = append(lines, nextStepLine(nextStepConfigApply))
	case strings.Contains(baseText, "当前没有待确认的配置计划"):
		lines = append(lines, nextStepLine(nextStepConfigPlanFirst))
	case strings.Contains(baseText, "配置已应用"):
		lines = append(lines, nextStepLine(nextStepConfigShow))
	default:
		lines = append(lines, nextStepLine(nextStepConfigApplyPersist))
	}
	return strings.Join(lines, "\n")
}

func buildConfigExecConclusion(text string) string {
	text = strings.TrimSpace(text)
	switch {
	case strings.Contains(text, "配置已应用"):
		return "配置已应用并写入当前环境。"
	case strings.Contains(text, "当前没有待确认"), strings.Contains(text, "当前没有待确认的配置计划"):
		return "当前没有待应用配置计划，需先生成计划。"
	case strings.Contains(text, "已取消待确认计划"):
		return "待确认配置计划已取消。"
	case strings.Contains(text, "已生成配置计划"), strings.Contains(text, "已更新待确认计划"), strings.Contains(text, "待确认计划"):
		return "配置计划已生成，等待确认后应用。"
	default:
		return "配置指令已执行。"
	}
}

func extractConfigExecEvidence(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	lower := strings.ToLower(text)
	evidence := make([]string, 0, 8)
	if strings.Contains(lower, "/config plan") {
		evidence = append(evidence, "命令提示：/config plan")
	}
	if strings.Contains(lower, "/config show") {
		evidence = append(evidence, "命令提示：/config show")
	}
	if strings.Contains(lower, "/config apply") {
		evidence = append(evidence, "命令提示：/config apply")
	}
	if strings.Contains(lower, "/config cancel") {
		evidence = append(evidence, "命令提示：/config cancel")
	}
	if strings.Contains(text, "待确认计划") {
		evidence = append(evidence, "状态：存在待确认配置计划")
	}
	if strings.Contains(text, "配置已应用") {
		evidence = append(evidence, "状态：配置已落盘应用")
	}
	if len(evidence) == 0 {
		evidence = append(evidence, summarizeText(text, 140))
	}
	return evidence
}

func humanizeRuntimeTaskDelegatesQueryNote(note string) string {
	lines := splitNonEmptyLines(note)
	if len(lines) == 0 {
		return note
	}
	head := lines[0]
	agent := fallbackValue(extractActionPlanTokenValueExact(head, "agent"), "main")
	total := parseActionPlanTokenInt(head, "total")
	queued := parseActionPlanTokenInt(head, "queued")
	running := parseActionPlanTokenInt(head, "running")
	succeeded := parseActionPlanTokenInt(head, "succeeded")
	failed := parseActionPlanTokenInt(head, "failed")
	canceled := parseActionPlanTokenInt(head, "canceled")
	parentTaskID := strings.TrimSpace(extractActionPlanTokenValueExact(head, "parent_task_id"))

	card := []string{"查询结果：runtime.task.delegates"}
	card = append(card, "结论："+buildRuntimeTaskDelegatesConclusion(total, queued, running, succeeded, failed, canceled))
	card = append(card, "完成状态："+mapRuntimeTaskDelegatesStatusLabel(total, queued, running, succeeded, failed, canceled)+"。")
	summary := fmt.Sprintf(
		"子任务进展：agent=%s total=%d（queued=%d running=%d succeeded=%d failed=%d canceled=%d）",
		agent,
		total,
		queued,
		running,
		succeeded,
		failed,
		canceled,
	)
	if parentTaskID != "" {
		summary += " parent_task_id=" + parentTaskID
	}
	card = appendRuntimeSummaryLine(card, summary+"。")
	evidenceItems := []string{
		fmt.Sprintf("关键计数：total=%d queued=%d running=%d succeeded=%d failed=%d canceled=%d。", total, queued, running, succeeded, failed, canceled),
	}

	taskLines := make([]string, 0, 8)
	failedTop := ""
	remainingHint := ""
	nextStep := ""
	execSource := ""
	for _, line := range lines[1:] {
		switch {
		case strings.HasPrefix(line, "#"):
			taskLines = append(taskLines, line)
		case strings.HasPrefix(line, "失败原因Top："):
			failedTop = line
		case strings.HasPrefix(line, "..."):
			remainingHint = line
		case strings.HasPrefix(line, "下一步："):
			nextStep = line
		case strings.HasPrefix(line, "执行来源："):
			execSource = strings.TrimSpace(line)
		}
	}
	if execSource == "" {
		execSource = extractActionPlanInlineExecutionSource(note)
	}
	if execSource != "" {
		card = append(card, execSource)
	}
	if failedTop != "" {
		evidenceItems = append(evidenceItems, failedTop)
	}
	if len(taskLines) > 0 {
		selected := make([]string, 0, len(taskLines))
		for _, line := range taskLines {
			if strings.EqualFold(strings.TrimSpace(extractActionPlanTokenValueExact(line, "status")), "failed") {
				selected = append(selected, line)
			}
		}
		for _, line := range taskLines {
			if containsString(selected, line) {
				continue
			}
			selected = append(selected, line)
		}
		limit := 3
		if len(selected) < limit {
			limit = len(selected)
		}
		for i := 0; i < limit; i++ {
			evidenceItems = append(evidenceItems, "子任务样本："+selected[i])
		}
	}
	card = appendRuntimeEvidenceSection(card, evidenceItems)
	if remainingHint != "" {
		card = append(card, remainingHint)
	}
	if nextStep != "" {
		card = append(card, nextStepLine(nextStep))
	} else {
		card = append(card, nextStepLine(nextStepDelegatesPrioritizeFailed))
	}
	return strings.Join(card, "\n")
}

func buildRuntimeTaskDelegatesConclusion(total, queued, running, succeeded, failed, canceled int) string {
	switch {
	case total <= 0:
		return "当前没有可跟进的子任务。"
	case failed > 0:
		return fmt.Sprintf("存在 %d 个失败子任务，建议优先处理失败项。", failed)
	case running > 0 || queued > 0:
		return fmt.Sprintf("仍有 %d 个任务在运行、%d 个任务排队，建议等待执行完成后再汇总。", running, queued)
	case canceled >= total:
		return "子任务已全部取消，需确认是否重新委派。"
	case succeeded+canceled >= total:
		return "子任务已处理完成，可汇总结果回复用户。"
	default:
		return "已返回子任务状态，请按计数继续推进。"
	}
}

func mapRuntimeTaskDelegatesStatusLabel(total, queued, running, succeeded, failed, canceled int) string {
	switch {
	case total <= 0:
		return "无任务"
	case failed > 0:
		return "阻塞"
	case running > 0 || queued > 0:
		return "进行中"
	case canceled >= total:
		return "已取消"
	case succeeded+canceled >= total:
		return "已完成"
	default:
		return "已查询"
	}
}

func humanizeRuntimeTaskStatusQueryNote(note string) string {
	agent, status := extractRuntimeTaskStatusHead(note)
	if agent == "" {
		agent = "main"
	}
	if status == "" {
		status = "unknown"
	}

	taskID := strings.TrimSpace(extractActionPlanTokenValueExact(note, "task_id"))
	goal := extractActionPlanSegmentValue(note, "目标：")
	remaining := extractActionPlanSegmentValue(note, "剩余步骤：")
	nextAction := extractActionPlanSegmentValue(note, "下一步：")
	updated := extractActionPlanSegmentValue(note, "最近更新：")
	lastResult := extractActionPlanSegmentValue(note, "最近结果：")
	execStats := extractActionPlanSegmentValue(note, "执行统计：")
	execSource := extractActionPlanInlineExecutionSource(note)

	if strings.TrimSpace(nextAction) == "" {
		nextAction = defaultTaskStatusGuidance(status)
	}

	lines := []string{
		fmt.Sprintf("查询结果：%s 当前任务状态为 %s。", agent, status),
		fmt.Sprintf("结论：任务当前处于 %s 状态，建议按下一步动作继续推进。", status),
		fmt.Sprintf("完成状态：%s。", mapRuntimeTaskStatusLabel(status)),
	}
	lines = appendRuntimeSummaryLine(lines, fmt.Sprintf("agent=%s status=%s。", agent, status))
	if taskID != "" {
		lines = append(lines, "任务标识：task_id="+taskID+"。")
	}
	if goal != "" {
		lines = append(lines, "目标："+summarizeText(goal, 140))
	}
	if remaining != "" {
		lines = append(lines, "剩余步骤："+summarizeText(remaining, 200))
	}
	if updated != "" {
		lines = append(lines, "最近更新："+updated)
	}
	if execStats != "" {
		lines = append(lines, "执行统计："+execStats)
	}
	if lastResult != "" {
		lines = append(lines, "最近结果："+summarizeText(lastResult, 180))
	}
	if execSource != "" {
		lines = append(lines, execSource)
	}
	lines = appendRuntimeEvidenceSection(lines, []string{
		"状态：status=" + fallbackValue(status, "-") + "。",
		"任务标识：task_id=" + fallbackValue(taskID, "-") + "。",
		"目标：" + fallbackValue(summarizeText(goal, 120), "-"),
		"执行统计：" + fallbackValue(execStats, "-"),
		"最近更新：" + fallbackValue(updated, "-"),
		"最近结果：" + fallbackValue(summarizeText(lastResult, 120), "-"),
	})
	if nextAction != "" {
		lines = append(lines, nextStepLine(summarizeText(nextAction, 160)))
	}
	return strings.Join(lines, "\n")
}

func extractRuntimeTaskStatusHead(note string) (string, string) {
	pattern := regexp.MustCompile(`^查询结果：(.+?)\s+当前任务状态为\s+([a-zA-Z0-9_-]+)`)
	match := pattern.FindStringSubmatch(strings.TrimSpace(note))
	if len(match) < 3 {
		return "", ""
	}
	return strings.TrimSpace(match[1]), strings.TrimSpace(strings.ToLower(match[2]))
}

func defaultTaskStatusGuidance(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "running":
		return "继续等待执行并在关键节点回报进展"
	case "pending":
		return "等待任务开始执行，必要时检查 worker 是否在线"
	case "blocked", "failed":
		return "优先处理阻塞原因后重试"
	case "completed":
		return "可汇总结果并回复用户"
	default:
		return ""
	}
}

func mapRuntimeTaskStatusLabel(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "running", "partial":
		return "进行中"
	case "pending", "queued":
		return "待开始"
	case "blocked", "failed":
		return "阻塞"
	case "completed", "succeeded", "done":
		return "已完成"
	case "canceled", "cancelled":
		return "已取消"
	default:
		status = strings.TrimSpace(status)
		if status == "" {
			return "未知"
		}
		return status
	}
}

func humanizeRuntimeTaskDelegateMutationNote(note string) string {
	agent := fallbackValue(extractActionPlanTokenValueExact(note, "agent"), "main")
	taskID := fallbackValue(extractActionPlanTokenValueExact(note, "child_task_id"), "-")
	status := fallbackValue(extractActionPlanTokenValueExact(note, "status"), "-")
	parentTaskID := strings.TrimSpace(extractActionPlanTokenValueExact(note, "parent_task_id"))
	dispatched := strings.TrimSpace(extractActionPlanTokenValueExact(note, "dispatched"))
	execSource := extractActionPlanInlineExecutionSource(note)

	lines := []string{
		fmt.Sprintf("runtime.task.delegate 完成：已创建子任务 %s（agent=%s，状态=%s）。", taskID, agent, status),
		"结论：" + buildRuntimeTaskDelegateMutationConclusion(status),
		"完成状态：" + mapRuntimeTaskStatusLabel(status) + "。",
	}
	lines = appendRuntimeSummaryLine(lines, fmt.Sprintf("已完成子任务委派（child_task_id=%s agent=%s status=%s）。", taskID, agent, status))
	if parentTaskID != "" {
		lines = append(lines, "关联父任务：parent_task_id="+parentTaskID+"。")
	}
	if dispatched != "" && dispatched != "0" {
		lines = append(lines, "调度结果：dispatched="+dispatched+"。")
	}
	if execSource != "" {
		lines = append(lines, execSource)
	}
	lines = appendRuntimeEvidenceSection(lines, []string{
		"child_task_id=" + taskID,
		"parent_task_id=" + fallbackValue(parentTaskID, "-"),
		"dispatched=" + fallbackValue(dispatched, "-"),
	})
	lines = append(lines, nextStepLine(nextStepDelegateWait))
	return strings.Join(lines, "\n")
}

func buildRuntimeTaskDelegateMutationConclusion(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "queued", "pending":
		return "子任务已进入队列，等待执行。"
	case "running":
		return "子任务已开始执行，等待完成结果。"
	case "blocked", "failed":
		return "子任务执行失败或阻塞，需优先排障后再推进。"
	case "completed", "succeeded", "done":
		return "子任务已完成，可汇总结果。"
	case "canceled", "cancelled":
		return "子任务已取消，需确认是否重新委派。"
	default:
		return "子任务委派已记录。"
	}
}

func humanizeRuntimeTaskRetryMutationNote(note string) string {
	agent := fallbackValue(extractActionPlanTokenValueExact(note, "agent"), "main")
	taskID := fallbackValue(extractActionPlanTokenValueExact(note, "task_id"), "-")
	status := fallbackValue(extractActionPlanTokenValueExact(note, "status"), "-")
	retry := fallbackValue(extractActionPlanTokenValueExact(note, "retry"), "-")
	parentTaskID := strings.TrimSpace(extractActionPlanTokenValueExact(note, "parent_task_id"))
	execSource := extractActionPlanInlineExecutionSource(note)

	lines := []string{
		fmt.Sprintf("runtime.task.retry 完成：已重试子任务 %s（agent=%s，状态=%s，重试=%s）。", taskID, agent, status, retry),
		"结论：" + buildRuntimeTaskRetryMutationConclusion(status),
		"完成状态：" + mapRuntimeTaskStatusLabel(status) + "。",
		"任务标识：task_id=" + taskID + "。",
	}
	lines = appendRuntimeSummaryLine(lines, fmt.Sprintf("子任务已重试（task_id=%s retry=%s）。", taskID, retry))
	if parentTaskID != "" {
		lines = append(lines, "关联父任务：parent_task_id="+parentTaskID+"。")
	}
	if execSource != "" {
		lines = append(lines, execSource)
	}
	lines = appendRuntimeEvidenceSection(lines, []string{
		"task_id=" + taskID,
		"retry=" + retry,
		"parent_task_id=" + fallbackValue(parentTaskID, "-"),
	})
	lines = append(lines, nextStepLine(nextStepRetryWait))
	return strings.Join(lines, "\n")
}

func buildRuntimeTaskRetryMutationConclusion(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "queued", "pending", "running":
		return "子任务已提交重试，等待重试结果。"
	case "blocked", "failed":
		return "重试后仍处于失败/阻塞状态，需继续排障。"
	case "completed", "succeeded", "done":
		return "重试后任务已完成。"
	case "canceled", "cancelled":
		return "重试任务已取消，需确认后续策略。"
	default:
		return "子任务重试已触发。"
	}
}

func humanizeRuntimeTaskCancelMutationNote(note string) string {
	agent := fallbackValue(extractActionPlanTokenValueExact(note, "agent"), "main")
	taskID := fallbackValue(extractActionPlanTokenValueExact(note, "task_id"), "-")
	status := fallbackValue(extractActionPlanTokenValueExact(note, "status"), "-")
	previousStatus := fallbackValue(extractActionPlanTokenValueExact(note, "previous_status"), "-")
	parentTaskID := strings.TrimSpace(extractActionPlanTokenValueExact(note, "parent_task_id"))
	execSource := extractActionPlanInlineExecutionSource(note)

	lines := []string{
		fmt.Sprintf("runtime.task.cancel 完成：已取消子任务 %s（agent=%s，原状态=%s，当前=%s）。", taskID, agent, previousStatus, status),
		"结论：" + buildRuntimeTaskCancelMutationConclusion(previousStatus, status),
		"完成状态：" + mapRuntimeTaskStatusLabel(status) + "。",
		"任务标识：task_id=" + taskID + "。",
	}
	lines = appendRuntimeSummaryLine(lines, fmt.Sprintf("子任务已取消（task_id=%s previous_status=%s status=%s）。", taskID, previousStatus, status))
	if parentTaskID != "" {
		lines = append(lines, "关联父任务：parent_task_id="+parentTaskID+"。")
	}
	if execSource != "" {
		lines = append(lines, execSource)
	}
	lines = appendRuntimeEvidenceSection(lines, []string{
		"task_id=" + taskID,
		"previous_status=" + previousStatus,
		"status=" + status,
		"parent_task_id=" + fallbackValue(parentTaskID, "-"),
	})
	lines = append(lines, nextStepLine(nextStepCancelDecide))
	return strings.Join(lines, "\n")
}

func buildRuntimeTaskCancelMutationConclusion(previousStatus string, status string) string {
	status = strings.TrimSpace(strings.ToLower(status))
	previousStatus = strings.TrimSpace(strings.ToLower(previousStatus))
	switch status {
	case "canceled", "cancelled":
		return "子任务已取消，可决定是否重试或重新委派。"
	case "completed", "succeeded", "done":
		return "子任务已完成，无需继续取消动作。"
	case "failed", "blocked":
		return "子任务处于失败/阻塞状态，建议改用重试或排障处理。"
	case "running":
		if previousStatus == "running" {
			return "取消动作已执行，需确认任务是否已从运行态退出。"
		}
		return "子任务当前仍在运行，建议继续跟踪取消结果。"
	default:
		return "子任务取消动作已执行。"
	}
}

func humanizeRuntimeTaskControlMutationNote(note string) string {
	agent := fallbackValue(extractActionPlanTokenValueExact(note, "agent"), "main")
	serviceName := fallbackValue(extractActionPlanTokenValueExact(note, "service"), "-")
	operation := fallbackValue(extractActionPlanTokenValueExact(note, "operation"), "-")
	operationLabel := mapTaskControlOperationLabel(operation)
	execSource := extractActionPlanInlineExecutionSource(note)
	tail := ""
	if idx := strings.Index(note, "；"); idx >= 0 && idx+len("；") < len(note) {
		tail = strings.TrimSpace(note[idx+len("；"):])
	}
	tailOperation := strings.TrimSpace(strings.ToLower(extractActionPlanTokenValueExact(tail, "operation")))
	tailOutput := strings.TrimSpace(strings.ToLower(extractActionPlanTokenValueExact(tail, "output")))
	healthURL := sanitizeManagedHealthURL(extractActionPlanTokenValueExact(tail, "health"))
	healthState := strings.TrimSpace(strings.ToLower(extractActionPlanTokenValueExact(tail, "health_state")))
	healthCode := parseActionPlanTokenInt(tail, "health_code")
	ensureRunningSkipped := strings.EqualFold(operation, "ensure_running") &&
		(tailOperation == "ensure_running_skip" || tailOutput == "already_active")

	lines := make([]string, 0, 8)
	if ensureRunningSkipped {
		lines = append(lines, fmt.Sprintf("runtime.task.control 完成：检测到服务已在线且健康，本次已跳过重启（operation=%s，agent=%s，service=%s）。", operation, agent, serviceName))
		lines = append(lines, "结论：worker 已在线且健康，本轮无需重启。")
		lines = append(lines, "完成状态：已跳过重启。")
		lines = append(lines, "动作说明：服务已在线，复用现有进程并继续执行。")
		lines = appendRuntimeSummaryLine(lines, fmt.Sprintf("worker 已在线，已跳过重启（service=%s operation=%s output=already_active）。", serviceName, operation))
		if healthURL != "" || healthState != "" {
			healthLabel := "未知"
			switch healthState {
			case "up":
				healthLabel = "可用"
			case "down":
				healthLabel = "异常"
			}
			if healthURL != "" && healthCode > 0 {
				lines = append(lines, fmt.Sprintf("健康探针：%s（url=%s http=%d）。", healthLabel, healthURL, healthCode))
			} else if healthURL != "" {
				lines = append(lines, fmt.Sprintf("健康探针：%s（url=%s）。", healthLabel, healthURL))
			} else {
				lines = append(lines, fmt.Sprintf("健康探针：%s。", healthLabel))
			}
		}
	} else {
		lines = append(lines,
			fmt.Sprintf("runtime.task.control 完成：已执行 worker 控制（operation=%s，agent=%s，service=%s）。", operation, agent, serviceName),
			"结论："+buildRuntimeTaskControlMutationConclusion(operation),
			"完成状态："+mapRuntimeTaskControlMutationStatus(operation)+"。",
			"动作说明："+operationLabel+"。",
		)
		lines = appendRuntimeSummaryLine(lines, fmt.Sprintf("worker 控制已执行（service=%s operation=%s）。", serviceName, operation))
	}
	if execSource != "" {
		lines = append(lines, execSource)
	}
	evidenceItems := []string{
		"agent=" + fallbackValue(agent, "-"),
		"service=" + fallbackValue(serviceName, "-"),
		"operation=" + fallbackValue(operation, "-"),
	}
	if tailOperation != "" {
		evidenceItems = append(evidenceItems, "service_operation="+tailOperation)
	}
	if tailOutput != "" {
		evidenceItems = append(evidenceItems, "service_output="+tailOutput)
	}
	if healthURL != "" {
		evidenceItems = append(evidenceItems, "health="+healthURL)
	}
	if healthState != "" {
		evidenceItems = append(evidenceItems, "health_state="+healthState)
	}
	if healthCode > 0 {
		evidenceItems = append(evidenceItems, fmt.Sprintf("health_code=%d", healthCode))
	}
	if tail != "" {
		lines = append(lines, "附加结果："+summarizeText(tail, 180))
		evidenceItems = append(evidenceItems, "附加结果："+summarizeText(tail, 180))
		if ensureRunningSkipped {
			evidenceItems = append(evidenceItems,
				"service="+serviceName,
				"operation="+operation,
				"output=already_active",
			)
			if healthURL != "" {
				evidenceItems = append(evidenceItems, "health="+healthURL)
			}
			if healthState != "" {
				evidenceItems = append(evidenceItems, "health_state="+healthState)
			}
			if healthCode > 0 {
				evidenceItems = append(evidenceItems, fmt.Sprintf("health_code=%d", healthCode))
			}
		}
	}
	lines = appendRuntimeEvidenceSection(lines, evidenceItems)
	lines = append(lines, nextStepLine(nextStepTaskControlCheckStatus))
	return strings.Join(lines, "\n")
}

func buildRuntimeTaskControlMutationConclusion(operation string) string {
	switch strings.TrimSpace(strings.ToLower(operation)) {
	case "ensure_running":
		return "worker 保障动作已执行，建议复核任务与健康状态。"
	case "restart":
		return "worker 重启动作已执行，建议观察任务恢复情况。"
	case "start":
		return "worker 启动已执行，可继续检查任务推进状态。"
	case "stop":
		return "worker 停止动作已执行，若需恢复可执行 ensure_running/start。"
	case "status":
		return "worker 状态查询已完成。"
	default:
		return "worker 控制动作已执行。"
	}
}

func humanizeRuntimeTaskControlStatusNote(note string) string {
	agent := fallbackValue(extractActionPlanTokenValueExact(note, "agent"), "main")
	serviceName := fallbackValue(extractActionPlanTokenValueExact(note, "service"), "-")
	active := fallbackValue(extractActionPlanTokenValueExact(note, "active"), "-")
	enabled := fallbackValue(extractActionPlanTokenValueExact(note, "enabled"), "-")
	healthURL := sanitizeManagedHealthURL(extractActionPlanTokenValueExact(note, "health_url"))
	healthState := strings.TrimSpace(strings.ToLower(extractActionPlanTokenValueExact(note, "health_state")))
	healthCode := parseActionPlanTokenInt(note, "health_code")
	healthSource := strings.TrimSpace(strings.ToLower(extractActionPlanTokenValueExact(note, "health_source")))
	lastOperation := strings.TrimSpace(strings.ToLower(extractActionPlanTokenValueExact(note, "last_operation")))
	lastOperationAt := strings.TrimSpace(extractActionPlanTokenValueExact(note, "last_operation_at"))
	autoRecovery := strings.TrimSpace(strings.ToLower(extractActionPlanTokenValueExact(note, "auto_recovery")))
	autoRecoveryReason := strings.TrimSpace(strings.ToLower(extractActionPlanTokenValueExact(note, "auto_recovery_reason")))
	autoRecoveryFailures := parseActionPlanTokenInt(note, "auto_recovery_failures")
	autoRecoveryCooldownUntil := strings.TrimSpace(extractActionPlanTokenValueExact(note, "auto_recovery_cooldown_until"))
	trackedReleaseVersion := strings.TrimSpace(extractActionPlanTokenValueExact(note, "tracked_release_version"))
	currentReleaseVersion := strings.TrimSpace(extractActionPlanTokenValueExact(note, "current_release_version"))
	trackedReleaseID := strings.TrimSpace(extractActionPlanTokenValueExact(note, "tracked_release_id"))
	currentReleaseID := strings.TrimSpace(extractActionPlanTokenValueExact(note, "current_release_id"))
	if active == "-" || active == "" {
		if outputState := normalizeManagedServiceStateToken(extractActionPlanTokenValueExact(note, "output")); outputState != "" {
			active = outputState
		}
	}
	taskStatus := extractActionPlanTaskStatusFromControlNote(note)
	execSource := extractActionPlanInlineExecutionSource(note)
	alertLevel, alertExplain := mapTaskControlAutoRecoveryAlertLevelAndGuidance(autoRecovery)

	workerOnline := strings.EqualFold(active, "active")
	if healthState == "up" {
		workerOnline = true
	} else if healthState == "down" {
		workerOnline = false
	}
	if alertLevel == "critical" || alertLevel == "warning" {
		workerOnline = false
	}

	lines := []string{
		fmt.Sprintf("查询结果：runtime.task.control 状态：agent=%s service=%s。", agent, serviceName),
		fmt.Sprintf("结论：%s", buildTaskControlStatusConclusion(workerOnline, taskStatus, alertLevel)),
		fmt.Sprintf("完成状态：%s。", mapRuntimeTaskControlStatusLabel(workerOnline, taskStatus, alertLevel)),
		fmt.Sprintf("当前任务状态：%s；服务状态：active=%s enabled=%s。", fallbackValue(taskStatus, "-"), active, enabled),
		fmt.Sprintf("服务回执：已执行 runtime.service status（service=%s）。", serviceName),
	}
	if healthURL != "" || healthState != "" {
		healthLabel := "未知"
		switch healthState {
		case "up":
			healthLabel = "可用"
		case "down":
			healthLabel = "异常"
		}
		if healthURL != "" && healthCode > 0 {
			lines = append(lines, fmt.Sprintf("健康探针：%s（url=%s http=%d）。", healthLabel, healthURL, healthCode))
		} else if healthURL != "" {
			lines = append(lines, fmt.Sprintf("健康探针：%s（url=%s）。", healthLabel, healthURL))
		} else {
			lines = append(lines, fmt.Sprintf("健康探针：%s。", healthLabel))
		}
		if healthSource != "" {
			lines = append(lines, "健康来源："+healthSource+"。")
		}
	}
	if lastOperation != "" || lastOperationAt != "" {
		lastLine := "最近控制："
		if lastOperation != "" {
			lastLine += "operation=" + lastOperation
		}
		if lastOperationAt != "" {
			if lastOperation != "" {
				lastLine += "；"
			}
			lastLine += "updated_at=" + lastOperationAt
		}
		lines = append(lines, lastLine+"。")
		if lastOperation != "" {
			lines = append(lines, "最近控制说明："+mapTaskControlOperationLabel(lastOperation)+"。")
		}
	}
	if autoRecovery != "" {
		recoveryLine := "自动恢复：" + mapTaskControlAutoRecoveryStateLabel(autoRecovery)
		if reasonLabel := mapTaskControlAutoRecoveryReasonLabel(autoRecoveryReason); reasonLabel != "" {
			recoveryLine += "（原因：" + reasonLabel + "）"
		}
		lines = append(lines, recoveryLine+"。")
		if alertLevel != "" {
			lines = append(lines, fmt.Sprintf("自愈告警级别：%s（%s）。", alertLevel, alertExplain))
		}
		if autoRecoveryReason == "release_changed" {
			versionDiff := buildTaskControlReleaseDeltaSummary(trackedReleaseVersion, currentReleaseVersion, trackedReleaseID, currentReleaseID)
			if versionDiff != "" {
				lines = append(lines, "版本变更："+versionDiff+"。")
			}
		}
		if autoRecoveryFailures > 0 {
			lines = append(lines, fmt.Sprintf("自动恢复失败计数：%d。", autoRecoveryFailures))
		}
		if autoRecoveryCooldownUntil != "" {
			lines = append(lines, "自动恢复冷却至："+autoRecoveryCooldownUntil+"。")
		}
	}
	summary := fmt.Sprintf("agent=%s service=%s task_status=%s", agent, serviceName, fallbackValue(taskStatus, "-"))
	if healthState != "" {
		summary += " health_state=" + healthState
	}
	if lastOperation != "" {
		summary += " last_operation=" + lastOperation
	}
	if autoRecovery != "" {
		summary += " auto_recovery=" + autoRecovery
	}
	if alertLevel != "" {
		summary += " self_heal_alert=" + alertLevel
	}
	if autoRecoveryFailures > 0 {
		summary += fmt.Sprintf(" auto_recovery_failures=%d", autoRecoveryFailures)
	}
	lines = appendRuntimeSummaryLine(lines, summary+"。")
	if execSource != "" {
		lines = append(lines, execSource)
	}
	evidence := []string{
		fmt.Sprintf("service=%s active=%s enabled=%s", serviceName, active, enabled),
		fmt.Sprintf("task_status=%s", fallbackValue(taskStatus, "-")),
	}
	if healthURL != "" {
		evidence = append(evidence, "health_url="+healthURL)
	}
	if healthState != "" {
		evidence = append(evidence, "health_state="+healthState)
	}
	if healthCode > 0 {
		evidence = append(evidence, fmt.Sprintf("health_code=%d", healthCode))
	}
	if healthSource != "" {
		evidence = append(evidence, "health_source="+healthSource)
	}
	if lastOperation != "" {
		evidence = append(evidence, "last_operation="+lastOperation)
	}
	if lastOperationAt != "" {
		evidence = append(evidence, "last_operation_at="+lastOperationAt)
	}
	if autoRecovery != "" {
		evidence = append(evidence, "auto_recovery="+autoRecovery)
	}
	if autoRecoveryReason != "" {
		evidence = append(evidence, "auto_recovery_reason="+autoRecoveryReason)
	}
	if alertLevel != "" {
		evidence = append(evidence, "self_heal_alert="+alertLevel)
	}
	if autoRecoveryReason == "release_changed" {
		if trackedReleaseVersion != "" {
			evidence = append(evidence, "tracked_release_version="+trackedReleaseVersion)
		}
		if currentReleaseVersion != "" {
			evidence = append(evidence, "current_release_version="+currentReleaseVersion)
		}
		if trackedReleaseID != "" {
			evidence = append(evidence, "tracked_release_id="+trackedReleaseID)
		}
		if currentReleaseID != "" {
			evidence = append(evidence, "current_release_id="+currentReleaseID)
		}
	}
	if autoRecoveryFailures > 0 {
		evidence = append(evidence, fmt.Sprintf("auto_recovery_failures=%d", autoRecoveryFailures))
	}
	if autoRecoveryCooldownUntil != "" {
		evidence = append(evidence, "auto_recovery_cooldown_until="+autoRecoveryCooldownUntil)
	}
	lines = appendRuntimeEvidenceSection(lines, evidence)

	if workerOnline {
		lines = append(lines, nextStepLine(nextStepWorkerOnline))
	} else {
		lines = append(lines, nextStepLine(nextStepWorkerRecover))
	}
	return strings.Join(lines, "\n")
}

func buildTaskControlStatusConclusion(workerOnline bool, taskStatus string, alertLevel string) string {
	taskStatus = strings.TrimSpace(strings.ToLower(taskStatus))
	alertLevel = strings.TrimSpace(strings.ToLower(alertLevel))
	if workerOnline {
		if taskStatus == "blocked" || taskStatus == "failed" {
			return "worker 服务在线，但任务处于阻塞/失败状态，需先处理任务阻塞再继续推进。"
		}
		return "worker 服务在线，可继续推进任务。"
	}
	if alertLevel == "critical" {
		return "worker 状态异常且自愈已进入高风险阶段，建议立即人工介入恢复。"
	}
	return "worker 当前异常或离线，建议先执行 ensure_running 或 restart 恢复服务。"
}

func normalizeManagedServiceStateToken(raw string) string {
	state := strings.ToLower(strings.TrimSpace(raw))
	if state == "" {
		return ""
	}
	fields := strings.Fields(state)
	if len(fields) == 0 {
		return ""
	}
	candidate := strings.TrimSpace(fields[0])
	switch candidate {
	case "active", "inactive", "failed", "activating", "deactivating", "reloading":
		return candidate
	default:
		return ""
	}
}

func mapTaskControlOperationLabel(operation string) string {
	switch strings.TrimSpace(strings.ToLower(operation)) {
	case "ensure_running":
		return "已确保 worker 服务可用并继续执行任务"
	case "ensure_running_skip":
		return "检测到服务已在线且健康，已复用现有进程并跳过重启"
	case "restart":
		return "已重启 worker 服务"
	case "start":
		return "已启动 worker 服务"
	case "stop":
		return "已停止 worker 服务"
	default:
		return "已执行 worker 控制动作"
	}
}

func mapTaskControlAutoRecoveryStateLabel(state string) string {
	switch strings.TrimSpace(strings.ToLower(state)) {
	case "applied":
		return "已触发 ensure_running 自愈"
	case "failed":
		return "已尝试自动恢复但执行失败"
	case "throttled":
		return "自动恢复已进入冷却窗口，暂不重复重启"
	case "not_needed":
		return "状态正常，无需自动恢复"
	default:
		return "自动恢复状态未知"
	}
}

func mapTaskControlAutoRecoveryReasonLabel(reason string) string {
	switch strings.TrimSpace(strings.ToLower(reason)) {
	case "health_down":
		return "健康探针异常"
	case "service_not_active":
		return "服务未处于 active"
	case "release_changed":
		return "检测到发布版本变更"
	default:
		return ""
	}
}

func mapTaskControlAutoRecoveryAlertLevelAndGuidance(state string) (string, string) {
	switch strings.TrimSpace(strings.ToLower(state)) {
	case "applied":
		return "observing", "已触发自动恢复，建议持续观察健康状态"
	case "failed":
		return "warning", "自动恢复执行失败，建议尽快人工排障"
	case "throttled":
		return "critical", "自动恢复进入冷却窗口，需立即人工介入"
	case "not_needed":
		return "ok", "当前无需恢复动作"
	default:
		return "", ""
	}
}

func buildTaskControlReleaseDeltaSummary(trackedVersion, currentVersion, trackedID, currentID string) string {
	parts := make([]string, 0, 2)
	trackedVersion = strings.TrimSpace(trackedVersion)
	currentVersion = strings.TrimSpace(currentVersion)
	if trackedVersion != "" || currentVersion != "" {
		parts = append(parts, fmt.Sprintf("version %s -> %s", fallbackValue(trackedVersion, "-"), fallbackValue(currentVersion, "-")))
	}
	trackedID = strings.TrimSpace(trackedID)
	currentID = strings.TrimSpace(currentID)
	if trackedID != "" || currentID != "" {
		parts = append(parts, fmt.Sprintf("release_id %s -> %s", fallbackValue(trackedID, "-"), fallbackValue(currentID, "-")))
	}
	return strings.Join(parts, "；")
}

func runtimeTaskControlStatusAutoRecoverEnabled(action actionPlanItem) bool {
	mode := strings.TrimSpace(strings.ToLower(action.Mode))
	switch mode {
	case "query", "query_only", "query-only", "status_only", "status-only", "readonly", "read_only", "read-only":
		return false
	case "auto_recover", "auto-recover", "self_heal", "self-heal", "status_auto_recover", "status-auto-recover", "status.recover", "recover":
		return true
	}
	return parseManagedBool(os.Getenv("CLAWX_RUNTIME_TASK_CONTROL_STATUS_SELF_HEAL"))
}

func shouldAutoRecoverTaskControlStatus(activeState string, healthURL string, healthState string) (bool, string) {
	activeState = normalizeManagedServiceStateToken(activeState)
	healthURL = sanitizeManagedHealthURL(healthURL)
	healthState = strings.TrimSpace(strings.ToLower(healthState))

	if healthURL != "" && healthState == "down" {
		return true, "health_down"
	}
	switch activeState {
	case "inactive", "failed", "activating", "deactivating", "reloading":
		return true, "service_not_active"
	default:
		return false, ""
	}
}

func resolveTaskControlSelfHealBackoffConfig() (int, time.Duration) {
	threshold := parseManagedBoundedInt(
		os.Getenv("CLAWX_RUNTIME_TASK_CONTROL_SELF_HEAL_FAILURE_THRESHOLD"),
		defaultTaskControlSelfHealFailureThreshold,
		1,
		maxTaskControlSelfHealFailureThreshold,
	)
	cooldownSeconds := parseManagedBoundedInt(
		os.Getenv("CLAWX_RUNTIME_TASK_CONTROL_SELF_HEAL_COOLDOWN_SECONDS"),
		int(defaultTaskControlSelfHealCooldown/time.Second),
		0,
		int(maxTaskControlSelfHealCooldown/time.Second),
	)
	return threshold, time.Duration(cooldownSeconds) * time.Second
}

func parseManagedBoundedInt(raw string, fallback int, min int, max int) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	if parsed < min {
		return min
	}
	if parsed > max {
		return max
	}
	return parsed
}

func mapServiceOperationLabel(operation string) string {
	switch strings.TrimSpace(strings.ToLower(operation)) {
	case "restart":
		return "已重启服务"
	case "start":
		return "已启动服务"
	case "stop":
		return "已停止服务"
	case "status":
		return "已查询服务状态"
	default:
		return "已执行服务操作"
	}
}

func mapSupervisorOperationLabel(operation string) string {
	switch strings.TrimSpace(strings.ToLower(operation)) {
	case "ensure":
		return "已安装/更新并启用 supervisor unit"
	case "disable":
		return "已停用 supervisor unit"
	case "status":
		return "已查询 supervisor 状态"
	default:
		return "已执行 supervisor 操作"
	}
}

func extractActionPlanTaskStatusFromControlNote(note string) string {
	pattern := regexp.MustCompile(`当前任务状态为\s*([a-zA-Z0-9_-]+)`)
	match := pattern.FindStringSubmatch(note)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func extractRuntimeServiceStateFromStatusNote(note string) string {
	active := normalizeManagedServiceStateToken(extractActionPlanTokenValueExact(note, "active"))
	if active != "" {
		return active
	}
	output := normalizeManagedServiceStateToken(extractActionPlanTokenValueExact(note, "output"))
	if output != "" {
		return output
	}
	return ""
}

func extractActionPlanTokenValueExact(text string, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	pattern := regexp.MustCompile(`(?:^|[\s；,:：])` + regexp.QuoteMeta(key) + `=([^\s；,，]+)`)
	match := pattern.FindStringSubmatch(strings.TrimSpace(text))
	if len(match) >= 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func parseActionPlanTokenInt(text string, key string) int {
	value := strings.TrimSpace(extractActionPlanTokenValueExact(text, key))
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func splitNonEmptyLines(text string) []string {
	raw := strings.Split(strings.TrimSpace(text), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func containsString(items []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" || len(items) == 0 {
		return false
	}
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}

func extractActionPlanSegmentValue(note string, label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	note = strings.TrimSpace(note)
	start := strings.Index(note, label)
	if start < 0 {
		return ""
	}
	valueStart := start + len(label)
	if valueStart >= len(note) {
		return ""
	}
	end := len(note)
	markers := []string{
		"；task_id=",
		"；目标：",
		"；剩余步骤：",
		"；下一步：",
		"；最近更新：",
		"；最近结果：",
		"；执行统计：",
	}
	for _, marker := range markers {
		if marker == "；"+label {
			continue
		}
		if idx := strings.Index(note[valueStart:], marker); idx >= 0 {
			candidateEnd := valueStart + idx
			if candidateEnd < end {
				end = candidateEnd
			}
		}
	}
	value := strings.TrimSpace(note[valueStart:end])
	value = strings.TrimSuffix(value, "。")
	return strings.TrimSpace(value)
}

func extractActionPlanInlineExecutionSource(note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return ""
	}
	lines := strings.Split(note, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "执行来源：") {
			return strings.TrimSpace(strings.TrimSuffix(line, "。"))
		}
	}
	idx := strings.Index(note, "执行来源：")
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(note[idx:])
	if rest == "" {
		return ""
	}
	end := len(rest)
	for _, marker := range []string{"；", "\n"} {
		if i := strings.Index(rest, marker); i >= 0 && i < end {
			end = i
		}
	}
	line := strings.TrimSpace(rest[:end])
	line = strings.TrimSuffix(line, "。")
	return strings.TrimSpace(line)
}

func extractActionPlanTokenValue(text string, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	text = strings.TrimSpace(text)
	idx := strings.Index(text, key)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(text[idx+len(key):])
	if rest == "" {
		return ""
	}
	for i, r := range rest {
		if unicode.IsSpace(r) {
			return strings.TrimSpace(rest[:i])
		}
	}
	return strings.TrimSpace(rest)
}

func maybeAutoApplyActionPlan(
	ctx context.Context,
	runtime agentRuntime,
	decision service.Decision,
	output string,
	channel string,
	instanceID string,
	scopeKey string,
	overrides *conversationAgentOverrides,
	runtimes map[string]agentRuntime,
	defaultAgentID string,
	fallbackCWD string,
) (actionPlanApplyResult, bool, error) {
	explicitDecisionHint := detectRuntimeExecDecisionHint(decision.Message.Text)
	if strings.TrimSpace(explicitDecisionHint.Mode) != "" {
		persistRuntimeExecDecisionHint(decision.ConversationID, explicitDecisionHint)
	}
	decisionHint := resolveRuntimeExecDecisionHint(decision.ConversationID, decision.Message.Text)
	decisionSource := fallbackValue(strings.TrimSpace(decisionHint.Source), "user_phrase")
	decisionSourceLabel := formatRuntimeExecDecisionApplySource(decisionSource, decision.ConversationID)
	if strings.TrimSpace(decisionHint.Mode) == "paused" {
		statusLine := buildRuntimeExecDecisionStatusLine(decision.ConversationID)
		pauseMessage := strings.Join([]string{
			"runtime_exec_decision 已生效：mode=paused，已按你的指令暂停自动执行。",
			"结论：自动执行已暂停，等待你确认后继续。",
			"完成状态：已暂停（等待确认）。",
			"下一步：回复“继续”恢复自治，或直接发送新的执行指令。",
		}, "\n")
		return actionPlanApplyResult{
			Applied:       false,
			Status:        "applied",
			Message:       prependRuntimeExecDecisionStatusLine(pauseMessage, statusLine),
			ReadOnlyQuery: true,
		}, true, nil
	}

	plan, ok := parseActionPlanFromText(output)
	if !ok {
		if fallbackAction, fallbackOK := inferReleaseStatusActionFromUserText(decision.Message.Text, runtime, runtimes, defaultAgentID); fallbackOK {
			plan = actionPlan{
				Type:   "action_plan",
				Mode:   "execute",
				Reason: "auto_release_status_query",
				Actions: []actionPlanItem{
					fallbackAction,
				},
			}
			ok = true
		} else if fallbackAction, fallbackOK := inferTaskDelegatesActionFromUserText(decision.Message.Text, runtime, runtimes, defaultAgentID); fallbackOK {
			plan = actionPlan{
				Type:   "action_plan",
				Mode:   "execute",
				Reason: "auto_task_delegates_query",
				Actions: []actionPlanItem{
					fallbackAction,
				},
			}
			ok = true
		} else if fallbackAction, fallbackOK := inferTaskStatusActionFromUserText(decision.Message.Text, runtime, runtimes, defaultAgentID); fallbackOK {
			plan = actionPlan{
				Type:   "action_plan",
				Mode:   "execute",
				Reason: "auto_task_status_query",
				Actions: []actionPlanItem{
					fallbackAction,
				},
			}
			ok = true
		} else if fallbackAction, fallbackReason, fallbackOK := inferTaskControlActionFromUserText(decision.Message.Text, runtime, runtimes, defaultAgentID, decision.ConversationID, scopeKey, fallbackCWD); fallbackOK {
			plan = actionPlan{
				Type:   "action_plan",
				Mode:   "execute",
				Reason: fallbackReason,
				Actions: []actionPlanItem{
					fallbackAction,
				},
			}
			ok = true
		}
	}
	if ok {
		plan, _ = enforceStatusTruthQueryPlanFromUserText(
			plan,
			decision.Message.Text,
			runtime,
			runtimes,
			defaultAgentID,
			decision.ConversationID,
			scopeKey,
			fallbackCWD,
		)
	}
	if !ok {
		return actionPlanApplyResult{}, false, nil
	}
	decisionNotes := make([]string, 0, 1)
	switch strings.TrimSpace(decisionHint.Mode) {
	case "clear":
		decisionNotes = append(decisionNotes, "- runtime_exec_decision 已生效：mode=clear，已清除已锁定策略，恢复默认自治。")
	case "blocked.command_override":
		if cmdline := strings.TrimSpace(decisionHint.AlternateCommand); cmdline != "" {
			plan = actionPlan{
				Type:   "action_plan",
				Mode:   "execute",
				Reason: "runtime_exec_decision_override",
				Actions: []actionPlanItem{
					{
						Kind:   "runtime.exec",
						Cmd:    cmdline,
						CWD:    strings.TrimSpace(fallbackCWD),
						Reason: "runtime_exec_decision_override",
					},
				},
			}
			decisionNotes = append(decisionNotes, fmt.Sprintf("- runtime_exec_decision 已生效：mode=blocked.command_override source=%s，已按你指定的替代命令执行。", decisionSourceLabel))
		}
	case "service.retry_only":
		originalCount := len(plan.Actions)
		plan.Actions = constrainActionPlanForRetryOnly(plan.Actions)
		if len(plan.Actions) == 0 {
			serviceName := inferManagedServiceNameForAgent(runtime.agentID)
			action := actionPlanItem{
				Kind:      "runtime.task.control",
				AgentID:   strings.TrimSpace(runtime.agentID),
				Service:   serviceName,
				Operation: "ensure_running",
				Scope:     "user",
			}
			if healthURL := inferTaskControlHealthURL(decision.Message.Text, serviceName, runtime, fallbackCWD, decision.ConversationID, scopeKey); healthURL != "" {
				action.HealthURL = healthURL
			}
			plan.Actions = append(plan.Actions, action)
		}
		dropped := originalCount - len(plan.Actions)
		if dropped > 0 {
			decisionNotes = append(decisionNotes, fmt.Sprintf("- runtime_exec_decision 已生效：mode=service.retry_only source=%s，已跳过 %d 条非重试动作。", decisionSourceLabel, dropped))
		}
	case "build.switch_source":
		rewritten := rewriteBuildSwitchSourceCommands(plan.Actions)
		if rewritten > 0 {
			decisionNotes = append(decisionNotes, fmt.Sprintf("- runtime_exec_decision 已生效：mode=build.switch_source source=%s，已为 %d 条构建命令注入镜像源策略。", decisionSourceLabel, rewritten))
		} else {
			decisionNotes = append(decisionNotes, fmt.Sprintf("- runtime_exec_decision 已生效：mode=build.switch_source source=%s，本轮未识别到可改写的构建命令。", decisionSourceLabel))
		}
	case "service.deep_repair":
		originalCount := len(plan.Actions)
		filtered := constrainActionPlanForServiceDeepRepair(plan.Actions)
		dropped := originalCount - len(filtered)
		plan.Actions = filtered
		fallbackAdded := false
		fallbackSource := ""
		if len(plan.Actions) == 0 {
			fallbackCmd, resolvedSource := resolveRuntimeExecDecisionFallbackCommand(
				"service.deep_repair",
				runtime,
				fallbackCWD,
				defaultServiceDeepRepairDiagnosticCommand(inferManagedServiceNameForAgent(runtime.agentID)),
			)
			fallbackSource = resolvedSource
			plan.Actions = append(plan.Actions, actionPlanItem{
				Kind:           "runtime.exec",
				CWD:            strings.TrimSpace(fallbackCWD),
				Reason:         "runtime_exec_decision_deep_repair_fallback",
				Cmd:            fallbackCmd,
				FallbackSource: fallbackValue(strings.TrimSpace(fallbackSource), "default"),
			})
			fallbackAdded = true
		}
		if dropped > 0 {
			decisionNotes = append(decisionNotes, fmt.Sprintf("- runtime_exec_decision 已生效：mode=service.deep_repair source=%s，已跳过 %d 条非深修动作。", decisionSourceLabel, dropped))
		} else {
			decisionNotes = append(decisionNotes, fmt.Sprintf("- runtime_exec_decision 已生效：mode=service.deep_repair source=%s，继续执行深修链路。", decisionSourceLabel))
		}
		if fallbackAdded {
			decisionNotes = append(decisionNotes, fmt.Sprintf("- runtime_exec_decision 深修兜底：已注入日志诊断命令（fallback_source=%s）。", fallbackValue(strings.TrimSpace(fallbackSource), "default")))
		}
	case "build.continue":
		originalCount := len(plan.Actions)
		filtered := constrainActionPlanForBuildContinue(plan.Actions)
		dropped := originalCount - len(filtered)
		plan.Actions = filtered
		fallbackAdded := false
		fallbackSource := ""
		if len(plan.Actions) == 0 {
			fallbackCmd, resolvedSource := resolveRuntimeExecDecisionFallbackCommand(
				"build.continue",
				runtime,
				fallbackCWD,
				defaultBuildContinueDiagnosticCommand(),
			)
			fallbackSource = resolvedSource
			plan.Actions = append(plan.Actions, actionPlanItem{
				Kind:           "runtime.exec",
				CWD:            strings.TrimSpace(fallbackCWD),
				Reason:         "runtime_exec_decision_build_continue_fallback",
				Cmd:            fallbackCmd,
				FallbackSource: fallbackValue(strings.TrimSpace(fallbackSource), "default"),
			})
			fallbackAdded = true
		}
		if dropped > 0 {
			decisionNotes = append(decisionNotes, fmt.Sprintf("- runtime_exec_decision 已生效：mode=build.continue source=%s，已跳过 %d 条非构建动作。", decisionSourceLabel, dropped))
		} else {
			decisionNotes = append(decisionNotes, fmt.Sprintf("- runtime_exec_decision 已生效：mode=build.continue source=%s，继续执行构建链修复。", decisionSourceLabel))
		}
		if fallbackAdded {
			decisionNotes = append(decisionNotes, fmt.Sprintf("- runtime_exec_decision 构建兜底：已注入构建环境诊断命令（fallback_source=%s）。", fallbackValue(strings.TrimSpace(fallbackSource), "default")))
		}
	}
	if strings.TrimSpace(plan.Reason) != "" && !isSyntheticQueryPlanReason(plan.Reason) {
		setExecutionGoalState(decision.ConversationID, executionGoalState{
			Goal:   strings.TrimSpace(plan.Reason),
			Status: "running",
		})
		appendTrackingGoal(runtime, fallbackCWD, decision.ConversationID, strings.TrimSpace(plan.Reason))
	}
	if strings.TrimSpace(plan.Mode) == "" {
		plan.Mode = "execute"
	}
	notes := make([]string, 0, len(plan.Actions)+1+len(decisionNotes))
	notes = append(notes, decisionNotes...)
	appliedCount := 0
	failedCount := 0
	responseAgentID := strings.TrimSpace(runtime.agentID)
	readOnlyQuery := len(plan.Actions) > 0

	decisionModeHint := normalizeRuntimeExecDecisionMode(decisionHint.Mode)
	_, lockSource := resolveRuntimeExecDecisionLockState(decision.ConversationID)
	decisionModeForExec := ""
	if decisionModeHint != "" && decisionModeHint != "clear" {
		decisionModeForExec = decisionModeHint
	}
	decisionApplySourceForExec := ""
	if decisionModeHint != "" && decisionModeHint != "clear" {
		decisionApplySourceForExec = fallbackValue(strings.TrimSpace(decisionHint.Source), "user_phrase")
	}
	decisionLockSourceForExec := strings.TrimSpace(lockSource)
	if decisionModeForExec != "" && decisionLockSourceForExec == "" {
		decisionLockSourceForExec = "locked_state"
	}
	if decisionModeForExec == "" {
		decisionLockSourceForExec = ""
	}

	execPlan := runtimeExecPlan{
		Type:                           "runtime.exec",
		Mode:                           plan.Mode,
		Reason:                         plan.Reason,
		RuntimeExecDecisionMode:        decisionModeForExec,
		RuntimeExecDecisionApplySource: decisionApplySourceForExec,
		RuntimeExecDecisionLockSource:  decisionLockSourceForExec,
	}
	for _, action := range plan.Actions {
		kind := strings.TrimSpace(strings.ToLower(action.Kind))
		if !isReadOnlyActionPlanItem(kind, action) {
			readOnlyQuery = false
		}
		switch kind {
		case "runtime.exec":
			cmd := strings.TrimSpace(action.Cmd)
			if cmd == "" {
				continue
			}
			execPlan.Commands = append(execPlan.Commands, runtimeExecCommand{
				Cmd:            cmd,
				CWD:            strings.TrimSpace(action.CWD),
				Reason:         strings.TrimSpace(action.Reason),
				FallbackSource: strings.TrimSpace(action.FallbackSource),
			})
		case "runtime.bootstrap":
			workspace := strings.TrimSpace(action.CWD)
			if workspace == "" {
				workspace = strings.TrimSpace(fallbackCWD)
			}
			if workspace == "" {
				workspace = strings.TrimSpace(runtime.cwd)
			}
			if workspace == "" {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "runtime.bootstrap", fmt.Errorf("缺少可用 workspace，已跳过"), action, decision, runtime, defaultAgentID)
				continue
			}
			if err := runtime.cfgSnapshot.ValidateWorkingDirectory(workspace); err != nil {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "runtime.bootstrap", fmt.Errorf("workspace 超出允许范围: %w", err), action, decision, runtime, defaultAgentID)
				continue
			}
			svc := runtimeorchestrator.NewService()
			result, err := svc.Bootstrap(runtimeorchestrator.BootstrapOptions{
				WorkspaceRoot: workspace,
				AgentID:       strings.TrimSpace(runtime.agentID),
				WorkerRoles:   action.WorkerRoles,
			})
			if err != nil {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "runtime.bootstrap", err, action, decision, runtime, defaultAgentID)
				continue
			}
			appliedCount++
			notes = append(notes, fmt.Sprintf("- runtime 初始化完成：workers=%d runtime_dir=%s", result.WorkerCount, result.RuntimeDir))
		case "runtime.service":
			note, err := applyActionPlanRuntimeService(ctx, action)
			if err != nil {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "runtime.service", err, action, decision, runtime, defaultAgentID)
				continue
			}
			appliedCount++
			notes = append(notes, "- "+note)
		case "runtime.release":
			if strings.TrimSpace(action.ConversationID) == "" {
				action.ConversationID = strings.TrimSpace(decision.ConversationID)
			}
			note, err := applyActionPlanRuntimeRelease(ctx, runtime, action, fallbackCWD)
			if err != nil {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "runtime.release", err, action, decision, runtime, defaultAgentID)
				continue
			}
			appliedCount++
			notes = append(notes, "- "+note)
		case "runtime.release.status":
			note, err := applyActionPlanRuntimeReleaseStatus(action, decision.ConversationID)
			if err != nil {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "runtime.release.status", err, action, decision, runtime, defaultAgentID)
				continue
			}
			appliedCount++
			notes = append(notes, "- "+note)
		case "runtime.task.status":
			note, err := applyActionPlanRuntimeTaskStatus(runtime, decision, action, fallbackCWD, runtimes, defaultAgentID)
			if err != nil {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "runtime.task.status", err, action, decision, runtime, defaultAgentID)
				continue
			}
			appliedCount++
			notes = append(notes, "- "+note)
		case "runtime.task.delegate":
			note, err := applyActionPlanRuntimeTaskDelegate(runtime, decision, action, fallbackCWD, runtimes, defaultAgentID)
			if err != nil {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "runtime.task.delegate", err, action, decision, runtime, defaultAgentID)
				continue
			}
			appliedCount++
			notes = append(notes, "- "+note)
		case "runtime.task.delegates":
			note, err := applyActionPlanRuntimeTaskDelegates(runtime, decision, action, fallbackCWD, runtimes, defaultAgentID)
			if err != nil {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "runtime.task.delegates", err, action, decision, runtime, defaultAgentID)
				continue
			}
			appliedCount++
			notes = append(notes, "- "+note)
		case "runtime.task.retry":
			note, err := applyActionPlanRuntimeTaskRetry(runtime, decision, action, fallbackCWD, runtimes, defaultAgentID)
			if err != nil {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "runtime.task.retry", err, action, decision, runtime, defaultAgentID)
				continue
			}
			appliedCount++
			notes = append(notes, "- "+note)
		case "runtime.task.cancel":
			note, err := applyActionPlanRuntimeTaskCancel(runtime, decision, action, fallbackCWD, runtimes, defaultAgentID)
			if err != nil {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "runtime.task.cancel", err, action, decision, runtime, defaultAgentID)
				continue
			}
			appliedCount++
			notes = append(notes, "- "+note)
		case "runtime.task.control":
			note, err := applyActionPlanRuntimeTaskControl(ctx, runtime, decision, action, scopeKey, fallbackCWD, runtimes, defaultAgentID)
			if err != nil {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "runtime.task.control", err, action, decision, runtime, defaultAgentID)
				continue
			}
			if strings.Contains(note, "auto_recovery=applied") || strings.Contains(note, "auto_recovery=failed") {
				readOnlyQuery = false
			}
			appliedCount++
			notes = append(notes, "- "+note)
		case "runtime.supervisor":
			note, err := applyActionPlanRuntimeSupervisor(ctx, runtime, action, fallbackCWD)
			if err != nil {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "runtime.supervisor", err, action, decision, runtime, defaultAgentID)
				continue
			}
			appliedCount++
			notes = append(notes, "- "+note)
		case "agent.use":
			agentID := cleanAgentIDToken(action.AgentID)
			if agentID == "" {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "agent.use", fmt.Errorf("缺少 agent_id，已跳过"), action, decision, runtime, defaultAgentID)
				continue
			}
			applied, ok, err := applyActionPlanAgentUse(agentID, scopeKey, overrides, runtimes, defaultAgentID)
			if err != nil {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "agent.use", err, action, decision, runtime, defaultAgentID)
				continue
			}
			if ok {
				appliedCount++
				agentNote := formatControlApplyResult(applied)
				notes = appendRuntimeActionOutcomeNote(notes, agentNote, action, decision, runtime, defaultAgentID)
				if strings.TrimSpace(applied.Target) != "" {
					responseAgentID = strings.TrimSpace(applied.Target)
				}
			}
		case "requirement.sync":
			req := strings.TrimSpace(action.Requirement)
			if req == "" {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "requirement.sync", fmt.Errorf("缺少 requirement，已跳过"), action, decision, runtime, defaultAgentID)
				continue
			}
			targetAgentID := cleanAgentIDToken(action.AgentID)
			mode := strings.TrimSpace(strings.ToLower(action.Mode))
			if mode == "" {
				mode = "execute"
			}
			reqResult, reqApplied, reqErr := applyActionPlanRequirementSync(ctx, runtime, decision, channel, instanceID, scopeKey, overrides, runtimes, defaultAgentID, targetAgentID, mode, req)
			if reqErr != nil {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "requirement.sync", reqErr, action, decision, runtime, defaultAgentID)
				continue
			}
			if reqApplied {
				appliedCount++
				reqNote := strings.ReplaceAll(formatRequirementSyncResult(reqResult), "\n", " ")
				notes = appendRuntimeActionOutcomeNote(notes, reqNote, action, decision, runtime, defaultAgentID)
				if strings.TrimSpace(reqResult.AgentID) != "" {
					responseAgentID = strings.TrimSpace(reqResult.AgentID)
				}
			}
		case "config.exec":
			commandText := strings.TrimSpace(action.Command)
			if commandText == "" {
				commandText = strings.TrimSpace(action.Cmd)
			}
			if commandText == "" {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "config.exec", fmt.Errorf("缺少 command/cmd，已跳过"), action, decision, runtime, defaultAgentID)
				continue
			}
			if !strings.HasPrefix(strings.TrimSpace(commandText), "/config") {
				commandText = "/config " + strings.TrimSpace(commandText)
			}
			handled, response, cfgErr := handleConfigChatCommand(chatiface.Message{
				Text:           commandText,
				ConversationID: decision.ConversationID,
				UserID:         decision.Message.UserID,
				Channel:        channel,
				ContextFlags:   decision.Message.ContextFlags,
			})
			if cfgErr != nil {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "config.exec", cfgErr, action, decision, runtime, defaultAgentID)
				continue
			}
			if !handled {
				failedCount++
				notes = appendRuntimeActionFailureNote(notes, "config.exec", fmt.Errorf("未被处理: %s", commandText), action, decision, runtime, defaultAgentID)
				continue
			}
			appliedCount++
			cfgNote := strings.ReplaceAll(strings.TrimSpace(response), "\n", " ")
			notes = appendRuntimeActionOutcomeNote(notes, cfgNote, action, decision, runtime, defaultAgentID)
		default:
			failedCount++
			notes = appendRuntimeActionFailureNote(notes, "action.kind", fmt.Errorf("未支持的 action.kind: %s", kind), action, decision, runtime, defaultAgentID)
			continue
		}
	}
	if len(execPlan.Commands) > 0 || strings.TrimSpace(plan.Mode) == "suggest" {
		readOnlyQuery = false
		execResult := applyRuntimeExecPlan(ctx, runtime, execPlan, fallbackCWD, decision.ConversationID)
		if strings.TrimSpace(execResult.Message) != "" {
			notes = append(notes, strings.TrimSpace(execResult.Message))
		}
		if execResult.Applied || execResult.Status == "suggest" {
			appliedCount++
		}
		if execResult.Failed > 0 && execResult.Executed == 0 && execResult.Status != "suggest" {
			failedCount++
		}
		if looksLikeSpecKitWorkflowRequest(strings.ToLower(strings.TrimSpace(decision.Message.Text))) {
			if missing, specDir := findMissingSpecKitDocs(execResult.SuccessInfo, collectSpecKitDocRootsFromCommands(execPlan.Commands, fallbackCWD)); len(missing) > 0 {
				setSpecKitFlowActive(decision.ConversationID, true)
				failedCount++
				notes = append(notes, "- "+formatSpecKitGateNote(specDir, missing))
			} else {
				setSpecKitFlowActive(decision.ConversationID, false)
			}
		}
	} else if len(plan.Actions) == 0 {
		readOnlyQuery = false
		failedCount++
		notes = appendRuntimeActionFailureNote(notes, "action_plan", fmt.Errorf("action_plan 未包含 actions"), actionPlanItem{}, decision, runtime, defaultAgentID)
	}

	status := "applied"
	if appliedCount == 0 && failedCount > 0 {
		status = "failed"
	} else if appliedCount > 0 && failedCount > 0 {
		status = "partial"
	} else if appliedCount == 0 && failedCount == 0 {
		status = "skipped"
	}
	msg := renderActionPlanUserMessage(status, notes, appliedCount, failedCount)
	msg = prependRuntimeExecDecisionStatusLine(msg, buildRuntimeExecDecisionStatusLine(decision.ConversationID))
	if strings.TrimSpace(plan.Reason) != "" && !isSyntheticQueryPlanReason(plan.Reason) {
		goalStatus := status
		if containsTaskControlMutation(plan.Actions) {
			switch status {
			case "applied":
				goalStatus = "running"
			case "partial", "failed":
				goalStatus = "blocked"
			}
		}
		setExecutionGoalState(decision.ConversationID, executionGoalState{
			Goal:       strings.TrimSpace(plan.Reason),
			Status:     goalStatus,
			LastResult: summarizeText(msg, 220),
		})
	}
	return actionPlanApplyResult{
		Applied:         appliedCount > 0,
		Status:          status,
		Message:         strings.TrimSpace(msg),
		ResponseAgentID: strings.TrimSpace(responseAgentID),
		ReadOnlyQuery:   readOnlyQuery && appliedCount > 0 && failedCount == 0,
	}, true, nil
}

func isReadOnlyActionPlanItem(kind string, action actionPlanItem) bool {
	kind = strings.TrimSpace(strings.ToLower(kind))
	switch kind {
	case "runtime.release.status", "runtime.task.status", "runtime.task.delegates":
		return true
	case "runtime.task.control":
		operation, err := normalizeManagedTaskControlOperation(action.Operation)
		return err == nil && operation == "status"
	default:
		return false
	}
}

func containsTaskControlMutation(actions []actionPlanItem) bool {
	for _, action := range actions {
		kind := strings.TrimSpace(strings.ToLower(action.Kind))
		if kind != "runtime.task.control" {
			continue
		}
		operation, err := normalizeManagedTaskControlOperation(action.Operation)
		if err != nil {
			continue
		}
		if operation != "status" {
			return true
		}
	}
	return false
}

func findMissingSpecKitDocs(successInfo []string, extraDocRoots []string) ([]string, string) {
	docRoots := make([]string, 0, 2)
	seen := map[string]struct{}{}
	for _, item := range successInfo {
		matches := specKitDocPathPattern.FindAllStringSubmatch(item, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			path := strings.TrimSpace(match[1])
			if path == "" {
				continue
			}
			root := filepath.Dir(path)
			if root == "" {
				continue
			}
			if _, ok := seen[root]; ok {
				continue
			}
			seen[root] = struct{}{}
			docRoots = append(docRoots, root)
		}
	}
	for _, root := range extraDocRoots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		if _, ok := seen[root]; ok {
			continue
		}
		seen[root] = struct{}{}
		docRoots = append(docRoots, root)
	}
	if len(docRoots) == 0 {
		return nil, ""
	}
	required := []string{"SPEC.md", "PLAN.md", "TASKS.md", "ANALYZE.md"}
	for _, root := range docRoots {
		missing := make([]string, 0, len(required))
		for _, name := range required {
			p := filepath.Join(root, name)
			if _, err := os.Stat(p); err != nil {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			return missing, root
		}
	}
	return nil, docRoots[0]
}

func collectSpecKitDocRootsFromCommands(commands []runtimeExecCommand, fallbackCWD string) []string {
	roots := make([]string, 0, 2)
	seen := map[string]struct{}{}
	for _, command := range commands {
		cmdline := strings.TrimSpace(command.Cmd)
		if cmdline == "" {
			continue
		}
		execCWD := strings.TrimSpace(command.CWD)
		if execCWD == "" {
			execCWD = strings.TrimSpace(fallbackCWD)
		}

		if absMatches := specKitDirPathPattern.FindAllStringSubmatch(cmdline, -1); len(absMatches) > 0 {
			for _, match := range absMatches {
				if len(match) < 2 {
					continue
				}
				root := filepath.Clean(strings.TrimSpace(match[1]))
				if _, ok := seen[root]; ok {
					continue
				}
				seen[root] = struct{}{}
				roots = append(roots, root)
			}
		}
		if relMatches := specKitDirRelPattern.FindAllStringSubmatch(cmdline, -1); len(relMatches) > 0 {
			for _, match := range relMatches {
				if len(match) < 2 {
					continue
				}
				feature := strings.TrimSpace(match[1])
				if feature == "" {
					continue
				}
				root := filepath.Clean(filepath.Join(execCWD, "docs", "spec-kit", feature))
				if _, ok := seen[root]; ok {
					continue
				}
				seen[root] = struct{}{}
				roots = append(roots, root)
			}
		}
	}
	return roots
}

func applyActionPlanAgentUse(
	agentID string,
	scopeKey string,
	overrides *conversationAgentOverrides,
	runtimes map[string]agentRuntime,
	defaultAgentID string,
) (controlApplyResult, bool, error) {
	command := "/agent use " + strings.TrimSpace(agentID)
	handled, response, err := handleAgentChatCommand(chatiface.Message{Text: command}, scopeKey, overrides, runtimes, defaultAgentID)
	if err != nil {
		return controlApplyResult{}, false, err
	}
	if !handled {
		return controlApplyResult{}, false, nil
	}
	status := "applied"
	if strings.Contains(response, "无需切换") {
		status = "noop"
	}
	return controlApplyResult{
		Action:  "agent_use",
		Target:  strings.TrimSpace(agentID),
		Status:  status,
		Message: strings.TrimSpace(response),
	}, true, nil
}

func applyActionPlanRequirementSync(
	ctx context.Context,
	runtime agentRuntime,
	decision service.Decision,
	channel string,
	instanceID string,
	scopeKey string,
	overrides *conversationAgentOverrides,
	runtimes map[string]agentRuntime,
	defaultAgentID string,
	targetAgentID string,
	mode string,
	requirement string,
) (requirementSyncResult, bool, error) {
	targetRuntime := runtime
	if mode == "suggest" {
		suggestAgent := strings.TrimSpace(targetAgentID)
		if suggestAgent == "" {
			suggestAgent = strings.TrimSpace(runtime.agentID)
		}
		message := "已识别需求更新建议。请确认是否继续执行 requirement.sync。"
		if suggestAgent != "" && suggestAgent != strings.TrimSpace(runtime.agentID) {
			message = fmt.Sprintf("已识别需求更新建议。请确认是否切换到 `%s` 并执行 requirement.sync。", suggestAgent)
		}
		return requirementSyncResult{
			Status:  "suggest",
			Source:  "action_plan",
			AgentID: suggestAgent,
			Message: message,
		}, true, nil
	}

	switchMsg := ""
	targetAgentID = strings.TrimSpace(targetAgentID)
	if targetAgentID != "" {
		if _, exists := runtimes[targetAgentID]; !exists {
			return requirementSyncResult{
				Status:  "skipped",
				Source:  "action_plan",
				Message: "requirement.sync 指定的 agent_id 不存在，未更新文档。",
			}, true, nil
		}
		if targetAgentID != strings.TrimSpace(runtime.agentID) {
			applied, ok, err := applyActionPlanAgentUse(targetAgentID, scopeKey, overrides, runtimes, defaultAgentID)
			if err != nil {
				return requirementSyncResult{}, true, err
			}
			if ok {
				switchMsg = strings.TrimSpace(formatControlApplyResult(applied))
			}
		}
		targetRuntime = selectRuntime(runtimes, defaultAgentID, targetAgentID)
	}

	root, _ := resolveRequirementWorkspace(ctx, targetRuntime, decision)
	if strings.TrimSpace(root) == "" {
		return requirementSyncResult{
			Status:  "skipped",
			Source:  "action_plan",
			AgentID: strings.TrimSpace(targetRuntime.agentID),
			Message: "检测到 requirement.sync，但当前未解析到可写 workspace，未落盘。请检查 agent workspace 配置。",
		}, true, nil
	}
	if err := maybeSyncRequirementDocsWithText(ctx, targetRuntime, decision, requirement, true); err != nil {
		emitTrace("requirement_doc_sync", map[string]any{
			"channel":         strings.TrimSpace(channel),
			"instance":        strings.TrimSpace(instanceID),
			"agent_id":        strings.TrimSpace(runtime.agentID),
			"conversation_id": strings.TrimSpace(decision.ConversationID),
			"project_id":      strings.TrimSpace(decision.ProjectID),
			"status":          "error",
			"source":          "action_plan",
			"error":           tracePreview(err.Error(), 240),
		})
		return requirementSyncResult{}, true, err
	}
	if contains, err := markdownContains(filepath.Join(root, "USER.md"), requirement); err == nil && !contains {
		return requirementSyncResult{
			Status:    "skipped",
			Source:    "action_plan",
			Workspace: root,
			AgentID:   strings.TrimSpace(targetRuntime.agentID),
			Message:   "requirement.sync 已识别，但未检测到 USER.md 内容变化。",
		}, true, nil
	}
	msg := "已根据 action_plan.requirement.sync 更新需求文档"
	if switchMsg != "" && !strings.Contains(switchMsg, "无需切换") {
		msg = switchMsg + "\n" + msg
	}
	return requirementSyncResult{
		Status:    "applied",
		Source:    "action_plan",
		Workspace: root,
		AgentID:   strings.TrimSpace(targetRuntime.agentID),
		Message:   msg,
	}, true, nil
}

func applyActionPlanRuntimeService(ctx context.Context, action actionPlanItem) (string, error) {
	service, err := normalizeManagedServiceName(action.Service)
	if err != nil {
		return "", err
	}
	if !isManagedServiceAllowed(service) {
		return "", fmt.Errorf("service %q 不在允许名单中（设置 CLAWX_RUNTIME_SERVICE_ALLOWLIST 以放行）", service)
	}

	scope := strings.ToLower(strings.TrimSpace(action.Scope))
	if scope == "" {
		scope = "user"
	}
	if scope != "user" {
		return "", fmt.Errorf("仅支持 scope=user 的受控服务动作")
	}

	operation, err := normalizeManagedServiceOperation(action.Operation)
	if err != nil {
		return "", err
	}
	timeout := normalizeManagedServiceTimeout(action.TimeoutSec)
	if operation != "status" {
		if err := enforceManagedActionApproval(action, "runtime.service", service); err != nil {
			return "", err
		}
		lock, err := acquireManagedServiceLock(ctx, service, timeout)
		if err != nil {
			return "", err
		}
		defer lock.Release()
	}

	command := operation
	if operation == "status" {
		command = "is-active"
	}

	output, err := runManagedSystemctlUser(ctx, timeout, command, service+".service")
	if err != nil {
		return "", err
	}
	if (operation == "start" || operation == "restart") && strings.TrimSpace(action.HealthURL) != "" {
		if err := waitManagedServiceHealthy(ctx, strings.TrimSpace(action.HealthURL), timeout); err != nil {
			return "", err
		}
		if strings.TrimSpace(output) == "" {
			return fmt.Sprintf("runtime.service 完成：service=%s operation=%s health=%s", service, operation, strings.TrimSpace(action.HealthURL)), nil
		}
		return fmt.Sprintf("runtime.service 完成：service=%s operation=%s output=%s health=%s", service, operation, summarizeActionPlanCommandOutput(output), strings.TrimSpace(action.HealthURL)), nil
	}
	if strings.TrimSpace(output) == "" {
		return fmt.Sprintf("runtime.service 完成：service=%s operation=%s", service, operation), nil
	}
	return fmt.Sprintf("runtime.service 完成：service=%s operation=%s output=%s", service, operation, summarizeActionPlanCommandOutput(output)), nil
}

func applyActionPlanRuntimeServiceWithPortRecovery(ctx context.Context, action actionPlanItem) (string, error) {
	serviceNote, err := applyActionPlanRuntimeService(ctx, action)
	if err == nil {
		return serviceNote, nil
	}
	if !isManagedServicePortConflictError(err) {
		return "", err
	}

	service, serviceErr := normalizeManagedServiceName(action.Service)
	if serviceErr != nil {
		service = strings.TrimSpace(strings.ToLower(strings.TrimSuffix(action.Service, ".service")))
	}
	operation, opErr := normalizeManagedServiceOperation(action.Operation)
	if opErr != nil {
		operation = strings.TrimSpace(strings.ToLower(action.Operation))
	}
	if operation == "" {
		operation = "restart"
	}

	timeout := normalizeManagedServiceTimeout(action.TimeoutSec)
	healthURL := sanitizeManagedHealthURL(action.HealthURL)
	probeTimeout := managedServiceRecoveryProbeTimeout(timeout)
	if healthURL != "" {
		if probeErr := waitManagedServiceHealthy(ctx, healthURL, probeTimeout); probeErr == nil {
			return fmt.Sprintf("runtime.service 完成：service=%s operation=%s output=port_conflict_reused_existing health=%s", service, operation, healthURL), nil
		}
	}

	if operation == "start" || operation == "restart" {
		stopAction := action
		stopAction.Operation = "stop"
		stopAction.HealthURL = ""
		if _, stopErr := applyActionPlanRuntimeService(ctx, stopAction); stopErr == nil {
			startAction := action
			startAction.Operation = "start"
			startNote, startErr := applyActionPlanRuntimeService(ctx, startAction)
			if startErr == nil {
				startNote = strings.TrimSpace(startNote)
				if startNote == "" {
					startNote = fmt.Sprintf("runtime.service 完成：service=%s operation=start", service)
				}
				return strings.TrimSpace(startNote + " recovery=port_conflict_stop_start"), nil
			}
		}
	}
	return "", err
}

func managedServiceRecoveryProbeTimeout(timeout time.Duration) time.Duration {
	switch {
	case timeout <= 0:
		return 4 * time.Second
	case timeout < 4*time.Second:
		return timeout
	case timeout > 12*time.Second:
		return 12 * time.Second
	default:
		return timeout
	}
}

func isManagedServicePortConflictError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "address already in use") ||
		strings.Contains(lower, "eaddrinuse") ||
		strings.Contains(lower, "bind:") ||
		strings.Contains(lower, "failed to listen")
}

func applyActionPlanRuntimeSupervisor(ctx context.Context, runtime agentRuntime, action actionPlanItem, fallbackCWD string) (string, error) {
	service, err := normalizeManagedServiceName(action.Service)
	if err != nil {
		return "", err
	}
	if !isManagedServiceAllowed(service) {
		return "", fmt.Errorf("service %q 不在允许名单中（设置 CLAWX_RUNTIME_SERVICE_ALLOWLIST 以放行）", service)
	}

	scope := strings.ToLower(strings.TrimSpace(action.Scope))
	if scope == "" {
		scope = "user"
	}
	if scope != "user" {
		return "", fmt.Errorf("仅支持 scope=user 的受控 supervisor 动作")
	}

	operation, err := normalizeManagedSupervisorOperation(action.Operation)
	if err != nil {
		return "", err
	}
	timeout := normalizeManagedServiceTimeout(action.TimeoutSec)

	if operation != "status" {
		if err := enforceManagedActionApproval(action, "runtime.supervisor", service); err != nil {
			return "", err
		}
		lock, err := acquireManagedServiceLock(ctx, service, timeout)
		if err != nil {
			return "", err
		}
		defer lock.Release()
	}

	switch operation {
	case "status":
		enabled := queryManagedSystemctlUserState(ctx, timeout, "is-enabled", service+".service")
		active := queryManagedSystemctlUserState(ctx, timeout, "is-active", service+".service")
		return fmt.Sprintf("runtime.supervisor 状态：service=%s enabled=%s active=%s", service, enabled, active), nil
	case "ensure":
		unitPath, err := resolveManagedSupervisorUnitPath(runtime, strings.TrimSpace(action.CWD), fallbackCWD, strings.TrimSpace(action.UnitFile), service)
		if err != nil {
			return "", err
		}
		unitChanged, unitDest, err := installManagedSystemdUserUnit(service, unitPath)
		if err != nil {
			return "", err
		}
		if _, err := runManagedSystemctlUser(ctx, timeout, "daemon-reload"); err != nil {
			return "", err
		}
		output, err := runManagedSystemctlUser(ctx, timeout, "enable", "--now", service+".service")
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(action.HealthURL) != "" {
			if err := waitManagedServiceHealthy(ctx, strings.TrimSpace(action.HealthURL), timeout); err != nil {
				return "", err
			}
		}
		changeStatus := "updated"
		if !unitChanged {
			changeStatus = "unchanged"
		}
		if strings.TrimSpace(output) == "" {
			if strings.TrimSpace(action.HealthURL) == "" {
				return fmt.Sprintf("runtime.supervisor 完成：service=%s operation=%s unit=%s unit_status=%s", service, operation, unitDest, changeStatus), nil
			}
			return fmt.Sprintf("runtime.supervisor 完成：service=%s operation=%s unit=%s unit_status=%s health=%s", service, operation, unitDest, changeStatus, strings.TrimSpace(action.HealthURL)), nil
		}
		if strings.TrimSpace(action.HealthURL) == "" {
			return fmt.Sprintf("runtime.supervisor 完成：service=%s operation=%s unit=%s unit_status=%s output=%s", service, operation, unitDest, changeStatus, summarizeActionPlanCommandOutput(output)), nil
		}
		return fmt.Sprintf("runtime.supervisor 完成：service=%s operation=%s unit=%s unit_status=%s output=%s health=%s", service, operation, unitDest, changeStatus, summarizeActionPlanCommandOutput(output), strings.TrimSpace(action.HealthURL)), nil
	case "disable":
		output, err := runManagedSystemctlUser(ctx, timeout, "disable", "--now", service+".service")
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(output) == "" {
			return fmt.Sprintf("runtime.supervisor 完成：service=%s operation=%s", service, operation), nil
		}
		return fmt.Sprintf("runtime.supervisor 完成：service=%s operation=%s output=%s", service, operation, summarizeActionPlanCommandOutput(output)), nil
	default:
		return "", fmt.Errorf("runtime.supervisor operation 不支持: %q", action.Operation)
	}
}

func applyActionPlanRuntimeTaskControl(
	ctx context.Context,
	runtime agentRuntime,
	decision service.Decision,
	action actionPlanItem,
	scopeKey string,
	fallbackCWD string,
	runtimes map[string]agentRuntime,
	defaultAgentID string,
) (string, error) {
	scope := strings.ToLower(strings.TrimSpace(action.Scope))
	if scope == "" {
		scope = "user"
	}
	if scope != "user" {
		return "", fmt.Errorf("仅支持 scope=user 的任务控制动作")
	}

	targetAgentID := cleanAgentIDToken(action.AgentID)
	if targetAgentID == "" {
		targetAgentID = strings.TrimSpace(runtime.agentID)
	}
	if targetAgentID == "" {
		targetAgentID = strings.TrimSpace(defaultAgentID)
	}
	targetRuntime := runtime
	if selected := selectRuntime(runtimes, defaultAgentID, targetAgentID); strings.TrimSpace(selected.agentID) != "" {
		targetRuntime = selected
	}
	if strings.TrimSpace(targetRuntime.agentID) == "" {
		targetRuntime.agentID = targetAgentID
	}
	if strings.TrimSpace(targetAgentID) == "" {
		targetAgentID = strings.TrimSpace(targetRuntime.agentID)
	}

	serviceCandidate := strings.TrimSpace(action.Service)
	if serviceCandidate == "" {
		serviceCandidate = inferManagedServiceNameForAgent(targetAgentID)
	}
	serviceName, err := normalizeManagedServiceName(serviceCandidate)
	if err != nil {
		return "", err
	}

	operation, err := normalizeManagedTaskControlOperation(action.Operation)
	if err != nil {
		return "", err
	}

	baseAction := action
	baseAction.Service = serviceName
	baseAction.AgentID = targetAgentID
	baseAction.Scope = scope

	switch operation {
	case "status":
		conversationID := strings.TrimSpace(action.ConversationID)
		if conversationID == "" {
			conversationID = strings.TrimSpace(decision.ConversationID)
		}
		timeout := normalizeManagedServiceTimeout(action.TimeoutSec)
		healthURL, healthSource := resolveTaskControlStatusHealthProbe(
			targetRuntime,
			fallbackCWD,
			serviceName,
			conversationID,
			scopeKey,
			decision.Message.Text,
			strings.TrimSpace(action.HealthURL),
		)
		healthState, healthCode := probeTaskControlHealthOnce(ctx, healthURL, timeout)
		lastOperation, lastOperationAt := lookupTaskControlLastAction(targetRuntime, fallbackCWD, serviceName, conversationID, scopeKey)
		currentReleaseID, currentReleaseVersion := lookupManagedServiceCurrentRelease(serviceName)
		trackedReleaseID, trackedReleaseVersion := lookupTaskControlTrackedRelease(targetRuntime, fallbackCWD, serviceName)
		releaseChanged := false
		switch {
		case strings.TrimSpace(currentReleaseID) != "":
			releaseChanged = !strings.EqualFold(strings.TrimSpace(currentReleaseID), strings.TrimSpace(trackedReleaseID))
		case strings.TrimSpace(currentReleaseVersion) != "":
			releaseChanged = !strings.EqualFold(strings.TrimSpace(currentReleaseVersion), strings.TrimSpace(trackedReleaseVersion))
		}

		taskStatusNote, err := applyActionPlanRuntimeTaskStatus(targetRuntime, decision, actionPlanItem{
			Kind:           "runtime.task.status",
			AgentID:        targetAgentID,
			ConversationID: conversationID,
			Scope:          scope,
		}, fallbackCWD, runtimes, defaultAgentID)
		if err != nil {
			return "", err
		}
		serviceStatusNote, err := applyActionPlanRuntimeService(ctx, actionPlanItem{
			Kind:       "runtime.service",
			Service:    serviceName,
			Operation:  "status",
			Scope:      scope,
			TimeoutSec: action.TimeoutSec,
		})
		if err != nil {
			return "", err
		}
		activeState := extractRuntimeServiceStateFromStatusNote(serviceStatusNote)
		autoRecoveryState := ""
		autoRecoveryReason := ""
		autoRecoveryNote := ""
		autoRecoveryError := ""
		autoRecoveryFailures := 0
		autoRecoveryCooldownUntil := ""
		if runtimeTaskControlStatusAutoRecoverEnabled(action) {
			autoRecoveryState = "not_needed"
			threshold, cooldownWindow := resolveTaskControlSelfHealBackoffConfig()
			now := time.Now().UTC()
			failureCount, cooldownUntil := lookupTaskControlSelfHealBackoff(targetRuntime, fallbackCWD, serviceName)
			shouldRecover, reason := shouldAutoRecoverTaskControlStatus(activeState, healthURL, healthState)
			if !shouldRecover && releaseChanged {
				shouldRecover = true
				reason = "release_changed"
			}
			if shouldRecover {
				autoRecoveryReason = reason
				if !cooldownUntil.IsZero() && cooldownUntil.After(now) {
					autoRecoveryState = "throttled"
					autoRecoveryFailures = failureCount
					autoRecoveryCooldownUntil = cooldownUntil.Format(time.RFC3339)
					persistTaskControlSelfHealBackoff(targetRuntime, fallbackCWD, conversationID, serviceName, autoRecoveryState, autoRecoveryReason, "", failureCount, cooldownUntil)
				} else {
					ensureAction := baseAction
					ensureAction.Operation = "ensure_running"
					ensureAction.HealthURL = healthURL
					ensureAction.TimeoutSec = action.TimeoutSec
					ensureAction.ConversationID = conversationID
					recoverNote, recoverErr := applyActionPlanRuntimeTaskControl(ctx, targetRuntime, decision, ensureAction, scopeKey, fallbackCWD, runtimes, defaultAgentID)
					if recoverErr != nil {
						autoRecoveryState = "failed"
						autoRecoveryError = summarizeActionPlanCommandOutput(recoverErr.Error())
						failureCount++
						autoRecoveryFailures = failureCount
						nextCooldownUntil := time.Time{}
						if cooldownWindow > 0 && failureCount >= threshold {
							nextCooldownUntil = now.Add(cooldownWindow)
							autoRecoveryCooldownUntil = nextCooldownUntil.Format(time.RFC3339)
						}
						persistTaskControlSelfHealBackoff(targetRuntime, fallbackCWD, conversationID, serviceName, autoRecoveryState, autoRecoveryReason, autoRecoveryError, failureCount, nextCooldownUntil)
					} else {
						autoRecoveryState = "applied"
						autoRecoveryNote = recoverNote
						autoRecoveryFailures = 0
						persistTaskControlSelfHealBackoff(targetRuntime, fallbackCWD, conversationID, serviceName, autoRecoveryState, autoRecoveryReason, "", 0, time.Time{})
						if refreshedTaskNote, refreshErr := applyActionPlanRuntimeTaskStatus(targetRuntime, decision, actionPlanItem{
							Kind:           "runtime.task.status",
							AgentID:        targetAgentID,
							ConversationID: conversationID,
							Scope:          scope,
						}, fallbackCWD, runtimes, defaultAgentID); refreshErr == nil && strings.TrimSpace(refreshedTaskNote) != "" {
							taskStatusNote = refreshedTaskNote
						}
						if refreshedServiceNote, refreshErr := applyActionPlanRuntimeService(ctx, actionPlanItem{
							Kind:       "runtime.service",
							Service:    serviceName,
							Operation:  "status",
							Scope:      scope,
							TimeoutSec: action.TimeoutSec,
						}); refreshErr == nil && strings.TrimSpace(refreshedServiceNote) != "" {
							serviceStatusNote = refreshedServiceNote
							activeState = extractRuntimeServiceStateFromStatusNote(serviceStatusNote)
						}
						healthState, healthCode = probeTaskControlHealthOnce(ctx, healthURL, timeout)
						lastOperation, lastOperationAt = lookupTaskControlLastAction(targetRuntime, fallbackCWD, serviceName, conversationID, scopeKey)
					}
				}
			} else if failureCount > 0 || !cooldownUntil.IsZero() {
				persistTaskControlSelfHealBackoff(targetRuntime, fallbackCWD, conversationID, serviceName, autoRecoveryState, "", "", 0, time.Time{})
			}
		}

		headParts := []string{
			fmt.Sprintf("查询结果：runtime.task.control 状态 agent=%s service=%s", fallbackValue(strings.TrimSpace(targetAgentID), "main"), serviceName),
		}
		if healthURL != "" {
			headParts = append(headParts, "health_url="+healthURL)
		}
		if healthState != "" {
			headParts = append(headParts, "health_state="+healthState)
		}
		if healthCode > 0 {
			headParts = append(headParts, fmt.Sprintf("health_code=%d", healthCode))
		}
		if src := strings.TrimSpace(strings.ToLower(healthSource)); src != "" {
			headParts = append(headParts, "health_source="+src)
		}
		if op := strings.TrimSpace(strings.ToLower(lastOperation)); op != "" {
			headParts = append(headParts, "last_operation="+op)
		}
		if at := strings.TrimSpace(lastOperationAt); at != "" {
			headParts = append(headParts, "last_operation_at="+at)
		}
		if autoRecoveryState != "" {
			headParts = append(headParts, "auto_recovery="+autoRecoveryState)
		}
		if autoRecoveryReason != "" {
			headParts = append(headParts, "auto_recovery_reason="+autoRecoveryReason)
		}
		if autoRecoveryFailures > 0 {
			headParts = append(headParts, fmt.Sprintf("auto_recovery_failures=%d", autoRecoveryFailures))
		}
		if autoRecoveryCooldownUntil != "" {
			headParts = append(headParts, "auto_recovery_cooldown_until="+autoRecoveryCooldownUntil)
		}
		if releaseChanged {
			headParts = append(headParts,
				"tracked_release_id="+fallbackValue(strings.TrimSpace(trackedReleaseID), "-"),
				"current_release_id="+fallbackValue(strings.TrimSpace(currentReleaseID), "-"),
				"tracked_release_version="+fallbackValue(strings.TrimSpace(trackedReleaseVersion), "-"),
				"current_release_version="+fallbackValue(strings.TrimSpace(currentReleaseVersion), "-"),
			)
		}
		segments := []string{strings.Join(headParts, " "), taskStatusNote, serviceStatusNote}
		if strings.TrimSpace(autoRecoveryNote) != "" {
			segments = append(segments, "自动恢复："+autoRecoveryNote)
		}
		if strings.TrimSpace(autoRecoveryError) != "" {
			segments = append(segments, "自动恢复失败："+autoRecoveryError)
		}
		return strings.Join(segments, "；"), nil
	case "ensure_running":
		healthURL := sanitizeManagedHealthURL(action.HealthURL)
		notes := make([]string, 0, 3)
		supervisorAction := baseAction
		supervisorAction.Kind = "runtime.supervisor"
		supervisorAction.Operation = "ensure"

		explicitUnitFile := strings.TrimSpace(action.UnitFile)
		if explicitUnitFile != "" {
			supervisorAction.UnitFile = explicitUnitFile
			note, err := applyActionPlanRuntimeSupervisor(ctx, targetRuntime, supervisorAction, fallbackCWD)
			if err != nil {
				return "", err
			}
			notes = append(notes, note)
		} else if detectedUnit, err := resolveManagedSupervisorUnitPath(targetRuntime, strings.TrimSpace(action.CWD), fallbackCWD, "", serviceName); err == nil {
			supervisorAction.UnitFile = detectedUnit
			note, err := applyActionPlanRuntimeSupervisor(ctx, targetRuntime, supervisorAction, fallbackCWD)
			if err != nil {
				return "", err
			}
			notes = append(notes, note)
		}

		timeout := normalizeManagedServiceTimeout(action.TimeoutSec)
		currentReleaseID, currentReleaseVersion := lookupManagedServiceCurrentRelease(serviceName)
		trackedReleaseID, trackedReleaseVersion := lookupTaskControlTrackedRelease(targetRuntime, fallbackCWD, serviceName)
		releaseChanged := false
		switch {
		case strings.TrimSpace(currentReleaseID) != "":
			releaseChanged = !strings.EqualFold(strings.TrimSpace(currentReleaseID), strings.TrimSpace(trackedReleaseID))
		case strings.TrimSpace(currentReleaseVersion) != "":
			releaseChanged = !strings.EqualFold(strings.TrimSpace(currentReleaseVersion), strings.TrimSpace(trackedReleaseVersion))
		}
		preStatusNote, preStatusErr := applyActionPlanRuntimeService(ctx, actionPlanItem{
			Kind:       "runtime.service",
			Service:    serviceName,
			Operation:  "status",
			Scope:      scope,
			TimeoutSec: action.TimeoutSec,
		})
		activeState := ""
		if preStatusErr == nil {
			activeState = extractRuntimeServiceStateFromStatusNote(preStatusNote)
		}
		healthState, healthCode := probeTaskControlHealthOnce(ctx, healthURL, timeout)

		shouldSkipRestart := strings.EqualFold(activeState, "active") &&
			healthURL != "" &&
			strings.EqualFold(healthState, "up") &&
			!releaseChanged
		serviceNote := ""
		if shouldSkipRestart {
			serviceNote = fmt.Sprintf("runtime.service 完成：service=%s operation=ensure_running_skip output=already_active", serviceName)
			if healthURL != "" {
				serviceNote += " health=" + healthURL
			}
			if healthState != "" {
				serviceNote += " health_state=" + healthState
			}
			if healthCode > 0 {
				serviceNote += fmt.Sprintf(" health_code=%d", healthCode)
			}
		} else {
			if releaseChanged {
				notes = append(notes, fmt.Sprintf(
					"runtime.task.control 检测到发布版本变更：service=%s tracked_release_id=%s current_release_id=%s tracked_version=%s current_version=%s",
					serviceName,
					fallbackValue(strings.TrimSpace(trackedReleaseID), "-"),
					fallbackValue(strings.TrimSpace(currentReleaseID), "-"),
					fallbackValue(strings.TrimSpace(trackedReleaseVersion), "-"),
					fallbackValue(strings.TrimSpace(currentReleaseVersion), "-"),
				))
			}
			var err error
			serviceNote, err = applyActionPlanRuntimeServiceWithPortRecovery(ctx, actionPlanItem{
				Kind:          "runtime.service",
				Service:       serviceName,
				Operation:     "restart",
				Scope:         scope,
				HealthURL:     healthURL,
				TimeoutSec:    action.TimeoutSec,
				ApprovalToken: strings.TrimSpace(action.ApprovalToken),
			})
			if err != nil {
				return "", err
			}
		}
		notes = append(notes, serviceNote)

		conversationID := strings.TrimSpace(action.ConversationID)
		if conversationID == "" {
			conversationID = strings.TrimSpace(decision.ConversationID)
		}
		taskStatusNote, err := applyActionPlanRuntimeTaskStatus(targetRuntime, decision, actionPlanItem{
			Kind:           "runtime.task.status",
			AgentID:        targetAgentID,
			ConversationID: conversationID,
			Scope:          scope,
		}, fallbackCWD, runtimes, defaultAgentID)
		if err == nil && strings.TrimSpace(taskStatusNote) != "" {
			notes = append(notes, taskStatusNote)
		}

		recordedOperation := "ensure_running"
		if shouldSkipRestart {
			recordedOperation = "ensure_running_skip"
		}
		persistTaskControlActionHint(targetRuntime, fallbackCWD, conversationID, scopeKey, serviceName, recordedOperation, healthURL, currentReleaseID, currentReleaseVersion)
		if conversationID != "" {
			setExecutionGoalState(conversationID, executionGoalState{
				AgentID:    strings.TrimSpace(targetAgentID),
				Status:     "running",
				LastResult: summarizeText(strings.Join(notes, "；"), 220),
				NextAction: "继续推进任务执行并回传进展",
			})
		}
		return fmt.Sprintf("runtime.task.control 完成：agent=%s service=%s operation=ensure_running；%s", fallbackValue(strings.TrimSpace(targetAgentID), "main"), serviceName, strings.Join(notes, "；")), nil
	case "restart", "start":
		healthURL := sanitizeManagedHealthURL(action.HealthURL)
		serviceNote, err := applyActionPlanRuntimeServiceWithPortRecovery(ctx, actionPlanItem{
			Kind:          "runtime.service",
			Service:       serviceName,
			Operation:     operation,
			Scope:         scope,
			HealthURL:     healthURL,
			TimeoutSec:    action.TimeoutSec,
			ApprovalToken: strings.TrimSpace(action.ApprovalToken),
		})
		if err != nil {
			return "", err
		}
		notes := []string{serviceNote}
		taskStatusNote, err := applyActionPlanRuntimeTaskStatus(targetRuntime, decision, actionPlanItem{
			Kind:           "runtime.task.status",
			AgentID:        targetAgentID,
			ConversationID: strings.TrimSpace(action.ConversationID),
			Scope:          scope,
		}, fallbackCWD, runtimes, defaultAgentID)
		if err == nil && strings.TrimSpace(taskStatusNote) != "" {
			notes = append(notes, taskStatusNote)
		}

		conversationID := strings.TrimSpace(action.ConversationID)
		if conversationID == "" {
			conversationID = strings.TrimSpace(decision.ConversationID)
		}
		persistTaskControlActionHint(targetRuntime, fallbackCWD, conversationID, scopeKey, serviceName, operation, healthURL, "", "")
		if conversationID != "" {
			setExecutionGoalState(conversationID, executionGoalState{
				AgentID:    strings.TrimSpace(targetAgentID),
				Status:     "running",
				LastResult: summarizeText(strings.Join(notes, "；"), 220),
				NextAction: "继续推进任务执行并回传进展",
			})
		}
		return fmt.Sprintf("runtime.task.control 完成：agent=%s service=%s operation=%s；%s", fallbackValue(strings.TrimSpace(targetAgentID), "main"), serviceName, operation, strings.Join(notes, "；")), nil
	case "stop":
		serviceNote, err := applyActionPlanRuntimeService(ctx, actionPlanItem{
			Kind:          "runtime.service",
			Service:       serviceName,
			Operation:     "stop",
			Scope:         scope,
			TimeoutSec:    action.TimeoutSec,
			ApprovalToken: strings.TrimSpace(action.ApprovalToken),
		})
		if err != nil {
			return "", err
		}
		conversationID := strings.TrimSpace(action.ConversationID)
		if conversationID == "" {
			conversationID = strings.TrimSpace(decision.ConversationID)
		}
		persistTaskControlActionHint(targetRuntime, fallbackCWD, conversationID, scopeKey, serviceName, "stop", "", "", "")
		if conversationID != "" {
			setExecutionGoalState(conversationID, executionGoalState{
				AgentID:    strings.TrimSpace(targetAgentID),
				Status:     "blocked",
				LastResult: summarizeText(serviceNote, 220),
				NextAction: "确认是否需要重新拉起 worker",
			})
		}
		return fmt.Sprintf("runtime.task.control 完成：agent=%s service=%s operation=stop；%s", fallbackValue(strings.TrimSpace(targetAgentID), "main"), serviceName, serviceNote), nil
	default:
		return "", fmt.Errorf("runtime.task.control operation 不支持: %q", action.Operation)
	}
}

func persistTaskControlActionHint(runtime agentRuntime, fallbackCWD string, conversationID string, routeScopeKey string, service string, operation string, healthURL string, releaseID string, releaseVersion string) {
	service = strings.TrimSpace(strings.ToLower(strings.TrimSuffix(service, ".service")))
	if service == "" {
		return
	}
	operation = strings.TrimSpace(strings.ToLower(operation))
	if operation == "" {
		operation = "ensure_running"
	}
	conversationID = strings.TrimSpace(conversationID)
	routeScopeKey = normalizeTaskControlRouteScopeKey(routeScopeKey)
	if routeScopeKey == "" {
		routeScopeKey = inferRoutingScopeKeyFromScopedConversationID(conversationID)
	}
	root := resolveTaskTrackingRoot(runtime, fallbackCWD)
	if strings.TrimSpace(root) == "" {
		return
	}
	now := time.Now().UTC()
	if err := ensureTaskTrackingScaffold(root, now); err != nil {
		return
	}

	serviceURLs := map[string]string{}
	serviceReleaseIDs := map[string]string{}
	serviceReleaseVersions := map[string]string{}
	routeHints := map[string]taskControlRouteHintRecord{}
	if state, ok, err := loadWorkspaceTaskTrackingState(root); err == nil && ok {
		for key, value := range readTaskTrackingStringMap(state.TaskTrackingSnapshot, "service_health_urls") {
			serviceURLs[key] = value
		}
		for key, value := range readTaskTrackingStringMap(state.TaskTrackingSnapshot, "service_release_ids") {
			serviceReleaseIDs[key] = value
		}
		for key, value := range readTaskTrackingStringMap(state.TaskTrackingSnapshot, "service_release_versions") {
			serviceReleaseVersions[key] = value
		}
		for key, value := range readTaskTrackingRouteHintMap(state.TaskTrackingSnapshot, "task_control_route_hints") {
			routeHints[key] = value
		}
	}
	healthURL = sanitizeManagedHealthURL(healthURL)
	releaseID = strings.TrimSpace(releaseID)
	releaseVersion = strings.TrimSpace(releaseVersion)
	hasHealthURL := healthURL != ""
	hasReleaseMetadata := releaseID != "" || releaseVersion != ""
	if healthURL != "" {
		serviceURLs[service] = healthURL
		if routeScopeKey != "" {
			routeHints[routeScopeKey] = taskControlRouteHintRecord{
				Service:        service,
				HealthURL:      healthURL,
				Operation:      operation,
				UpdatedAt:      now.Format(time.RFC3339),
				ConversationID: conversationID,
			}
		}
	}
	if releaseID != "" {
		serviceReleaseIDs[service] = releaseID
	}
	if releaseVersion != "" {
		serviceReleaseVersions[service] = releaseVersion
	}
	pruneBefore := len(routeHints)
	routeHints, pruneStats := pruneTaskControlRouteHintMapWithStats(routeHints, now)

	fields := map[string]interface{}{
		"last_task_control_service":   service,
		"last_task_control_operation": operation,
		"last_task_control_at":        now.Format(time.RFC3339),
		"service_health_urls":         serviceURLs,
		"agent_id":                    strings.TrimSpace(runtime.agentID),
	}
	if len(serviceReleaseIDs) > 0 {
		fields["service_release_ids"] = serviceReleaseIDs
	}
	if len(serviceReleaseVersions) > 0 {
		fields["service_release_versions"] = serviceReleaseVersions
	}
	if len(routeHints) > 0 {
		fields["task_control_route_hints"] = encodeTaskTrackingRouteHintMap(routeHints)
	}
	if healthURL != "" {
		fields["health_service"] = service
		fields["health_url"] = healthURL
		fields["last_task_control_health_url"] = healthURL
	}
	if conversationID != "" {
		fields["conversation_id"] = conversationID
	}
	if routeScopeKey != "" {
		fields["last_task_control_scope"] = routeScopeKey
	}
	if releaseID != "" {
		fields["last_task_control_release_id"] = releaseID
	}
	if releaseVersion != "" {
		fields["last_task_control_release_version"] = releaseVersion
	}
	_ = updateWorkspaceTaskState(root, now, fields)
	if hasHealthURL || hasReleaseMetadata || pruneStats.DroppedExpired > 0 || pruneStats.DroppedTrimmed > 0 || pruneStats.DroppedInvalid > 0 {
		log.Printf(
			"task_control_hint_persist: agent=%s service=%s scope=%s conversation_id=%s health_url_set=%t release_metadata_set=%t route_hints_before=%d route_hints_after=%d dropped_invalid=%d dropped_expired=%d dropped_trimmed=%d ttl=%s max_entries=%d",
			strings.TrimSpace(runtime.agentID),
			service,
			routeScopeKey,
			conversationID,
			hasHealthURL,
			hasReleaseMetadata,
			pruneBefore,
			len(routeHints),
			pruneStats.DroppedInvalid,
			pruneStats.DroppedExpired,
			pruneStats.DroppedTrimmed,
			pruneStats.TTL.String(),
			pruneStats.MaxEntries,
		)
	}
	recordTaskControlPersistMetric(strings.TrimSpace(runtime.agentID), service, routeScopeKey, hasHealthURL, pruneBefore, len(routeHints), pruneStats)
}

func lookupTaskControlSelfHealBackoff(runtime agentRuntime, fallbackCWD string, service string) (int, time.Time) {
	root := resolveTaskTrackingRoot(runtime, fallbackCWD)
	if strings.TrimSpace(root) == "" {
		return 0, time.Time{}
	}
	state, ok, err := loadWorkspaceTaskTrackingState(root)
	if err != nil || !ok {
		return 0, time.Time{}
	}
	snapshot := state.TaskTrackingSnapshot
	if len(snapshot) == 0 {
		return 0, time.Time{}
	}
	service = strings.TrimSpace(strings.ToLower(strings.TrimSuffix(service, ".service")))
	if service == "" {
		return 0, time.Time{}
	}

	failures := 0
	if value := strings.TrimSpace(readTaskTrackingStringMap(snapshot, "service_self_heal_failures")[service]); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			failures = parsed
		}
	}
	cooldownUntil := time.Time{}
	if value := strings.TrimSpace(readTaskTrackingStringMap(snapshot, "service_self_heal_cooldown_until")[service]); value != "" {
		if parsed, ok := parseTaskControlHintTime(value); ok {
			cooldownUntil = parsed
		}
	}
	return failures, cooldownUntil
}

func persistTaskControlSelfHealBackoff(
	runtime agentRuntime,
	fallbackCWD string,
	conversationID string,
	service string,
	state string,
	reason string,
	errorSummary string,
	failures int,
	cooldownUntil time.Time,
) {
	service = strings.TrimSpace(strings.ToLower(strings.TrimSuffix(service, ".service")))
	if service == "" {
		return
	}
	root := resolveTaskTrackingRoot(runtime, fallbackCWD)
	if strings.TrimSpace(root) == "" {
		return
	}
	now := time.Now().UTC()
	if err := ensureTaskTrackingScaffold(root, now); err != nil {
		return
	}

	serviceFailures := map[string]string{}
	serviceCooldowns := map[string]string{}
	if workspace, ok, err := loadWorkspaceTaskTrackingState(root); err == nil && ok {
		for key, value := range readTaskTrackingStringMap(workspace.TaskTrackingSnapshot, "service_self_heal_failures") {
			serviceFailures[key] = value
		}
		for key, value := range readTaskTrackingStringMap(workspace.TaskTrackingSnapshot, "service_self_heal_cooldown_until") {
			serviceCooldowns[key] = value
		}
	}
	if failures > 0 {
		serviceFailures[service] = strconv.Itoa(failures)
	} else {
		delete(serviceFailures, service)
	}
	if !cooldownUntil.IsZero() {
		serviceCooldowns[service] = cooldownUntil.UTC().Format(time.RFC3339)
	} else {
		delete(serviceCooldowns, service)
	}

	fields := map[string]interface{}{
		"agent_id":                            strings.TrimSpace(runtime.agentID),
		"last_task_control_self_heal_at":      now.Format(time.RFC3339),
		"last_task_control_self_heal_service": service,
		"last_task_control_self_heal_state":   strings.TrimSpace(strings.ToLower(state)),
		"service_self_heal_failures":          serviceFailures,
		"service_self_heal_cooldown_until":    serviceCooldowns,
	}
	if convo := strings.TrimSpace(conversationID); convo != "" {
		fields["conversation_id"] = convo
	}
	if normalizedReason := strings.TrimSpace(strings.ToLower(reason)); normalizedReason != "" {
		fields["last_task_control_self_heal_reason"] = normalizedReason
	}
	if summary := strings.TrimSpace(errorSummary); summary != "" {
		fields["last_task_control_self_heal_error"] = summarizeActionPlanCommandOutput(summary)
	}
	if !cooldownUntil.IsZero() {
		fields["last_task_control_self_heal_cooldown_until"] = cooldownUntil.UTC().Format(time.RFC3339)
	}
	fields["last_task_control_self_heal_failures"] = failures

	_ = updateWorkspaceTaskState(root, now, fields)
	recordTaskControlSelfHealMetric(
		strings.TrimSpace(runtime.agentID),
		service,
		state,
		reason,
		failures,
		cooldownUntil,
		strings.TrimSpace(conversationID),
	)
}

func persistManagedReleaseTrackingHint(runtime agentRuntime, fallbackCWD string, conversationID string, service string, releaseID string, releaseVersion string, healthURL string) {
	service = strings.TrimSpace(strings.ToLower(strings.TrimSuffix(service, ".service")))
	releaseID = strings.TrimSpace(releaseID)
	releaseVersion = strings.TrimSpace(releaseVersion)
	if service == "" || (releaseID == "" && releaseVersion == "") {
		return
	}
	root := resolveTaskTrackingRoot(runtime, fallbackCWD)
	if strings.TrimSpace(root) == "" {
		return
	}
	now := time.Now().UTC()
	if err := ensureTaskTrackingScaffold(root, now); err != nil {
		return
	}

	serviceReleaseIDs := map[string]string{}
	serviceReleaseVersions := map[string]string{}
	serviceHealthURLs := map[string]string{}
	if state, ok, err := loadWorkspaceTaskTrackingState(root); err == nil && ok {
		for key, value := range readTaskTrackingStringMap(state.TaskTrackingSnapshot, "service_release_ids") {
			serviceReleaseIDs[key] = value
		}
		for key, value := range readTaskTrackingStringMap(state.TaskTrackingSnapshot, "service_release_versions") {
			serviceReleaseVersions[key] = value
		}
		for key, value := range readTaskTrackingStringMap(state.TaskTrackingSnapshot, "service_health_urls") {
			serviceHealthURLs[key] = value
		}
	}
	if releaseID != "" {
		serviceReleaseIDs[service] = releaseID
	}
	if releaseVersion != "" {
		serviceReleaseVersions[service] = releaseVersion
	}
	healthURL = sanitizeManagedHealthURL(healthURL)
	if healthURL != "" {
		serviceHealthURLs[service] = healthURL
	}

	fields := map[string]interface{}{
		"agent_id":                 strings.TrimSpace(runtime.agentID),
		"last_release_service":     service,
		"last_release_id":          releaseID,
		"last_release_version":     releaseVersion,
		"last_release_at":          now.Format(time.RFC3339),
		"service_release_ids":      serviceReleaseIDs,
		"service_release_versions": serviceReleaseVersions,
	}
	if len(serviceHealthURLs) > 0 {
		fields["service_health_urls"] = serviceHealthURLs
	}
	if healthURL != "" {
		fields["health_service"] = service
		fields["health_url"] = healthURL
	}
	if convo := strings.TrimSpace(conversationID); convo != "" {
		fields["conversation_id"] = convo
	}
	_ = updateWorkspaceTaskState(root, now, fields)
}

func normalizeManagedServiceName(raw string) (string, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	value = strings.TrimSuffix(value, ".service")
	if value == "" {
		return "", fmt.Errorf("runtime.service 缺少 service")
	}
	if !managedServiceNamePattern.MatchString(value) {
		return "", fmt.Errorf("runtime.service service 名称非法: %q", raw)
	}
	return value, nil
}

func normalizeManagedServiceOperation(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", "restart":
		return "restart", nil
	case "start":
		return "start", nil
	case "stop":
		return "stop", nil
	case "status":
		return "status", nil
	default:
		return "", fmt.Errorf("runtime.service operation 不支持: %q（仅支持 start|restart|stop|status）", raw)
	}
}

func normalizeManagedTaskControlOperation(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", "ensure_running", "ensure-running", "ensure":
		return "ensure_running", nil
	case "restart":
		return "restart", nil
	case "start":
		return "start", nil
	case "stop":
		return "stop", nil
	case "status":
		return "status", nil
	default:
		return "", fmt.Errorf("runtime.task.control operation 不支持: %q（仅支持 ensure_running|restart|start|stop|status）", raw)
	}
}

func inferManagedServiceNameForAgent(agentID string) string {
	agentID = strings.TrimSpace(strings.ToLower(agentID))
	if agentID == "" || agentID == "main" {
		return "clawx"
	}
	return "clawx-" + agentID
}

func inferAgentIDFromManagedServiceName(service string) string {
	service = strings.TrimSpace(strings.ToLower(service))
	if service == "" {
		return ""
	}
	if service == "clawx" {
		return "main"
	}
	if strings.HasPrefix(service, "clawx-") {
		agentID := strings.TrimSpace(strings.TrimPrefix(service, "clawx-"))
		if agentID != "" {
			return agentID
		}
	}
	return ""
}

func normalizeManagedSupervisorOperation(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", "ensure":
		return "ensure", nil
	case "status":
		return "status", nil
	case "disable":
		return "disable", nil
	default:
		return "", fmt.Errorf("runtime.supervisor operation 不支持: %q（仅支持 ensure|status|disable）", raw)
	}
}

func normalizeManagedServiceTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultManagedServiceTimeout
	}
	timeout := time.Duration(seconds) * time.Second
	if timeout > maxManagedServiceTimeout {
		return maxManagedServiceTimeout
	}
	return timeout
}

func isManagedServiceAllowed(service string) bool {
	allowRaw := strings.TrimSpace(os.Getenv("CLAWX_RUNTIME_SERVICE_ALLOWLIST"))
	if allowRaw == "" {
		allowRaw = strings.TrimSpace(os.Getenv("CLAWX_MANAGED_SERVICE_ALLOWLIST"))
	}
	if allowRaw == "" {
		return service == "clawx" || strings.HasPrefix(service, "clawx-")
	}
	for _, token := range strings.Split(allowRaw, ",") {
		rule := strings.TrimSpace(strings.ToLower(token))
		if rule == "" {
			continue
		}
		if rule == "*" {
			return true
		}
		if strings.HasSuffix(rule, "*") {
			prefix := strings.TrimSuffix(rule, "*")
			if strings.HasPrefix(service, prefix) {
				return true
			}
			continue
		}
		if service == rule {
			return true
		}
	}
	return false
}

func runManagedSystemctlUser(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return "", fmt.Errorf("systemctl 不可用: %w", err)
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "systemctl", append([]string{"--user"}, args...)...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("systemctl --user %s 超时（%s）", strings.Join(args, " "), timeout)
		}
		if trimmed == "" {
			trimmed = err.Error()
		}
		return "", fmt.Errorf("systemctl --user %s 失败: %s", strings.Join(args, " "), trimmed)
	}
	return trimmed, nil
}

func queryManagedSystemctlUserState(ctx context.Context, timeout time.Duration, args ...string) string {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return "unavailable"
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "systemctl", append([]string{"--user"}, args...)...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(strings.ToLower(string(output)))
	if err == nil {
		if trimmed == "" {
			return "unknown"
		}
		return trimmed
	}
	if runCtx.Err() == context.DeadlineExceeded {
		return "timeout"
	}
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}

func resolveManagedSupervisorUnitPath(runtime agentRuntime, actionCWD string, fallbackCWD string, rawPath string, service string) (string, error) {
	baseCWD := resolveManagedActionBaseCWD(actionCWD, fallbackCWD, runtime.cwd)
	candidates := make([]string, 0, 3)
	if strings.TrimSpace(rawPath) != "" {
		candidates = append(candidates, strings.TrimSpace(rawPath))
	} else {
		candidates = append(candidates,
			filepath.Join("deploy", "systemd", service+".service"),
			filepath.Join("deploy", "systemd", "clawx.service"),
		)
	}

	for _, candidate := range candidates {
		resolved := candidate
		if !filepath.IsAbs(resolved) {
			if baseCWD == "" {
				continue
			}
			resolved = filepath.Join(baseCWD, resolved)
		}
		resolved = filepath.Clean(resolved)
		if err := runtime.cfgSnapshot.ValidateWorkingDirectory(filepath.Dir(resolved)); err != nil {
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		return resolved, nil
	}

	if strings.TrimSpace(rawPath) != "" {
		return "", fmt.Errorf("runtime.supervisor unit_file 不可用: %s", strings.TrimSpace(rawPath))
	}
	return "", fmt.Errorf("runtime.supervisor 未找到 unit_file（默认查找 deploy/systemd/%s.service 或 deploy/systemd/clawx.service）", service)
}

func installManagedSystemdUserUnit(service string, sourcePath string) (bool, string, error) {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return false, "", fmt.Errorf("读取 unit_file 失败: %w", err)
	}
	if len(content) == 0 {
		return false, "", fmt.Errorf("unit_file 为空: %s", sourcePath)
	}
	unitDir, err := managedSystemdUserUnitDir()
	if err != nil {
		return false, "", err
	}
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return false, "", fmt.Errorf("创建 user systemd 目录失败: %w", err)
	}
	destPath := filepath.Join(unitDir, service+".service")
	changed := true
	if existing, readErr := os.ReadFile(destPath); readErr == nil && string(existing) == string(content) {
		changed = false
	}
	if changed {
		tmpPath := fmt.Sprintf("%s.tmp-%d", destPath, os.Getpid())
		if err := os.WriteFile(tmpPath, content, 0o644); err != nil {
			return false, "", fmt.Errorf("写入临时 unit 文件失败: %w", err)
		}
		if err := os.Rename(tmpPath, destPath); err != nil {
			_ = os.Remove(tmpPath)
			return false, "", fmt.Errorf("安装 unit 文件失败: %w", err)
		}
	}
	return changed, destPath, nil
}

func managedSystemdUserUnitDir() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "systemd", "user"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("解析 HOME 失败: %w", err)
	}
	home = strings.TrimSpace(home)
	if home == "" {
		return "", fmt.Errorf("HOME 为空，无法安装 user systemd unit")
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

func resolveManagedActionBaseCWD(actionCWD string, fallbackCWD string, runtimeCWD string) string {
	base := strings.TrimSpace(actionCWD)
	if base == "" {
		base = strings.TrimSpace(fallbackCWD)
	}
	if base == "" {
		base = strings.TrimSpace(runtimeCWD)
	}
	return base
}

func waitManagedServiceHealthy(ctx context.Context, healthURL string, timeout time.Duration) error {
	client := &http.Client{Timeout: 4 * time.Second}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastErr := ""
	for {
		req, err := http.NewRequestWithContext(waitCtx, http.MethodGet, healthURL, nil)
		if err != nil {
			return fmt.Errorf("health url 无效: %w", err)
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return nil
			}
			lastErr = "status=" + strconv.Itoa(resp.StatusCode)
		} else {
			lastErr = err.Error()
		}
		select {
		case <-waitCtx.Done():
			if strings.TrimSpace(lastErr) == "" {
				lastErr = waitCtx.Err().Error()
			}
			return fmt.Errorf("health 检查失败（%s）: %s", healthURL, lastErr)
		case <-ticker.C:
		}
	}
}

func summarizeActionPlanCommandOutput(raw string) string {
	text := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if text == "" {
		return "(no output)"
	}
	if len(text) <= 120 {
		return text
	}
	return text[:120] + "..."
}

func applyActionPlanRuntimeRelease(ctx context.Context, runtime agentRuntime, action actionPlanItem, fallbackCWD string) (string, error) {
	service, err := normalizeManagedServiceName(action.Service)
	if err != nil {
		return "", err
	}
	if !isManagedServiceAllowed(service) {
		return "", fmt.Errorf("service %q 不在允许名单中（设置 CLAWX_RUNTIME_SERVICE_ALLOWLIST 以放行）", service)
	}
	conversationID := strings.TrimSpace(action.ConversationID)
	targetAgentID := inferAgentIDFromManagedServiceName(service)
	execSourceLine := buildRuntimeExecDecisionContextLineForConversation(conversationID, targetAgentID, 12)
	appendExecSource := func(message string) string {
		return appendActionPlanExecutionSourceLine(message, execSourceLine)
	}

	scope := strings.ToLower(strings.TrimSpace(action.Scope))
	if scope == "" {
		scope = "user"
	}
	if scope != "user" {
		return "", fmt.Errorf("仅支持 scope=user 的受控发布动作")
	}

	if err := enforceManagedActionApproval(action, "runtime.release", service); err != nil {
		return "", err
	}
	timeout := normalizeManagedServiceTimeout(action.TimeoutSec)
	lock, err := acquireManagedServiceLock(ctx, service, timeout)
	if err != nil {
		return "", err
	}
	defer lock.Release()

	artifactPath := ""
	if strings.TrimSpace(action.ArtifactPath) != "" {
		artifactPath, err = resolveManagedArtifactPath(runtime, strings.TrimSpace(action.CWD), fallbackCWD, action.ArtifactPath)
		if err != nil {
			return "", err
		}
	}
	if managedReleaseRequiresArtifact() && strings.TrimSpace(artifactPath) == "" {
		return "", fmt.Errorf("runtime.release 要求 artifact_path（已开启 CLAWX_RUNTIME_RELEASE_REQUIRE_ARTIFACT）")
	}
	releaseVersion := normalizeManagedReleaseVersion(strings.TrimSpace(action.ReleaseVersion), artifactPath)
	record := managedReleaseRecord{
		ReleaseID:      buildManagedReleaseID(service),
		Service:        service,
		Version:        releaseVersion,
		Status:         "running",
		StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		HealthURL:      strings.TrimSpace(action.HealthURL),
		ArtifactPath:   strings.TrimSpace(artifactPath),
		RollbackScript: "",
	}
	if strings.TrimSpace(artifactPath) != "" {
		sha, size, hashErr := computeManagedFileSHA256(artifactPath)
		if hashErr != nil {
			return "", fmt.Errorf("runtime.release artifact_path 校验失败: %w", hashErr)
		}
		record.ArtifactSHA256 = sha
		record.ArtifactSizeBytes = size
	}

	scriptPath, err := resolveManagedScriptPath(runtime, strings.TrimSpace(action.CWD), fallbackCWD, action.Script, "script")
	if err != nil {
		return "", err
	}
	record.Script = scriptPath

	rollbackPath := ""
	if strings.TrimSpace(action.RollbackScript) != "" {
		rollbackPath, err = resolveManagedScriptPath(runtime, strings.TrimSpace(action.CWD), fallbackCWD, action.RollbackScript, "rollback_script")
		if err != nil {
			return "", err
		}
	}
	record.RollbackScript = rollbackPath

	scriptEnv := map[string]string{}
	if strings.TrimSpace(artifactPath) != "" {
		scriptEnv["CLAWX_RELEASE_ARTIFACT"] = strings.TrimSpace(artifactPath)
	}
	if strings.TrimSpace(releaseVersion) != "" {
		scriptEnv["CLAWX_RELEASE_VERSION"] = strings.TrimSpace(releaseVersion)
	}
	persistReleaseFailure := func(releaseErr error, deployOutput string, rollbackNote string, rolledBack bool) error {
		record.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		record.Error = strings.TrimSpace(releaseErr.Error())
		record.DeployOutput = summarizeActionPlanCommandOutput(deployOutput)
		if rolledBack {
			record.Status = "rolled_back"
			record.RollbackNote = strings.TrimSpace(rollbackNote)
		} else {
			record.Status = "failed"
		}
		if metaErr := persistManagedReleaseRecord(record, false); metaErr != nil {
			return fmt.Errorf("%v；发布元数据写入失败: %v", releaseErr, metaErr)
		}
		return releaseErr
	}
	deployOutput, err := runManagedScript(ctx, timeout, scriptPath, scriptEnv)
	if err != nil {
		releaseErr, rollbackNote, rolledBack := finalizeManagedReleaseFailureWithRollback(ctx, timeout, rollbackPath, service, action.HealthURL, fmt.Errorf("发布脚本失败: %w", err))
		releaseErr = fmt.Errorf("%v（release_id=%s version=%s）", releaseErr, record.ReleaseID, record.Version)
		return "", persistReleaseFailure(releaseErr, deployOutput, rollbackNote, rolledBack)
	}
	fillManagedReleaseRecordFromDeployOutput(&record, deployOutput)
	if strings.TrimSpace(record.ArtifactPath) == "" && strings.TrimSpace(record.SourceBinary) != "" {
		sha, size, hashErr := computeManagedFileSHA256(strings.TrimSpace(record.SourceBinary))
		if hashErr == nil {
			record.ArtifactPath = strings.TrimSpace(record.SourceBinary)
			record.ArtifactSHA256 = sha
			record.ArtifactSizeBytes = size
		}
	}
	if _, err := runManagedSystemctlUser(ctx, timeout, "restart", service+".service"); err != nil {
		releaseErr, rollbackNote, rolledBack := finalizeManagedReleaseFailureWithRollback(ctx, timeout, rollbackPath, service, action.HealthURL, fmt.Errorf("发布后重启服务失败: %w", err))
		releaseErr = fmt.Errorf("%v（release_id=%s version=%s）", releaseErr, record.ReleaseID, record.Version)
		return "", persistReleaseFailure(releaseErr, deployOutput, rollbackNote, rolledBack)
	}
	if strings.TrimSpace(action.HealthURL) != "" {
		if err := waitManagedServiceHealthy(ctx, strings.TrimSpace(action.HealthURL), timeout); err != nil {
			releaseErr, rollbackNote, rolledBack := finalizeManagedReleaseFailureWithRollback(ctx, timeout, rollbackPath, service, action.HealthURL, fmt.Errorf("发布后健康检查失败: %w", err))
			releaseErr = fmt.Errorf("%v（release_id=%s version=%s）", releaseErr, record.ReleaseID, record.Version)
			return "", persistReleaseFailure(releaseErr, deployOutput, rollbackNote, rolledBack)
		}
	}

	record.Status = "succeeded"
	record.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	record.Error = ""
	record.RollbackNote = ""
	record.DeployOutput = summarizeActionPlanCommandOutput(deployOutput)
	metadataWarn := ""
	if metaErr := persistManagedReleaseRecord(record, true); metaErr != nil {
		metadataWarn = summarizeActionPlanCommandOutput(metaErr.Error())
	}
	persistManagedReleaseTrackingHint(runtime, fallbackCWD, conversationID, service, record.ReleaseID, record.Version, strings.TrimSpace(action.HealthURL))
	base := fmt.Sprintf("runtime.release 完成：service=%s version=%s release_id=%s script=%s", service, record.Version, record.ReleaseID, scriptPath)
	if strings.TrimSpace(action.HealthURL) == "" {
		if strings.TrimSpace(deployOutput) == "" {
			if metadataWarn == "" {
				return appendExecSource(base), nil
			}
			return appendExecSource(base + " metadata_warning=" + metadataWarn), nil
		}
		msg := base + " output=" + summarizeActionPlanCommandOutput(deployOutput)
		if metadataWarn != "" {
			msg += " metadata_warning=" + metadataWarn
		}
		return appendExecSource(msg), nil
	}
	if strings.TrimSpace(deployOutput) == "" {
		msg := base + " health=" + strings.TrimSpace(action.HealthURL)
		if metadataWarn != "" {
			msg += " metadata_warning=" + metadataWarn
		}
		return appendExecSource(msg), nil
	}
	msg := base + " output=" + summarizeActionPlanCommandOutput(deployOutput) + " health=" + strings.TrimSpace(action.HealthURL)
	if metadataWarn != "" {
		msg += " metadata_warning=" + metadataWarn
	}
	return appendExecSource(msg), nil
}

func applyActionPlanRuntimeReleaseStatus(action actionPlanItem, fallbackConversationID string) (string, error) {
	service, err := normalizeManagedServiceName(action.Service)
	if err != nil {
		return "", err
	}
	if !isManagedServiceAllowed(service) {
		return "", fmt.Errorf("service %q 不在允许名单中（设置 CLAWX_RUNTIME_SERVICE_ALLOWLIST 以放行）", service)
	}
	scope := strings.ToLower(strings.TrimSpace(action.Scope))
	if scope == "" {
		scope = "user"
	}
	if scope != "user" {
		return "", fmt.Errorf("仅支持 scope=user 的发布状态查询")
	}
	conversationID := strings.TrimSpace(action.ConversationID)
	if conversationID == "" {
		conversationID = strings.TrimSpace(fallbackConversationID)
	}
	targetAgentID := inferAgentIDFromManagedServiceName(service)
	execSourceLine := buildRuntimeExecDecisionContextLineForConversation(conversationID, targetAgentID, 12)
	operation, err := normalizeManagedReleaseStatusOperation(action.Operation)
	if err != nil {
		return "", err
	}
	stateDir, err := managedReleaseStateDir()
	if err != nil {
		return "", err
	}
	serviceDir := filepath.Join(stateDir, service)
	switch operation {
	case "current":
		currentPath := filepath.Join(serviceDir, "current.json")
		raw, err := os.ReadFile(currentPath)
		if err != nil {
			if os.IsNotExist(err) {
				return appendActionPlanExecutionSourceLine(fmt.Sprintf("runtime.release.status：service=%s current=none", service), execSourceLine), nil
			}
			return "", fmt.Errorf("读取 current 元数据失败: %w", err)
		}
		var record managedReleaseRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return "", fmt.Errorf("解析 current 元数据失败: %w", err)
		}
		return appendActionPlanExecutionSourceLine(formatManagedReleaseCurrentSummary(service, record), execSourceLine), nil
	case "history":
		historyPath := filepath.Join(serviceDir, "history.jsonl")
		raw, err := os.ReadFile(historyPath)
		if err != nil {
			if os.IsNotExist(err) {
				return appendActionPlanExecutionSourceLine(fmt.Sprintf("runtime.release.status：service=%s history=empty", service), execSourceLine), nil
			}
			return "", fmt.Errorf("读取发布历史失败: %w", err)
		}
		records, err := parseManagedReleaseHistory(raw)
		if err != nil {
			return "", err
		}
		if len(records) == 0 {
			return appendActionPlanExecutionSourceLine(fmt.Sprintf("runtime.release.status：service=%s history=empty", service), execSourceLine), nil
		}
		limit := normalizeManagedReleaseHistoryLimit(action.Limit)
		if len(records) > limit {
			records = records[len(records)-limit:]
		}
		lines := []string{fmt.Sprintf("runtime.release.status：service=%s history_count=%d", service, len(records))}
		for i := len(records) - 1; i >= 0; i-- {
			record := records[i]
			index := len(records) - i
			lines = append(lines, fmt.Sprintf("#%d version=%s status=%s release_id=%s finished_at=%s", index, fallbackManagedReleaseValue(record.Version, "-"), fallbackManagedReleaseValue(record.Status, "-"), fallbackManagedReleaseValue(record.ReleaseID, "-"), fallbackManagedReleaseValue(record.FinishedAt, "-")))
		}
		return appendActionPlanExecutionSourceLine(strings.Join(lines, "\n"), execSourceLine), nil
	default:
		return "", fmt.Errorf("runtime.release.status operation 不支持: %q", action.Operation)
	}
}

func appendActionPlanExecutionSourceLine(note string, sourceLine string) string {
	note = strings.TrimSpace(note)
	sourceLine = strings.TrimSpace(sourceLine)
	if note == "" {
		return sourceLine
	}
	if sourceLine == "" || strings.Contains(note, sourceLine) {
		return note
	}
	return strings.TrimSpace(note + "\n" + sourceLine)
}

func buildRuntimeActionExecutionSourceLine(
	action actionPlanItem,
	decision service.Decision,
	runtime agentRuntime,
	defaultAgentID string,
) string {
	conversationID := strings.TrimSpace(action.ConversationID)
	if conversationID == "" {
		conversationID = strings.TrimSpace(decision.ConversationID)
	}
	targetAgentID := cleanAgentIDToken(action.AgentID)
	if targetAgentID == "" {
		targetAgentID = inferAgentIDFromManagedServiceName(action.Service)
	}
	if targetAgentID == "" {
		targetAgentID = strings.TrimSpace(runtime.agentID)
	}
	if targetAgentID == "" {
		targetAgentID = strings.TrimSpace(defaultAgentID)
	}
	return buildRuntimeExecDecisionContextLineForConversation(conversationID, targetAgentID, 12)
}

func appendRuntimeActionFailureNote(
	notes []string,
	kind string,
	err error,
	action actionPlanItem,
	decision service.Decision,
	runtime agentRuntime,
	defaultAgentID string,
) []string {
	errNote := strings.TrimSpace(kind) + " 执行失败: " + strings.TrimSpace(err.Error())
	execSourceLine := buildRuntimeActionExecutionSourceLine(action, decision, runtime, defaultAgentID)
	return append(notes, "- "+appendActionPlanExecutionSourceLine(errNote, execSourceLine))
}

func appendRuntimeActionOutcomeNote(
	notes []string,
	note string,
	action actionPlanItem,
	decision service.Decision,
	runtime agentRuntime,
	defaultAgentID string,
) []string {
	note = strings.TrimSpace(note)
	if note == "" {
		return notes
	}
	execSourceLine := buildRuntimeActionExecutionSourceLine(action, decision, runtime, defaultAgentID)
	return append(notes, "- "+appendActionPlanExecutionSourceLine(note, execSourceLine))
}

func applyActionPlanRuntimeTaskStatus(
	runtime agentRuntime,
	decision service.Decision,
	action actionPlanItem,
	fallbackCWD string,
	runtimes map[string]agentRuntime,
	defaultAgentID string,
) (string, error) {
	scope := strings.ToLower(strings.TrimSpace(action.Scope))
	if scope == "" {
		scope = "user"
	}
	if scope != "user" {
		return "", fmt.Errorf("仅支持 scope=user 的任务状态查询")
	}

	targetAgentID := cleanAgentIDToken(action.AgentID)
	if targetAgentID == "" {
		targetAgentID = strings.TrimSpace(runtime.agentID)
	}
	if targetAgentID == "" {
		targetAgentID = strings.TrimSpace(defaultAgentID)
	}
	targetRuntime := runtime
	if selected := selectRuntime(runtimes, defaultAgentID, targetAgentID); strings.TrimSpace(selected.agentID) != "" {
		targetRuntime = selected
	}
	if strings.TrimSpace(targetRuntime.agentID) == "" {
		targetRuntime.agentID = targetAgentID
	}
	if strings.TrimSpace(targetAgentID) == "" {
		targetAgentID = strings.TrimSpace(targetRuntime.agentID)
	}

	conversationID := strings.TrimSpace(action.ConversationID)
	if conversationID == "" {
		conversationID = strings.TrimSpace(decision.ConversationID)
	}
	execSourceLine := buildRuntimeExecDecisionContextLineForConversation(conversationID, targetAgentID, 12)
	goalState, hasGoalState := getExecutionGoalState(conversationID)

	root := resolveTaskTrackingRoot(targetRuntime, fallbackCWD)
	trackingState, hasTrackingState, err := loadWorkspaceTaskTrackingState(root)
	if err != nil {
		return "", err
	}
	snapshot := trackingState.TaskTrackingSnapshot
	status := normalizeTaskStatusValue(readTaskTrackingString(snapshot, "status"))
	if status == "" && hasGoalState {
		status = normalizeTaskStatusValue(goalState.Status)
	}
	if status == "" {
		status = "unknown"
	}

	goal := strings.TrimSpace(readTaskTrackingString(snapshot, "goal"))
	if goal == "" && hasGoalState {
		goal = strings.TrimSpace(goalState.Goal)
	}
	taskID := strings.TrimSpace(readTaskTrackingString(snapshot, "task_id"))
	if taskID == "" && hasGoalState {
		taskID = strings.TrimSpace(goalState.TaskID)
	}
	lastResult := strings.TrimSpace(readTaskTrackingString(snapshot, "last_result"))
	if lastResult == "" && hasGoalState {
		lastResult = strings.TrimSpace(goalState.LastResult)
	}
	updatedAt := strings.TrimSpace(trackingState.LastTaskUpdatedAt)
	if updatedAt == "" && hasGoalState && !goalState.UpdatedAt.IsZero() {
		updatedAt = goalState.UpdatedAt.UTC().Format(time.RFC3339)
	}

	execTotal := readTaskTrackingInt(snapshot, "exec_total")
	execSuccess := readTaskTrackingInt(snapshot, "exec_success")
	execFailed := readTaskTrackingInt(snapshot, "exec_failed")
	remainingSteps := readTaskTrackingStringList(snapshot, "remaining_steps")
	if len(remainingSteps) == 0 && hasGoalState {
		remainingSteps = normalizeExecutionStringList(goalState.RemainingSteps)
	}
	nextAction := strings.TrimSpace(readTaskTrackingString(snapshot, "next_action"))
	if nextAction == "" && hasGoalState {
		nextAction = strings.TrimSpace(goalState.NextAction)
	}

	agentLabel := fallbackValue(strings.TrimSpace(targetAgentID), "main")
	if goal == "" && lastResult == "" && status == "unknown" && taskID == "" && len(remainingSteps) == 0 && nextAction == "" && !hasTrackingState && !hasGoalState {
		line := fmt.Sprintf("查询结果：%s 当前还没有可用任务进度。", agentLabel)
		if execSourceLine != "" {
			line += "\n" + execSourceLine
		}
		return line, nil
	}

	parts := []string{
		fmt.Sprintf("查询结果：%s 当前任务状态为 %s", agentLabel, status),
	}
	if taskID != "" {
		parts = append(parts, "task_id="+taskID)
	}
	if goal != "" {
		parts = append(parts, "目标："+summarizeText(goal, 120))
	}
	if len(remainingSteps) > 0 {
		parts = append(parts, "剩余步骤："+strings.Join(limitExecutionList(remainingSteps, 3), "；"))
	}
	if nextAction != "" {
		parts = append(parts, nextStepLine(summarizeText(nextAction, 120)))
	}
	if updatedAt != "" {
		parts = append(parts, "最近更新："+updatedAt)
	}
	if lastResult != "" {
		parts = append(parts, "最近结果："+summarizeText(lastResult, 160))
	}
	if execTotal > 0 || execSuccess > 0 || execFailed > 0 {
		parts = append(parts, fmt.Sprintf("执行统计：total=%d success=%d failed=%d", execTotal, execSuccess, execFailed))
	}
	if execSourceLine != "" {
		parts = append(parts, execSourceLine)
	}
	return strings.Join(parts, "；") + "。", nil
}

func applyActionPlanRuntimeTaskDelegate(
	runtime agentRuntime,
	decision service.Decision,
	action actionPlanItem,
	fallbackCWD string,
	runtimes map[string]agentRuntime,
	defaultAgentID string,
) (string, error) {
	scope := strings.ToLower(strings.TrimSpace(action.Scope))
	if scope == "" {
		scope = "user"
	}
	if scope != "user" {
		return "", fmt.Errorf("仅支持 scope=user 的任务委派动作")
	}

	targetAgentID := cleanAgentIDToken(action.AgentID)
	if targetAgentID == "" {
		targetAgentID = strings.TrimSpace(runtime.agentID)
	}
	if targetAgentID == "" {
		targetAgentID = strings.TrimSpace(defaultAgentID)
	}
	targetRuntime := runtime
	if selected := selectRuntime(runtimes, defaultAgentID, targetAgentID); strings.TrimSpace(selected.agentID) != "" {
		targetRuntime = selected
	}
	if strings.TrimSpace(targetRuntime.agentID) == "" {
		targetRuntime.agentID = targetAgentID
	}
	if strings.TrimSpace(targetAgentID) == "" {
		targetAgentID = strings.TrimSpace(targetRuntime.agentID)
	}

	root := resolveTaskTrackingRoot(targetRuntime, fallbackCWD)
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("未解析到可用 runtime workspace")
	}

	cmdline := strings.TrimSpace(action.Cmd)
	if cmdline == "" {
		cmdline = strings.TrimSpace(action.Command)
	}
	if cmdline == "" {
		return "", fmt.Errorf("runtime.task.delegate 缺少 cmd/command")
	}
	if err := validateRuntimeExecCommand(cmdline); err != nil {
		return "", fmt.Errorf("runtime.task.delegate 命令被拒绝: %w", err)
	}

	taskCWD := strings.TrimSpace(action.CWD)
	if taskCWD == "" {
		taskCWD = root
	}
	if err := targetRuntime.cfgSnapshot.ValidateWorkingDirectory(taskCWD); err != nil {
		return "", fmt.Errorf("runtime.task.delegate cwd 超出允许范围: %w", err)
	}

	conversationID := strings.TrimSpace(action.ConversationID)
	if conversationID == "" {
		conversationID = strings.TrimSpace(decision.ConversationID)
	}
	parentTaskID := strings.TrimSpace(action.ParentTaskID)
	if parentTaskID == "" && conversationID != "" {
		if goal, ok := getExecutionGoalState(conversationID); ok {
			parentTaskID = strings.TrimSpace(goal.TaskID)
		}
	}

	resourceKey := strings.TrimSpace(action.ResourceKey)
	if resourceKey == "" {
		resourceKey = strings.ToLower(strings.TrimSpace(taskCWD))
	}
	taskSummary := strings.TrimSpace(action.TaskSummary)
	if taskSummary == "" {
		taskSummary = strings.TrimSpace(action.Reason)
	}
	taskTitle := strings.TrimSpace(action.TaskTitle)
	if taskTitle == "" {
		taskTitle = taskSummary
	}
	if taskTitle == "" {
		taskTitle = summarizeCommand(cmdline)
	}
	delegatedAgent := cleanAgentIDToken(action.DelegatedAgent)
	if delegatedAgent == "" {
		delegatedAgent = targetAgentID
	}

	queueFile := filepath.Join(root, ".clawx", "runtime", "tasks.jsonl")
	queue := runtimeorchestrator.NewQueue(queueFile)
	payload := map[string]interface{}{
		"cmd":             cmdline,
		"cwd":             taskCWD,
		"reason":          strings.TrimSpace(action.Reason),
		"conversation":    conversationID,
		"resource_key":    resourceKey,
		"parent_task_id":  parentTaskID,
		"task_title":      summarizeText(taskTitle, 120),
		"task_summary":    summarizeText(taskSummary, 180),
		"delegated_agent": delegatedAgent,
		"delegated_by":    strings.TrimSpace(runtime.agentID),
		"delegated_at":    time.Now().UTC().Format(time.RFC3339),
	}
	task := runtimeorchestrator.RuntimeTask{
		Source:  "runtime.task.delegate",
		Intent:  "runtime.exec",
		Payload: payload,
		Status:  runtimeorchestrator.TaskQueued,
	}
	if action.MaxRetry > 0 {
		task.MaxRetry = action.MaxRetry
	}
	enqueued, err := queue.Enqueue(task)
	if err != nil {
		return "", err
	}

	dispatchedCount := 0
	dispatchWarning := ""
	dispatchFile := filepath.Join(root, ".clawx", "runtime", "dispatch_state.json")
	if leadMode, _, leadErr := loadLeadModeFromRuntimeMeta(root); leadErr == nil && leadMode {
		registry := runtimeorchestrator.NewRegistry(filepath.Join(root, ".clawx", "runtime", "worker_states.json"), filepath.Join(root, ".clawx", "runtime", "heartbeats.json"))
		dispatcher := runtimeorchestrator.NewDispatcher(queue, registry, dispatchFile)
		if decisions, dispatchErr := dispatcher.DispatchQueued(time.Now().UTC(), 1); dispatchErr == nil {
			dispatchedCount = len(decisions)
		} else {
			dispatchWarning = summarizeText(dispatchErr.Error(), 100)
		}
	}

	if conversationID != "" {
		last := fmt.Sprintf("已委派子任务 %s 给 %s", strings.TrimSpace(enqueued.TaskID), fallbackValue(strings.TrimSpace(targetAgentID), "main"))
		setExecutionGoalState(conversationID, executionGoalState{
			AgentID:    strings.TrimSpace(targetAgentID),
			Status:     "running",
			LastResult: summarizeText(last, 220),
			NextAction: "等待子任务执行并通过 runtime.task.delegates 汇总状态",
		})
	}

	parts := []string{
		fmt.Sprintf("runtime.task.delegate 完成：agent=%s child_task_id=%s status=%s", fallbackValue(strings.TrimSpace(targetAgentID), "main"), strings.TrimSpace(enqueued.TaskID), strings.TrimSpace(string(enqueued.Status))),
	}
	if parentTaskID != "" {
		parts = append(parts, "parent_task_id="+parentTaskID)
	}
	if strings.TrimSpace(taskTitle) != "" {
		parts = append(parts, "title="+summarizeText(taskTitle, 80))
	}
	parts = append(parts, "queue="+queueFile)
	if dispatchedCount > 0 {
		parts = append(parts, fmt.Sprintf("dispatched=%d", dispatchedCount))
	}
	if dispatchWarning != "" {
		parts = append(parts, "dispatch_warning="+dispatchWarning)
	}
	return strings.Join(parts, " "), nil
}

func applyActionPlanRuntimeTaskDelegates(
	runtime agentRuntime,
	decision service.Decision,
	action actionPlanItem,
	fallbackCWD string,
	runtimes map[string]agentRuntime,
	defaultAgentID string,
) (string, error) {
	scope := strings.ToLower(strings.TrimSpace(action.Scope))
	if scope == "" {
		scope = "user"
	}
	if scope != "user" {
		return "", fmt.Errorf("仅支持 scope=user 的任务委派查询")
	}

	targetAgentID := cleanAgentIDToken(action.AgentID)
	if targetAgentID == "" {
		targetAgentID = strings.TrimSpace(runtime.agentID)
	}
	if targetAgentID == "" {
		targetAgentID = strings.TrimSpace(defaultAgentID)
	}
	targetRuntime := runtime
	if selected := selectRuntime(runtimes, defaultAgentID, targetAgentID); strings.TrimSpace(selected.agentID) != "" {
		targetRuntime = selected
	}
	if strings.TrimSpace(targetRuntime.agentID) == "" {
		targetRuntime.agentID = targetAgentID
	}
	if strings.TrimSpace(targetAgentID) == "" {
		targetAgentID = strings.TrimSpace(targetRuntime.agentID)
	}

	root := resolveTaskTrackingRoot(targetRuntime, fallbackCWD)
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("未解析到可用 runtime workspace")
	}
	queueFile := filepath.Join(root, ".clawx", "runtime", "tasks.jsonl")
	queue := runtimeorchestrator.NewQueue(queueFile)
	tasks, err := queue.List()
	if err != nil {
		return "", err
	}

	conversationID := strings.TrimSpace(action.ConversationID)
	if conversationID == "" {
		conversationID = strings.TrimSpace(decision.ConversationID)
	}
	execSourceLine := buildRuntimeExecDecisionContextLineForConversation(conversationID, targetAgentID, 12)
	parentTaskID := strings.TrimSpace(action.ParentTaskID)
	if parentTaskID == "" && conversationID != "" {
		if goal, ok := getExecutionGoalState(conversationID); ok {
			parentTaskID = strings.TrimSpace(goal.TaskID)
		}
	}

	filtered := collectRuntimeDelegateTasks(tasks, conversationID, parentTaskID)
	if len(filtered) == 0 {
		line := fmt.Sprintf("查询结果：runtime.task.delegates 暂无子任务 agent=%s", fallbackValue(strings.TrimSpace(targetAgentID), "main"))
		if parentTaskID != "" {
			line += " parent_task_id=" + parentTaskID
		}
		if conversationID != "" {
			line += " conversation_id=" + conversationID
		}
		if execSourceLine != "" {
			line += "\n" + execSourceLine
		}
		return line, nil
	}

	progress := computeRuntimeDelegateProgress(filtered)

	limit := action.Limit
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	header := fmt.Sprintf(
		"查询结果：runtime.task.delegates agent=%s total=%d queued=%d running=%d succeeded=%d failed=%d canceled=%d",
		fallbackValue(strings.TrimSpace(targetAgentID), "main"),
		progress.Total,
		progress.Queued,
		progress.Running,
		progress.Succeeded,
		progress.Failed,
		progress.Canceled,
	)
	if parentTaskID != "" {
		header += " parent_task_id=" + parentTaskID
	}
	lines := []string{header}
	if execSourceLine != "" {
		lines = append(lines, execSourceLine)
	}
	for idx, task := range filtered {
		if idx >= limit {
			break
		}
		taskID := strings.TrimSpace(task.TaskID)
		status := strings.TrimSpace(strings.ToLower(string(task.Status)))
		if status == "" {
			status = "unknown"
		}
		worker := strings.TrimSpace(task.AssignedWorkerID)
		title := readRuntimeTaskPayloadString(task.Payload, "task_title")
		if title == "" {
			title = summarizeCommand(readRuntimeTaskPayloadString(task.Payload, "cmd"))
		}
		line := fmt.Sprintf("#%d task_id=%s status=%s", idx+1, fallbackValue(taskID, "-"), status)
		if worker != "" {
			line += " worker=" + worker
		}
		if parentTaskID == "" {
			if childParent := readRuntimeTaskPayloadString(task.Payload, "parent_task_id"); childParent != "" {
				line += " parent_task_id=" + childParent
			}
		}
		if title != "" {
			line += " title=" + summarizeText(title, 80)
		}
		if status == "failed" {
			if reason := extractRuntimeTaskFailureReason(task); reason != "" {
				line += " error=" + summarizeText(reason, 100)
			}
		}
		lines = append(lines, line)
	}
	if progress.Total > limit {
		lines = append(lines, fmt.Sprintf("... 其余 %d 项可通过提高 limit 查看。", progress.Total-limit))
	}
	if reasonParts := renderRuntimeDelegateFailedReasonTop(progress.FailedReasons, 3, 80); len(reasonParts) > 0 {
		if len(reasonParts) > 0 {
			lines = append(lines, "失败原因Top："+strings.Join(reasonParts, "；"))
		}
	}
	switch {
	case progress.Failed > 0:
		lines = append(lines, nextStepLine(nextStepDelegateRawRetry))
	case progress.Running > 0 || progress.Queued > 0:
		lines = append(lines, nextStepLine(nextStepDelegateRawWait))
	default:
		lines = append(lines, nextStepLine(nextStepDelegateRawDone))
	}
	return strings.Join(lines, "\n"), nil
}

func buildRuntimeExecDecisionContextLineForConversation(conversationID string, agentID string, limit int) string {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return ""
	}
	records := listRecentRuntimeExecAttestations(conversationID, limit)
	if len(records) == 0 {
		return ""
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return buildRuntimeExecDecisionContextLineFromAttestations(records)
	}
	filtered := make([]runtimeExecAttestationRecord, 0, len(records))
	for _, rec := range records {
		if strings.EqualFold(strings.TrimSpace(rec.AgentID), agentID) {
			filtered = append(filtered, rec)
		}
	}
	if len(filtered) == 0 {
		filtered = records
	}
	return buildRuntimeExecDecisionContextLineFromAttestations(filtered)
}

type runtimeDelegateProgress struct {
	Total         int
	Queued        int
	Running       int
	Succeeded     int
	Failed        int
	Canceled      int
	FailedReasons map[string]int
}

func collectRuntimeDelegateTasks(tasks []runtimeorchestrator.RuntimeTask, conversationID string, parentTaskID string) []runtimeorchestrator.RuntimeTask {
	conversationID = strings.TrimSpace(conversationID)
	parentTaskID = strings.TrimSpace(parentTaskID)
	filtered := make([]runtimeorchestrator.RuntimeTask, 0, len(tasks))
	for _, task := range tasks {
		source := strings.TrimSpace(strings.ToLower(task.Source))
		if source != "runtime.task.delegate" && readRuntimeTaskPayloadString(task.Payload, "parent_task_id") == "" {
			continue
		}
		taskConversation := readRuntimeTaskPayloadString(task.Payload, "conversation")
		if conversationID != "" && taskConversation != "" && taskConversation != conversationID {
			continue
		}
		childParentID := readRuntimeTaskPayloadString(task.Payload, "parent_task_id")
		if parentTaskID != "" && childParentID != parentTaskID {
			continue
		}
		filtered = append(filtered, task)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return runtimeTaskSortTimestamp(filtered[i]).After(runtimeTaskSortTimestamp(filtered[j]))
	})
	return filtered
}

func computeRuntimeDelegateProgress(tasks []runtimeorchestrator.RuntimeTask) runtimeDelegateProgress {
	progress := runtimeDelegateProgress{
		Total:         len(tasks),
		FailedReasons: map[string]int{},
	}
	for _, task := range tasks {
		switch strings.TrimSpace(strings.ToLower(string(task.Status))) {
		case "queued":
			progress.Queued++
		case "running":
			progress.Running++
		case "succeeded":
			progress.Succeeded++
		case "failed":
			progress.Failed++
			if reason := extractRuntimeTaskFailureReason(task); reason != "" {
				progress.FailedReasons[reason]++
			}
		case "canceled":
			progress.Canceled++
		default:
			progress.Queued++
		}
	}
	return progress
}

func renderRuntimeDelegateFailedReasonTop(reasons map[string]int, maxEntries int, summarizeLimit int) []string {
	if len(reasons) == 0 || maxEntries <= 0 {
		return nil
	}
	type failedReasonCount struct {
		Reason string
		Count  int
	}
	sortedReasons := make([]failedReasonCount, 0, len(reasons))
	for reason, count := range reasons {
		if strings.TrimSpace(reason) == "" || count <= 0 {
			continue
		}
		sortedReasons = append(sortedReasons, failedReasonCount{Reason: reason, Count: count})
	}
	sort.SliceStable(sortedReasons, func(i, j int) bool {
		if sortedReasons[i].Count == sortedReasons[j].Count {
			return sortedReasons[i].Reason < sortedReasons[j].Reason
		}
		return sortedReasons[i].Count > sortedReasons[j].Count
	})
	if len(sortedReasons) > maxEntries {
		sortedReasons = sortedReasons[:maxEntries]
	}
	parts := make([]string, 0, len(sortedReasons))
	for _, item := range sortedReasons {
		parts = append(parts, fmt.Sprintf("%s(x%d)", summarizeText(item.Reason, summarizeLimit), item.Count))
	}
	return parts
}

func updateExecutionGoalWithDelegateProgress(queue *runtimeorchestrator.Queue, conversationID string, parentTaskID string, agentID string) {
	if queue == nil {
		return
	}
	conversationID = strings.TrimSpace(conversationID)
	parentTaskID = strings.TrimSpace(parentTaskID)
	if conversationID == "" || parentTaskID == "" {
		return
	}
	tasks, err := queue.List()
	if err != nil {
		return
	}
	filtered := collectRuntimeDelegateTasks(tasks, conversationID, parentTaskID)
	if len(filtered) == 0 {
		return
	}
	progress := computeRuntimeDelegateProgress(filtered)
	goalStatus := "running"
	nextAction := "等待子任务完成后汇总结果"
	remaining := make([]string, 0, 2)
	switch {
	case progress.Failed > 0:
		goalStatus = "blocked"
		nextAction = "优先处理 failed 子任务后重试"
		remaining = append(remaining, fmt.Sprintf("处理 failed 子任务（%d）", progress.Failed))
	case progress.Running > 0 || progress.Queued > 0:
		goalStatus = "running"
		nextAction = "等待 running/queued 子任务完成后继续汇总"
		if progress.Running > 0 {
			remaining = append(remaining, fmt.Sprintf("等待 running 子任务完成（%d）", progress.Running))
		}
		if progress.Queued > 0 {
			remaining = append(remaining, fmt.Sprintf("等待 queued 子任务执行（%d）", progress.Queued))
		}
	case progress.Canceled > 0:
		goalStatus = "blocked"
		nextAction = "确认 canceled 子任务是否需要重试或重新委派"
		remaining = append(remaining, fmt.Sprintf("处理 canceled 子任务（%d）", progress.Canceled))
	default:
		goalStatus = "completed"
		nextAction = "子任务已完成，可汇总并回复用户"
	}
	summary := fmt.Sprintf(
		"子任务进度（parent_task_id=%s）：total=%d queued=%d running=%d succeeded=%d failed=%d canceled=%d",
		parentTaskID,
		progress.Total,
		progress.Queued,
		progress.Running,
		progress.Succeeded,
		progress.Failed,
		progress.Canceled,
	)
	if failedTop := renderRuntimeDelegateFailedReasonTop(progress.FailedReasons, 2, 60); len(failedTop) > 0 {
		summary = summary + "；失败原因Top：" + strings.Join(failedTop, "；")
	}
	setExecutionGoalState(conversationID, executionGoalState{
		AgentID:        strings.TrimSpace(agentID),
		Status:         goalStatus,
		LastResult:     summarizeText(summary, 220),
		RemainingSteps: remaining,
		NextAction:     nextAction,
	})
	notifyBody := strings.TrimSpace("子任务自治进展：" + summary + "\n" + nextStepLine(nextAction))
	emitConversationProgressNotification(conversationID, notifyBody)
}

func extractRuntimeTaskFailureReason(task runtimeorchestrator.RuntimeTask) string {
	candidates := []string{
		readRuntimeTaskPayloadString(task.Payload, "last_error"),
		readRuntimeTaskPayloadString(task.Payload, "error"),
		readRuntimeTaskPayloadString(task.Payload, "reason"),
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		return summarizeText(candidate, 120)
	}
	return ""
}

func applyActionPlanRuntimeTaskRetry(
	runtime agentRuntime,
	decision service.Decision,
	action actionPlanItem,
	fallbackCWD string,
	runtimes map[string]agentRuntime,
	defaultAgentID string,
) (string, error) {
	scope := strings.ToLower(strings.TrimSpace(action.Scope))
	if scope == "" {
		scope = "user"
	}
	if scope != "user" {
		return "", fmt.Errorf("仅支持 scope=user 的子任务重试动作")
	}

	_, targetAgentID, root, queue, tasks, conversationID, parentTaskID, err := resolveActionPlanTaskQueueContext(
		runtime,
		decision,
		action,
		fallbackCWD,
		runtimes,
		defaultAgentID,
	)
	if err != nil {
		return "", err
	}

	taskID := strings.TrimSpace(action.TaskID)
	task, ok := pickRuntimeTaskCandidate(tasks, taskID, conversationID, parentTaskID, []runtimeorchestrator.TaskStatus{
		runtimeorchestrator.TaskFailed,
		runtimeorchestrator.TaskCanceled,
	})
	if !ok {
		if taskID != "" {
			return "", fmt.Errorf("runtime.task.retry 未找到 task_id=%s", taskID)
		}
		return "", fmt.Errorf("runtime.task.retry 未找到可重试任务")
	}
	taskID = strings.TrimSpace(task.TaskID)
	if parentTaskID == "" {
		parentTaskID = readRuntimeTaskPayloadString(task.Payload, "parent_task_id")
	}

	retried, retriedOK, retryErr := queue.RetryTask(taskID)
	if retryErr != nil {
		return "", retryErr
	}
	if !retriedOK {
		return "", fmt.Errorf("runtime.task.retry 目标任务状态不允许重试: task_id=%s status=%s", taskID, strings.TrimSpace(string(task.Status)))
	}

	now := time.Now().UTC()
	patch := map[string]interface{}{
		"last_status":        "queued",
		"retry_requested_at": now.Format(time.RFC3339),
		"retry_requested_by": strings.TrimSpace(runtime.agentID),
	}
	if _, _, mergeErr := queue.MergePayload(taskID, patch); mergeErr != nil {
		log.Printf("runtime.task.retry payload merge warning: task_id=%s err=%v", taskID, mergeErr)
	}

	dispatchedCount := 0
	dispatchWarning := ""
	if leadMode, _, leadErr := loadLeadModeFromRuntimeMeta(root); leadErr == nil && leadMode {
		registry := runtimeorchestrator.NewRegistry(filepath.Join(root, ".clawx", "runtime", "worker_states.json"), filepath.Join(root, ".clawx", "runtime", "heartbeats.json"))
		dispatcher := runtimeorchestrator.NewDispatcher(queue, registry, filepath.Join(root, ".clawx", "runtime", "dispatch_state.json"))
		if decisions, dispatchErr := dispatcher.DispatchQueued(time.Now().UTC(), 1); dispatchErr == nil {
			dispatchedCount = len(decisions)
		} else {
			dispatchWarning = summarizeText(dispatchErr.Error(), 100)
		}
	}

	if conversationID != "" {
		setExecutionGoalState(conversationID, executionGoalState{
			AgentID:    strings.TrimSpace(targetAgentID),
			Status:     "running",
			LastResult: summarizeText(fmt.Sprintf("已重试子任务 %s", taskID), 220),
			NextAction: "等待子任务重试结果并继续汇总",
		})
		updateExecutionGoalWithDelegateProgress(queue, conversationID, parentTaskID, targetAgentID)
	}

	parts := []string{
		fmt.Sprintf("runtime.task.retry 完成：agent=%s task_id=%s status=%s retry=%d/%d", fallbackValue(strings.TrimSpace(targetAgentID), "main"), taskID, strings.TrimSpace(string(retried.Status)), retried.Retry, retried.MaxRetry),
	}
	if parentTaskID != "" {
		parts = append(parts, "parent_task_id="+parentTaskID)
	}
	parts = append(parts, "workspace="+root)
	if dispatchedCount > 0 {
		parts = append(parts, fmt.Sprintf("dispatched=%d", dispatchedCount))
	}
	if dispatchWarning != "" {
		parts = append(parts, "dispatch_warning="+dispatchWarning)
	}
	return strings.Join(parts, " "), nil
}

func applyActionPlanRuntimeTaskCancel(
	runtime agentRuntime,
	decision service.Decision,
	action actionPlanItem,
	fallbackCWD string,
	runtimes map[string]agentRuntime,
	defaultAgentID string,
) (string, error) {
	scope := strings.ToLower(strings.TrimSpace(action.Scope))
	if scope == "" {
		scope = "user"
	}
	if scope != "user" {
		return "", fmt.Errorf("仅支持 scope=user 的子任务取消动作")
	}

	_, targetAgentID, root, queue, tasks, conversationID, parentTaskID, err := resolveActionPlanTaskQueueContext(
		runtime,
		decision,
		action,
		fallbackCWD,
		runtimes,
		defaultAgentID,
	)
	if err != nil {
		return "", err
	}

	taskID := strings.TrimSpace(action.TaskID)
	task, ok := pickRuntimeTaskCandidate(tasks, taskID, conversationID, parentTaskID, []runtimeorchestrator.TaskStatus{
		runtimeorchestrator.TaskRunning,
		runtimeorchestrator.TaskQueued,
	})
	if !ok {
		if taskID != "" {
			return "", fmt.Errorf("runtime.task.cancel 未找到 task_id=%s", taskID)
		}
		return "", fmt.Errorf("runtime.task.cancel 未找到可取消任务")
	}
	taskID = strings.TrimSpace(task.TaskID)
	if parentTaskID == "" {
		parentTaskID = readRuntimeTaskPayloadString(task.Payload, "parent_task_id")
	}
	previousStatus := strings.TrimSpace(string(task.Status))

	canceled, canceledOK, cancelErr := queue.CancelTask(taskID)
	if cancelErr != nil {
		return "", cancelErr
	}
	if !canceledOK {
		return "", fmt.Errorf("runtime.task.cancel 目标任务状态不允许取消: task_id=%s status=%s", taskID, previousStatus)
	}

	now := time.Now().UTC()
	patch := map[string]interface{}{
		"last_status":         "canceled",
		"cancel_requested_at": now.Format(time.RFC3339),
		"cancel_requested_by": strings.TrimSpace(runtime.agentID),
	}
	if _, _, mergeErr := queue.MergePayload(taskID, patch); mergeErr != nil {
		log.Printf("runtime.task.cancel payload merge warning: task_id=%s err=%v", taskID, mergeErr)
	}

	if conversationID != "" {
		setExecutionGoalState(conversationID, executionGoalState{
			AgentID:    strings.TrimSpace(targetAgentID),
			Status:     "blocked",
			LastResult: summarizeText(fmt.Sprintf("已取消子任务 %s", taskID), 220),
			NextAction: "确认是否需要改为重试或重新委派",
		})
		updateExecutionGoalWithDelegateProgress(queue, conversationID, parentTaskID, targetAgentID)
	}

	parts := []string{
		fmt.Sprintf("runtime.task.cancel 完成：agent=%s task_id=%s previous_status=%s status=%s", fallbackValue(strings.TrimSpace(targetAgentID), "main"), taskID, fallbackValue(previousStatus, "unknown"), strings.TrimSpace(string(canceled.Status))),
	}
	if parentTaskID != "" {
		parts = append(parts, "parent_task_id="+parentTaskID)
	}
	parts = append(parts, "workspace="+root)
	if strings.EqualFold(previousStatus, string(runtimeorchestrator.TaskRunning)) {
		parts = append(parts, "note=若任务已在执行中，本次取消为尽力停止（后续执行器轮次将不再继续该任务）")
	}
	return strings.Join(parts, " "), nil
}

func resolveActionPlanTaskQueueContext(
	runtime agentRuntime,
	decision service.Decision,
	action actionPlanItem,
	fallbackCWD string,
	runtimes map[string]agentRuntime,
	defaultAgentID string,
) (agentRuntime, string, string, *runtimeorchestrator.Queue, []runtimeorchestrator.RuntimeTask, string, string, error) {
	targetAgentID := cleanAgentIDToken(action.AgentID)
	if targetAgentID == "" {
		targetAgentID = strings.TrimSpace(runtime.agentID)
	}
	if targetAgentID == "" {
		targetAgentID = strings.TrimSpace(defaultAgentID)
	}
	targetRuntime := runtime
	if selected := selectRuntime(runtimes, defaultAgentID, targetAgentID); strings.TrimSpace(selected.agentID) != "" {
		targetRuntime = selected
	}
	if strings.TrimSpace(targetRuntime.agentID) == "" {
		targetRuntime.agentID = targetAgentID
	}
	if strings.TrimSpace(targetAgentID) == "" {
		targetAgentID = strings.TrimSpace(targetRuntime.agentID)
	}

	root := resolveTaskTrackingRoot(targetRuntime, fallbackCWD)
	if strings.TrimSpace(root) == "" {
		return agentRuntime{}, "", "", nil, nil, "", "", fmt.Errorf("未解析到可用 runtime workspace")
	}
	queue := runtimeorchestrator.NewQueue(filepath.Join(root, ".clawx", "runtime", "tasks.jsonl"))
	tasks, err := queue.List()
	if err != nil {
		return agentRuntime{}, "", "", nil, nil, "", "", err
	}
	conversationID := strings.TrimSpace(action.ConversationID)
	if conversationID == "" {
		conversationID = strings.TrimSpace(decision.ConversationID)
	}
	parentTaskID := strings.TrimSpace(action.ParentTaskID)
	if parentTaskID == "" && conversationID != "" {
		if goal, ok := getExecutionGoalState(conversationID); ok {
			parentTaskID = strings.TrimSpace(goal.TaskID)
		}
	}
	return targetRuntime, targetAgentID, root, queue, tasks, conversationID, parentTaskID, nil
}

func pickRuntimeTaskCandidate(
	tasks []runtimeorchestrator.RuntimeTask,
	taskID string,
	conversationID string,
	parentTaskID string,
	preferred []runtimeorchestrator.TaskStatus,
) (runtimeorchestrator.RuntimeTask, bool) {
	taskID = strings.TrimSpace(taskID)
	if taskID != "" {
		for _, task := range tasks {
			if strings.TrimSpace(task.TaskID) == taskID {
				return task, true
			}
		}
		return runtimeorchestrator.RuntimeTask{}, false
	}

	filtered := make([]runtimeorchestrator.RuntimeTask, 0, len(tasks))
	for _, task := range tasks {
		if parentTaskID != "" && readRuntimeTaskPayloadString(task.Payload, "parent_task_id") != parentTaskID {
			continue
		}
		taskConversationID := readRuntimeTaskPayloadString(task.Payload, "conversation")
		if conversationID != "" && taskConversationID != "" && taskConversationID != conversationID {
			continue
		}
		filtered = append(filtered, task)
	}
	if len(filtered) == 0 {
		return runtimeorchestrator.RuntimeTask{}, false
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return runtimeTaskSortTimestamp(filtered[i]).After(runtimeTaskSortTimestamp(filtered[j]))
	})
	for _, wanted := range preferred {
		for _, task := range filtered {
			if task.Status == wanted {
				return task, true
			}
		}
	}
	return runtimeorchestrator.RuntimeTask{}, false
}

func runtimeTaskSortTimestamp(task runtimeorchestrator.RuntimeTask) time.Time {
	for _, raw := range []string{strings.TrimSpace(task.UpdatedAt), strings.TrimSpace(task.CreatedAt)} {
		if raw == "" {
			continue
		}
		if ts, err := time.Parse(time.RFC3339, raw); err == nil {
			return ts.UTC()
		}
	}
	return time.Time{}
}

func loadWorkspaceTaskTrackingState(root string) (workspaceTaskTrackingState, bool, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return workspaceTaskTrackingState{}, false, nil
	}
	statePath := filepath.Join(root, workspaceStateDirName, workspaceStateFileName)
	body, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return workspaceTaskTrackingState{}, false, nil
		}
		return workspaceTaskTrackingState{}, false, fmt.Errorf("读取任务状态快照失败: %w", err)
	}
	var state workspaceTaskTrackingState
	if err := json.Unmarshal(body, &state); err != nil {
		return workspaceTaskTrackingState{}, false, fmt.Errorf("解析任务状态快照失败: %w", err)
	}
	return state, true, nil
}

func readTaskTrackingString(snapshot map[string]interface{}, key string) string {
	if len(snapshot) == 0 {
		return ""
	}
	value, ok := snapshot[strings.TrimSpace(key)]
	if !ok {
		return ""
	}
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
}

func readTaskTrackingInt(snapshot map[string]interface{}, key string) int {
	if len(snapshot) == 0 {
		return 0
	}
	value, ok := snapshot[strings.TrimSpace(key)]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func readTaskTrackingStringList(snapshot map[string]interface{}, key string) []string {
	if len(snapshot) == 0 {
		return nil
	}
	value, ok := snapshot[strings.TrimSpace(key)]
	if !ok {
		return nil
	}
	items, ok := value.([]interface{})
	if !ok || len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		typed, ok := item.(string)
		if !ok {
			continue
		}
		typed = strings.TrimSpace(typed)
		if typed == "" {
			continue
		}
		out = append(out, typed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func limitExecutionList(items []string, limit int) []string {
	normalized := normalizeExecutionStringList(items)
	if len(normalized) <= limit || limit <= 0 {
		return normalized
	}
	return append([]string(nil), normalized[:limit]...)
}

func normalizeTaskStatusValue(raw string) string {
	status := strings.TrimSpace(strings.ToLower(raw))
	if status == "" {
		return ""
	}
	switch status {
	case "applied", "done", "completed", "success", "succeeded":
		return "completed"
	case "partial":
		return "partial"
	case "failed", "error":
		return "failed"
	case "running", "in_progress", "in-progress":
		return "running"
	case "pending", "queued", "waiting":
		return "pending"
	default:
		return status
	}
}

func normalizeReleaseStatusValue(raw string) string {
	status := strings.TrimSpace(strings.ToLower(raw))
	if status == "" {
		return ""
	}
	switch status {
	case "success", "succeeded", "done", "completed", "released", "ok", "成功", "已发布", "发布成功":
		return "succeeded"
	case "failed", "error", "失败", "发布失败":
		return "failed"
	case "running", "in_progress", "in-progress", "deploying", "publishing", "进行中", "发布中":
		return "running"
	case "canceled", "cancelled", "已取消":
		return "canceled"
	case "none", "empty", "未发布":
		return "none"
	default:
		return status
	}
}

func inferReleaseStatusActionFromUserText(text string, runtime agentRuntime, runtimes map[string]agentRuntime, defaultAgentID string) (actionPlanItem, bool) {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(text)), " "))
	if normalized == "" {
		return actionPlanItem{}, false
	}
	if !looksLikeReleaseStatusQuery(normalized) {
		return actionPlanItem{}, false
	}
	service := inferManagedReleaseServiceFromText(normalized, runtime, runtimes, defaultAgentID)
	if service == "" {
		return actionPlanItem{}, false
	}
	operation := "current"
	if containsAnyReleasePhrase(normalized,
		"history", "历史", "最近发布", "最近几次", "发布记录", "回滚记录", "releases", "records",
	) {
		operation = "history"
	}
	action := actionPlanItem{
		Kind:      "runtime.release.status",
		Service:   service,
		Operation: operation,
		Scope:     "user",
	}
	if operation == "history" {
		action.Limit = inferManagedReleaseHistoryLimitFromText(normalized)
	}
	return action, true
}

func inferTaskStatusActionFromUserText(text string, runtime agentRuntime, runtimes map[string]agentRuntime, defaultAgentID string) (actionPlanItem, bool) {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(text)), " "))
	if normalized == "" {
		return actionPlanItem{}, false
	}
	if !looksLikeTaskStatusQuery(normalized) {
		return actionPlanItem{}, false
	}
	targetAgentID := inferManagedReleaseAgentFromText(normalized, runtime, runtimes, defaultAgentID)
	action := actionPlanItem{
		Kind:  "runtime.task.status",
		Scope: "user",
	}
	if strings.TrimSpace(targetAgentID) != "" {
		action.AgentID = strings.TrimSpace(targetAgentID)
	}
	return action, true
}

func inferTaskDelegatesActionFromUserText(text string, runtime agentRuntime, runtimes map[string]agentRuntime, defaultAgentID string) (actionPlanItem, bool) {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(text)), " "))
	if normalized == "" {
		return actionPlanItem{}, false
	}
	if !looksLikeTaskDelegatesQuery(normalized) {
		return actionPlanItem{}, false
	}
	targetAgentID := inferManagedReleaseAgentFromText(normalized, runtime, runtimes, defaultAgentID)
	action := actionPlanItem{
		Kind:  "runtime.task.delegates",
		Scope: "user",
	}
	if strings.TrimSpace(targetAgentID) != "" {
		action.AgentID = strings.TrimSpace(targetAgentID)
	}
	return action, true
}

func enforceStatusTruthQueryPlanFromUserText(
	plan actionPlan,
	userText string,
	runtime agentRuntime,
	runtimes map[string]agentRuntime,
	defaultAgentID string,
	conversationID string,
	scopeKey string,
	fallbackCWD string,
) (actionPlan, bool) {
	releaseAction, releaseOK := inferReleaseStatusActionFromUserText(userText, runtime, runtimes, defaultAgentID)
	taskDelegatesAction, taskDelegatesOK := inferTaskDelegatesActionFromUserText(userText, runtime, runtimes, defaultAgentID)
	taskAction, taskOK := inferTaskStatusActionFromUserText(userText, runtime, runtimes, defaultAgentID)
	taskControlAction, taskControlReason, taskControlOK := inferTaskControlActionFromUserText(userText, runtime, runtimes, defaultAgentID, conversationID, scopeKey, fallbackCWD)
	if taskControlOK && strings.TrimSpace(strings.ToLower(taskControlReason)) != "auto_task_control_query" {
		taskControlOK = false
	}
	if !releaseOK && !taskDelegatesOK && !taskOK && !taskControlOK {
		return plan, false
	}

	expectedActions := make([]actionPlanItem, 0, 4)
	if releaseOK {
		expectedActions = append(expectedActions, releaseAction)
	}
	if taskDelegatesOK {
		expectedActions = append(expectedActions, taskDelegatesAction)
	}
	if taskOK {
		expectedActions = append(expectedActions, taskAction)
	}
	if taskControlOK {
		taskControlAction.Mode = "query"
		expectedActions = append(expectedActions, taskControlAction)
	}
	expectedActions = dedupeStatusTruthQueryActions(expectedActions)
	if len(expectedActions) == 0 {
		return plan, false
	}
	releaseKept, taskDelegatesKept, taskKept, taskControlKept := summarizeStatusTruthQueryActionKinds(expectedActions)

	hasMutation := false
	for _, action := range plan.Actions {
		kind := strings.TrimSpace(strings.ToLower(action.Kind))
		if !isReadOnlyActionPlanItem(kind, action) {
			hasMutation = true
			break
		}
	}
	if !hasMutation {
		return plan, false
	}

	coerced := actionPlan{
		Type:    "action_plan",
		Mode:    strings.TrimSpace(strings.ToLower(plan.Mode)),
		Reason:  buildStatusTruthQueryPlanReason(releaseKept, taskDelegatesKept, taskKept, taskControlKept),
		Actions: expectedActions,
	}
	if coerced.Mode == "" {
		coerced.Mode = "execute"
	}
	return coerced, true
}

func buildStatusTruthQueryPlanReason(release, taskDelegates, task, taskControl bool) string {
	tokens := make([]string, 0, 4)
	if release {
		tokens = append(tokens, "release")
	}
	if taskDelegates {
		tokens = append(tokens, "task_delegates")
	}
	if task {
		tokens = append(tokens, "task")
	}
	if taskControl {
		tokens = append(tokens, "task_control")
	}
	if len(tokens) == 0 {
		return "auto_status_truth_query"
	}
	return "auto_status_truth_query." + strings.Join(tokens, "_")
}

func dedupeStatusTruthQueryActions(actions []actionPlanItem) []actionPlanItem {
	if len(actions) <= 1 {
		return actions
	}
	hasTaskControlStatus := false
	hasTaskDelegates := false
	for _, action := range actions {
		kind := strings.TrimSpace(strings.ToLower(action.Kind))
		switch kind {
		case "runtime.task.control":
			operation, err := normalizeManagedTaskControlOperation(action.Operation)
			if err == nil && operation == "status" {
				hasTaskControlStatus = true
			}
		case "runtime.task.delegates":
			hasTaskDelegates = true
		}
	}
	out := make([]actionPlanItem, 0, len(actions))
	seen := map[string]struct{}{}
	for _, action := range actions {
		kind := strings.TrimSpace(strings.ToLower(action.Kind))
		if kind == "runtime.task.status" && (hasTaskControlStatus || hasTaskDelegates) {
			continue
		}
		signature := statusTruthQueryActionSignature(action)
		if signature == "" {
			continue
		}
		if _, exists := seen[signature]; exists {
			continue
		}
		seen[signature] = struct{}{}
		out = append(out, action)
	}
	return out
}

func summarizeStatusTruthQueryActionKinds(actions []actionPlanItem) (release bool, taskDelegates bool, task bool, taskControl bool) {
	for _, action := range actions {
		kind := strings.TrimSpace(strings.ToLower(action.Kind))
		switch kind {
		case "runtime.release.status":
			release = true
		case "runtime.task.delegates":
			taskDelegates = true
		case "runtime.task.status":
			task = true
		case "runtime.task.control":
			operation, err := normalizeManagedTaskControlOperation(action.Operation)
			if err == nil && operation == "status" {
				taskControl = true
			}
		}
	}
	return release, taskDelegates, task, taskControl
}

func statusTruthQueryActionSignature(action actionPlanItem) string {
	kind := strings.TrimSpace(strings.ToLower(action.Kind))
	if kind == "" {
		return ""
	}
	switch kind {
	case "runtime.release.status":
		op := strings.TrimSpace(strings.ToLower(action.Operation))
		if op == "" {
			op = "current"
		}
		return strings.Join([]string{
			kind,
			strings.TrimSpace(strings.ToLower(action.Service)),
			op,
			strconv.Itoa(action.Limit),
		}, "|")
	case "runtime.task.control":
		op, err := normalizeManagedTaskControlOperation(action.Operation)
		if err != nil {
			op = strings.TrimSpace(strings.ToLower(action.Operation))
		}
		return strings.Join([]string{
			kind,
			strings.TrimSpace(strings.ToLower(action.AgentID)),
			strings.TrimSpace(strings.ToLower(action.Service)),
			op,
		}, "|")
	default:
		return strings.Join([]string{
			kind,
			strings.TrimSpace(strings.ToLower(action.AgentID)),
		}, "|")
	}
}

func inferTaskControlActionFromUserText(text string, runtime agentRuntime, runtimes map[string]agentRuntime, defaultAgentID string, conversationID string, routeScopeKey string, fallbackCWD string) (actionPlanItem, string, bool) {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(text)), " "))
	if normalized == "" {
		return actionPlanItem{}, "", false
	}
	if !looksLikeTaskControlRequest(normalized) {
		return actionPlanItem{}, "", false
	}
	operation, ok := inferTaskControlOperationFromText(normalized)
	if !ok {
		return actionPlanItem{}, "", false
	}
	targetAgentID := inferManagedReleaseAgentFromText(normalized, runtime, runtimes, defaultAgentID)
	targetRuntime := runtime
	if selected := selectRuntime(runtimes, defaultAgentID, targetAgentID); strings.TrimSpace(selected.agentID) != "" {
		targetRuntime = selected
	}
	if strings.TrimSpace(targetRuntime.agentID) == "" {
		targetRuntime.agentID = strings.TrimSpace(targetAgentID)
	}
	service := inferManagedReleaseServiceFromText(normalized, runtime, runtimes, defaultAgentID)
	if strings.TrimSpace(service) == "" {
		service = inferManagedServiceNameForAgent(targetAgentID)
	}
	action := actionPlanItem{
		Kind:      "runtime.task.control",
		Operation: operation,
		Scope:     "user",
		Service:   strings.TrimSpace(service),
	}
	if strings.TrimSpace(targetAgentID) != "" {
		action.AgentID = strings.TrimSpace(targetAgentID)
	}
	if operation != "stop" && operation != "status" {
		if inferredHealthURL := inferTaskControlHealthURL(text, action.Service, targetRuntime, fallbackCWD, conversationID, routeScopeKey); inferredHealthURL != "" {
			action.HealthURL = inferredHealthURL
		}
	}
	if operation == "status" {
		return action, "auto_task_control_query", true
	}
	return action, "auto_task_control_execute", true
}

func constrainActionPlanForRetryOnly(actions []actionPlanItem) []actionPlanItem {
	if len(actions) == 0 {
		return nil
	}
	filtered := make([]actionPlanItem, 0, len(actions))
	for _, action := range actions {
		kind := strings.TrimSpace(strings.ToLower(action.Kind))
		switch kind {
		case "runtime.exec":
			if isRetryOnlyRuntimeExecCommand(action.Cmd) {
				filtered = append(filtered, action)
			}
		case "runtime.service":
			op := strings.TrimSpace(strings.ToLower(action.Operation))
			switch op {
			case "restart", "status", "start":
				filtered = append(filtered, action)
			}
		case "runtime.task.control":
			op := strings.TrimSpace(strings.ToLower(action.Operation))
			switch op {
			case "", "ensure_running", "ensure-running", "ensure", "restart", "status", "start":
				filtered = append(filtered, action)
			}
		case "runtime.task.status", "runtime.release.status":
			filtered = append(filtered, action)
		}
	}
	return filtered
}

func isRetryOnlyRuntimeExecCommand(cmdline string) bool {
	lower := normalizeRuntimeExecCommandline(cmdline)
	if lower == "" {
		return false
	}

	dangerKeywords := []string{
		" migrate ", " migration ", "alembic", "goose up", "flyway ", "liquibase", "dbmate",
		"drop table", "truncate table", "delete from", "update ", "insert into", "alter table", "create table",
		"redis-cli flushall", "redis-cli flushdb",
	}
	if matchesAnyRuntimeExecKeyword(lower, dangerKeywords) {
		return false
	}

	safeKeywords := []string{
		"systemctl restart", "systemctl status", "systemctl is-active", "systemctl start",
		"service ", " restart", " status", " start",
		"supervisorctl restart", "supervisorctl status",
		"pm2 restart", "pm2 status",
		"docker restart", "docker ps", "docker logs",
		"journalctl", "tail -n", "grep ", "cat ",
		"curl ", "wget ",
		"/health", "/healthz", "/ready", "/readyz",
		"echo ", "printf ", "sleep ",
	}
	return matchesAnyRuntimeExecKeyword(lower, safeKeywords)
}

func constrainActionPlanForServiceDeepRepair(actions []actionPlanItem) []actionPlanItem {
	if len(actions) == 0 {
		return nil
	}
	filtered := make([]actionPlanItem, 0, len(actions))
	for _, action := range actions {
		kind := strings.TrimSpace(strings.ToLower(action.Kind))
		switch kind {
		case "runtime.exec":
			if isServiceDeepRepairRuntimeExecCommand(action.Cmd) {
				filtered = append(filtered, action)
			}
		case "runtime.service":
			op := strings.TrimSpace(strings.ToLower(action.Operation))
			switch op {
			case "restart", "status", "start", "stop":
				filtered = append(filtered, action)
			}
		case "runtime.task.control":
			op := strings.TrimSpace(strings.ToLower(action.Operation))
			switch op {
			case "", "ensure_running", "ensure-running", "ensure", "restart", "status", "start", "stop":
				filtered = append(filtered, action)
			}
		case "runtime.supervisor":
			op := strings.TrimSpace(strings.ToLower(action.Operation))
			switch op {
			case "", "ensure", "status":
				filtered = append(filtered, action)
			}
		case "runtime.task.status", "runtime.release.status":
			filtered = append(filtered, action)
		}
	}
	return filtered
}

func isServiceDeepRepairRuntimeExecCommand(cmdline string) bool {
	lower := normalizeRuntimeExecCommandline(cmdline)
	if lower == "" {
		return false
	}
	hardBlockKeywords := []string{
		" rm -rf ", "mkfs", " dd if=", "shutdown", " reboot", " halt",
		"sudo ", " su ", " su -", "chown ", "chmod 777",
		"git reset --hard", "git clean -fd", "git clean -xdf",
		"| sh", "| bash", "| zsh", "| python", "| perl", "sh -c", "bash -c", "zsh -c",
		"nc -e", "ncat -e", "netcat -e", "powershell -enc",
	}
	if matchesAnyRuntimeExecKeyword(lower, hardBlockKeywords) {
		return false
	}

	modeMismatchKeywords := []string{
		"pip install", "pip3 install", "python -m pip install", "python3 -m pip install",
		"uv pip", "poetry install", "poetry lock",
		"npm install", "npm ci", "npm run", "pnpm ", "yarn ", "npx ",
		"go mod ", "go get ", "go build ", "go test ",
		"cargo build", "cargo test", "cargo check",
		"mvn ", "gradle ", "./gradlew", "make ", "cmake ",
		"composer install", "bundle install",
	}
	if matchesAnyRuntimeExecKeyword(lower, modeMismatchKeywords) {
		return false
	}

	allowKeywords := []string{
		" migrate ", " migration ", "alembic", "goose", "flyway", "liquibase", "dbmate",
		" schema ", "sql ", "psql", "mysql", "sqlite3", "redis-cli",
		"repair", "backfill", "vacuum", "reindex", "consistency", "checksum",
		"journalctl", "systemctl", "supervisorctl", "docker logs", "kubectl logs",
		"tail -n", "grep ", "cat ", "curl ", "wget ", "/health", "/ready",
	}
	return matchesAnyRuntimeExecKeyword(lower, allowKeywords)
}

func constrainActionPlanForBuildContinue(actions []actionPlanItem) []actionPlanItem {
	if len(actions) == 0 {
		return nil
	}
	filtered := make([]actionPlanItem, 0, len(actions))
	for _, action := range actions {
		kind := strings.TrimSpace(strings.ToLower(action.Kind))
		switch kind {
		case "runtime.exec":
			if isBuildContinueRuntimeExecCommand(action.Cmd) {
				filtered = append(filtered, action)
			}
		case "runtime.service":
			op := strings.TrimSpace(strings.ToLower(action.Operation))
			switch op {
			case "restart", "status", "start":
				filtered = append(filtered, action)
			}
		case "runtime.task.control":
			op := strings.TrimSpace(strings.ToLower(action.Operation))
			switch op {
			case "", "ensure_running", "ensure-running", "ensure", "restart", "status", "start":
				filtered = append(filtered, action)
			}
		case "runtime.supervisor":
			op := strings.TrimSpace(strings.ToLower(action.Operation))
			switch op {
			case "", "ensure", "status":
				filtered = append(filtered, action)
			}
		case "runtime.task.status", "runtime.release.status":
			filtered = append(filtered, action)
		}
	}
	return filtered
}

func isBuildContinueRuntimeExecCommand(cmdline string) bool {
	lower := normalizeRuntimeExecCommandline(cmdline)
	if lower == "" {
		return false
	}
	hardBlockKeywords := []string{
		" rm -rf ", "mkfs", " dd if=", "shutdown", " reboot", " halt",
		"sudo ", " su ", " su -", "chown ", "chmod 777",
		"git reset --hard", "git clean -fd", "git clean -xdf",
		"| sh", "| bash", "| zsh", "| python", "| perl", "sh -c", "bash -c", "zsh -c",
		"nc -e", "ncat -e", "netcat -e", "powershell -enc",
	}
	if matchesAnyRuntimeExecKeyword(lower, hardBlockKeywords) {
		return false
	}

	modeMismatchKeywords := []string{
		" migrate ", " migration ", "alembic", "goose", "flyway", "liquibase", "dbmate",
		" schema ", "sql ", "psql", "mysql", "sqlite3", "redis-cli",
		"drop table", "truncate table", "delete from", "insert into", "alter table", "create table",
		"backfill", "vacuum", "reindex", "checksum",
	}
	if matchesAnyRuntimeExecKeyword(lower, modeMismatchKeywords) {
		return false
	}

	allowKeywords := []string{
		"pip install", "pip3 install", "python -m pip install", "python3 -m pip install",
		"uv pip", "poetry install", "poetry lock",
		"npm install", "npm ci", "npm run", "pnpm ", "yarn ", "npx ",
		"go mod ", "go get ", "go build ", "go test ", "go env goproxy",
		"cargo build", "cargo test", "cargo check",
		"mvn ", "gradle ", "./gradlew", "make ", "cmake ",
		"composer install", "bundle install",
		"pip_index_url=", "npm_config_registry=", "goproxy=",
	}
	return matchesAnyRuntimeExecKeyword(lower, allowKeywords)
}

func normalizeRuntimeExecCommandline(cmdline string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(cmdline)), " "))
}

func matchesAnyRuntimeExecKeyword(normalized string, keywords []string) bool {
	normalized = strings.TrimSpace(strings.ToLower(normalized))
	if normalized == "" || len(keywords) == 0 {
		return false
	}
	padded := " " + normalized + " "
	for _, rawKeyword := range keywords {
		keyword := strings.ToLower(rawKeyword)
		if strings.TrimSpace(keyword) == "" {
			continue
		}
		if strings.HasPrefix(keyword, " ") || strings.HasSuffix(keyword, " ") {
			if strings.Contains(padded, keyword) {
				return true
			}
			continue
		}
		if strings.Contains(normalized, strings.TrimSpace(keyword)) {
			return true
		}
	}
	return false
}

func defaultServiceDeepRepairDiagnosticCommand(service string) string {
	service = strings.TrimSpace(service)
	if service == "" {
		service = "clawx"
	}
	return fmt.Sprintf("command -v journalctl >/dev/null 2>&1 && journalctl --user -u %s.service -n 120 --no-pager || true", service)
}

func defaultBuildContinueDiagnosticCommand() string {
	return "command -v go >/dev/null 2>&1 && go env GOPROXY || true; command -v python3 >/dev/null 2>&1 && python3 -m pip --version || true; command -v npm >/dev/null 2>&1 && npm config get registry || true"
}

func resolveRuntimeExecDecisionFallbackCommand(mode string, runtime agentRuntime, fallbackCWD string, defaultCmd string) (string, string) {
	mode = normalizeRuntimeExecDecisionMode(mode)
	defaultCmd = strings.TrimSpace(defaultCmd)
	if mode == "" {
		return defaultCmd, "default"
	}
	if cmdline := lookupRuntimeExecDecisionFallbackFromWorkspace(mode, runtime, fallbackCWD); cmdline != "" {
		return cmdline, "workspace"
	}
	if cmdline := lookupRuntimeExecDecisionFallbackFromEnv(mode, runtime.agentID); cmdline != "" {
		return cmdline, "env"
	}
	return defaultCmd, "default"
}

func lookupRuntimeExecDecisionFallbackFromWorkspace(mode string, runtime agentRuntime, fallbackCWD string) string {
	root := resolveTaskTrackingRoot(runtime, fallbackCWD)
	if strings.TrimSpace(root) == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(root, ".clawx", "runtime", "decision_fallbacks.json"),
		filepath.Join(root, ".clawx", "runtime", "runtime_meta.json"),
	}
	for _, path := range candidates {
		cmdline := extractRuntimeExecDecisionFallbackFromFile(path, mode, runtime.agentID)
		if cmdline != "" {
			return cmdline
		}
	}
	return ""
}

func extractRuntimeExecDecisionFallbackFromFile(path string, mode string, agentID string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	mode = normalizeRuntimeExecDecisionMode(mode)
	agentID = strings.TrimSpace(strings.ToLower(agentID))

	agentMaps := []string{"agents", "runtime_exec_decision_fallbacks_by_agent", "decision_fallbacks_by_agent"}
	for _, key := range agentMaps {
		if cmdline := extractRuntimeExecDecisionFallbackFromAgentMap(payload[key], agentID, mode); cmdline != "" {
			return cmdline
		}
	}

	modeMaps := []string{"runtime_exec_decision_fallbacks", "decision_fallbacks", "modes"}
	for _, key := range modeMaps {
		if cmdline := extractRuntimeExecDecisionFallbackFromModeMap(payload[key], mode); cmdline != "" {
			return cmdline
		}
	}

	if cmdline := extractRuntimeExecDecisionFallbackFromModeMap(payload, mode); cmdline != "" {
		return cmdline
	}
	return ""
}

func extractRuntimeExecDecisionFallbackFromAgentMap(raw interface{}, agentID string, mode string) string {
	agents, ok := raw.(map[string]interface{})
	if !ok || len(agents) == 0 {
		return ""
	}
	keys := make([]string, 0, 2)
	if agentID != "" {
		keys = append(keys, agentID)
	}
	keys = append(keys, "default")
	for _, key := range keys {
		rawModes, ok := agents[key]
		if !ok {
			continue
		}
		if cmdline := extractRuntimeExecDecisionFallbackFromModeMap(rawModes, mode); cmdline != "" {
			return cmdline
		}
	}
	return ""
}

func extractRuntimeExecDecisionFallbackFromModeMap(raw interface{}, mode string) string {
	modeMap, ok := raw.(map[string]interface{})
	if !ok || len(modeMap) == 0 {
		return ""
	}
	keys := []string{
		mode,
		strings.ReplaceAll(mode, ".", "_"),
		strings.ReplaceAll(mode, ".", "-"),
	}
	for _, key := range keys {
		value, ok := modeMap[key]
		if !ok {
			continue
		}
		cmdline, _ := value.(string)
		cmdline = strings.TrimSpace(cmdline)
		if cmdline != "" {
			return cmdline
		}
	}
	return ""
}

func lookupRuntimeExecDecisionFallbackFromEnv(mode string, agentID string) string {
	modeToken := strings.TrimSpace(strings.ToUpper(strings.ReplaceAll(mode, ".", "_")))
	agentToken := managedServiceEnvKeyToken(strings.TrimSpace(agentID))
	keys := make([]string, 0, 6)
	if modeToken != "" && agentToken != "" {
		keys = append(keys, "CLAWX_RUNTIME_EXEC_DECISION_FALLBACK_"+modeToken+"_"+agentToken)
	}
	if modeToken != "" {
		keys = append(keys, "CLAWX_RUNTIME_EXEC_DECISION_FALLBACK_"+modeToken)
	}
	switch mode {
	case "service.deep_repair":
		if agentToken != "" {
			keys = append(keys, "CLAWX_RUNTIME_DEEP_REPAIR_FALLBACK_CMD_"+agentToken)
		}
		keys = append(keys, "CLAWX_RUNTIME_DEEP_REPAIR_FALLBACK_CMD")
	case "build.continue":
		if agentToken != "" {
			keys = append(keys, "CLAWX_RUNTIME_BUILD_CONTINUE_FALLBACK_CMD_"+agentToken)
		}
		keys = append(keys, "CLAWX_RUNTIME_BUILD_CONTINUE_FALLBACK_CMD")
	}
	for _, key := range keys {
		if cmdline := strings.TrimSpace(os.Getenv(key)); cmdline != "" {
			return cmdline
		}
	}
	return ""
}

func rewriteBuildSwitchSourceCommands(actions []actionPlanItem) int {
	rewritten := 0
	for idx := range actions {
		if strings.TrimSpace(strings.ToLower(actions[idx].Kind)) != "runtime.exec" {
			continue
		}
		cmdline := strings.TrimSpace(actions[idx].Cmd)
		if cmdline == "" {
			continue
		}
		updated, ok := rewriteBuildSwitchSourceCommand(cmdline)
		if !ok {
			continue
		}
		actions[idx].Cmd = updated
		rewritten++
	}
	return rewritten
}

func rewriteBuildSwitchSourceCommand(cmdline string) (string, bool) {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(cmdline)), " "))
	if normalized == "" {
		return "", false
	}
	if strings.Contains(normalized, "pip install") {
		if strings.Contains(normalized, "--index-url") || strings.Contains(normalized, " -i ") || strings.Contains(normalized, "pip_index_url=") {
			return "", false
		}
		prefix := `PIP_INDEX_URL="${PIP_INDEX_URL:-https://pypi.tuna.tsinghua.edu.cn/simple}" PIP_TRUSTED_HOST="${PIP_TRUSTED_HOST:-pypi.tuna.tsinghua.edu.cn}" `
		return prefix + cmdline, true
	}
	if strings.Contains(normalized, "npm ") || strings.Contains(normalized, "pnpm ") || strings.Contains(normalized, "yarn ") {
		if strings.Contains(normalized, "--registry") || strings.Contains(normalized, "npm_config_registry=") {
			return "", false
		}
		prefix := `NPM_CONFIG_REGISTRY="${NPM_CONFIG_REGISTRY:-https://registry.npmmirror.com}" `
		return prefix + cmdline, true
	}
	if strings.Contains(normalized, "go mod download") || strings.Contains(normalized, "go get ") || strings.Contains(normalized, "go test ") || strings.Contains(normalized, "go build ") {
		if strings.Contains(normalized, "goproxy=") {
			return "", false
		}
		prefix := `GOPROXY="${GOPROXY:-https://goproxy.cn,direct}" `
		return prefix + cmdline, true
	}
	return "", false
}

func looksLikeTaskControlRequest(normalized string) bool {
	if normalized == "" {
		return false
	}
	if isLikelyTaskControlDesignRequest(normalized) {
		return false
	}
	hasControlVerb := containsAnyReleasePhrase(normalized,
		"重启", "restart", "拉起", "启动", "start", "stop", "停止", "重拉", "恢复", "kill", "status", "状态", "在线吗", "ensure_running", "ensure running",
	)
	if !hasControlVerb {
		return false
	}
	hasRuntimeDomain := containsAnyReleasePhrase(normalized,
		"worker", "服务", "service", "进程", "bot", "daemon", "runtime", "systemd", "clawx", ".service", "实例",
	)
	return hasRuntimeDomain
}

func isLikelyTaskControlDesignRequest(normalized string) bool {
	if normalized == "" {
		return false
	}
	return containsAnyReleasePhrase(normalized,
		"实现", "开发", "写代码", "代码", "机制", "逻辑", "设计", "方案", "文档", "如何", "怎么", "原理", "复刻", "翻译",
	)
}

func inferTaskControlOperationFromText(normalized string) (string, bool) {
	switch {
	case containsAnyReleasePhrase(normalized, "stop", "停止", "关停", "下线", "停掉", "停机"):
		return "stop", true
	case containsAnyReleasePhrase(normalized, "restart", "重启", "重开", "重拉"):
		return "restart", true
	case containsAnyReleasePhrase(normalized, "ensure_running", "ensure running", "拉起", "启动", "start", "恢复", "拉活", "开起来", "确保在线"):
		return "ensure_running", true
	case containsAnyReleasePhrase(normalized, "status", "状态", "在线吗", "活着吗", "running 吗", "是否在线", "现在怎么样"):
		return "status", true
	default:
		return "", false
	}
}

func inferTaskControlHealthURL(rawText string, service string, runtime agentRuntime, fallbackCWD string, conversationID string, routeScopeKey string) string {
	if resolved := inferHealthURLFromText(rawText); resolved != "" {
		log.Printf("task_control_health_hint: source=text service=%s scope=%s conversation_id=%s url=%s", strings.TrimSpace(strings.ToLower(service)), strings.TrimSpace(routeScopeKey), strings.TrimSpace(conversationID), resolved)
		recordTaskControlHealthHintMetric("text", service, routeScopeKey, conversationID, resolved)
		return resolved
	}
	if resolved := inferHealthURLFromLocalPortHint(rawText); resolved != "" {
		log.Printf("task_control_health_hint: source=local_port service=%s scope=%s conversation_id=%s url=%s", strings.TrimSpace(strings.ToLower(service)), strings.TrimSpace(routeScopeKey), strings.TrimSpace(conversationID), resolved)
		recordTaskControlHealthHintMetric("local_port", service, routeScopeKey, conversationID, resolved)
		return resolved
	}
	if resolved, source := inferTaskControlHealthURLFromWorkspaceState(runtime, fallbackCWD, service, conversationID, routeScopeKey); resolved != "" {
		log.Printf("task_control_health_hint: source=%s service=%s scope=%s conversation_id=%s url=%s", fallbackValue(strings.TrimSpace(source), "workspace"), strings.TrimSpace(strings.ToLower(service)), strings.TrimSpace(routeScopeKey), strings.TrimSpace(conversationID), resolved)
		recordTaskControlHealthHintMetric(fallbackValue(strings.TrimSpace(source), "workspace"), service, routeScopeKey, conversationID, resolved)
		return resolved
	}
	if resolved := resolveManagedServiceHealthURLFromEnv(service); resolved != "" {
		log.Printf("task_control_health_hint: source=env service=%s scope=%s conversation_id=%s url=%s", strings.TrimSpace(strings.ToLower(service)), strings.TrimSpace(routeScopeKey), strings.TrimSpace(conversationID), resolved)
		recordTaskControlHealthHintMetric("env", service, routeScopeKey, conversationID, resolved)
		return resolved
	}
	return ""
}

func resolveTaskControlStatusHealthProbe(
	runtime agentRuntime,
	fallbackCWD string,
	service string,
	conversationID string,
	routeScopeKey string,
	rawText string,
	explicitHealthURL string,
) (string, string) {
	if resolved := sanitizeManagedHealthURL(explicitHealthURL); resolved != "" {
		return resolved, "action"
	}
	if resolved := inferHealthURLFromText(rawText); resolved != "" {
		return resolved, "text"
	}
	if resolved := inferHealthURLFromLocalPortHint(rawText); resolved != "" {
		return resolved, "local_port"
	}
	if resolved, source := inferTaskControlHealthURLFromWorkspaceState(runtime, fallbackCWD, service, conversationID, routeScopeKey); resolved != "" {
		return resolved, source
	}
	if resolved := resolveManagedServiceHealthURLFromEnv(service); resolved != "" {
		return resolved, "env"
	}
	return "", ""
}

func probeTaskControlHealthOnce(ctx context.Context, healthURL string, timeout time.Duration) (string, int) {
	healthURL = sanitizeManagedHealthURL(healthURL)
	if healthURL == "" {
		return "", 0
	}
	probeTimeout := managedServiceRecoveryProbeTimeout(timeout)
	if probeTimeout <= 0 {
		probeTimeout = 4 * time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, healthURL, nil)
	if err != nil {
		return "down", 0
	}
	client := &http.Client{Timeout: probeTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "down", 0
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return "up", resp.StatusCode
	}
	return "down", resp.StatusCode
}

func lookupManagedServiceCurrentRelease(service string) (string, string) {
	service = strings.TrimSpace(strings.ToLower(strings.TrimSuffix(service, ".service")))
	if service == "" {
		return "", ""
	}
	stateDir, err := managedReleaseStateDir()
	if err != nil {
		return "", ""
	}
	currentPath := filepath.Join(stateDir, service, "current.json")
	raw, err := os.ReadFile(currentPath)
	if err != nil {
		return "", ""
	}
	var record managedReleaseRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return "", ""
	}
	return strings.TrimSpace(record.ReleaseID), strings.TrimSpace(record.Version)
}

func lookupTaskControlTrackedRelease(runtime agentRuntime, fallbackCWD string, service string) (string, string) {
	root := resolveTaskTrackingRoot(runtime, fallbackCWD)
	if strings.TrimSpace(root) == "" {
		return "", ""
	}
	state, ok, err := loadWorkspaceTaskTrackingState(root)
	if err != nil || !ok {
		return "", ""
	}
	snapshot := state.TaskTrackingSnapshot
	if len(snapshot) == 0 {
		return "", ""
	}
	service = strings.TrimSpace(strings.ToLower(strings.TrimSuffix(service, ".service")))
	if service == "" {
		return "", ""
	}

	serviceReleaseIDs := readTaskTrackingStringMap(snapshot, "service_release_ids")
	serviceReleaseVersions := readTaskTrackingStringMap(snapshot, "service_release_versions")
	releaseID := strings.TrimSpace(serviceReleaseIDs[service])
	releaseVersion := strings.TrimSpace(serviceReleaseVersions[service])
	if releaseID != "" || releaseVersion != "" {
		return releaseID, releaseVersion
	}

	lastService := strings.TrimSpace(strings.ToLower(readTaskTrackingString(snapshot, "last_task_control_service")))
	if lastService == "" || lastService == service {
		releaseID = strings.TrimSpace(readTaskTrackingString(snapshot, "last_task_control_release_id"))
		releaseVersion = strings.TrimSpace(readTaskTrackingString(snapshot, "last_task_control_release_version"))
	}
	return releaseID, releaseVersion
}

func lookupTaskControlLastAction(runtime agentRuntime, fallbackCWD string, service string, conversationID string, routeScopeKey string) (string, string) {
	root := resolveTaskTrackingRoot(runtime, fallbackCWD)
	if strings.TrimSpace(root) == "" {
		return "", ""
	}
	state, ok, err := loadWorkspaceTaskTrackingState(root)
	if err != nil || !ok {
		return "", ""
	}
	snapshot := state.TaskTrackingSnapshot
	if len(snapshot) == 0 {
		return "", ""
	}
	normalizedService := strings.TrimSpace(strings.ToLower(service))
	conversationID = strings.TrimSpace(conversationID)
	routeScopeKey = normalizeTaskControlRouteScopeKey(routeScopeKey)
	if routeScopeKey == "" {
		routeScopeKey = inferRoutingScopeKeyFromScopedConversationID(conversationID)
	}

	routeHints := readTaskTrackingRouteHintMap(snapshot, "task_control_route_hints")
	if routeScopeKey != "" && len(routeHints) > 0 {
		if record, exists := routeHints[routeScopeKey]; exists {
			recordService := strings.TrimSpace(strings.ToLower(record.Service))
			if normalizedService == "" || recordService == "" || recordService == normalizedService {
				return strings.TrimSpace(strings.ToLower(record.Operation)), strings.TrimSpace(record.UpdatedAt)
			}
		}
	}

	lastService := strings.TrimSpace(strings.ToLower(readTaskTrackingString(snapshot, "last_task_control_service")))
	if normalizedService != "" && lastService != "" && lastService != normalizedService {
		return "", ""
	}
	lastOperation := strings.TrimSpace(strings.ToLower(readTaskTrackingString(snapshot, "last_task_control_operation")))
	lastOperationAt := strings.TrimSpace(readTaskTrackingString(snapshot, "last_task_control_at"))
	return lastOperation, lastOperationAt
}

func inferTaskControlHealthURLFromWorkspaceState(runtime agentRuntime, fallbackCWD string, service string, conversationID string, routeScopeKey string) (string, string) {
	root := resolveTaskTrackingRoot(runtime, fallbackCWD)
	if strings.TrimSpace(root) == "" {
		return "", ""
	}
	state, ok, err := loadWorkspaceTaskTrackingState(root)
	if err != nil || !ok {
		return "", ""
	}
	snapshot := state.TaskTrackingSnapshot
	if len(snapshot) == 0 {
		return "", ""
	}
	service = strings.TrimSpace(strings.ToLower(service))
	conversationID = strings.TrimSpace(conversationID)
	routeScopeKey = normalizeTaskControlRouteScopeKey(routeScopeKey)
	if routeScopeKey == "" {
		routeScopeKey = inferRoutingScopeKeyFromScopedConversationID(conversationID)
	}

	routeHints := readTaskTrackingRouteHintMap(snapshot, "task_control_route_hints")
	if routeScopeKey != "" && len(routeHints) > 0 {
		if record, ok := routeHints[routeScopeKey]; ok {
			if service == "" || strings.TrimSpace(record.Service) == "" || strings.TrimSpace(record.Service) == service {
				if value := sanitizeManagedHealthURL(record.HealthURL); value != "" {
					return value, "workspace.route_scope"
				}
			}
		}
	}

	serviceURLs := readTaskTrackingStringMap(snapshot, "service_health_urls")
	if service != "" {
		if value := sanitizeManagedHealthURL(serviceURLs[service]); value != "" {
			return value, "workspace.service_map"
		}
	}
	recordedService := strings.TrimSpace(strings.ToLower(readTaskTrackingString(snapshot, "health_service")))
	recordedConversationID := strings.TrimSpace(readTaskTrackingString(snapshot, "conversation_id"))
	recordedURL := sanitizeManagedHealthURL(readTaskTrackingString(snapshot, "health_url"))
	if recordedURL == "" {
		return "", ""
	}
	if service != "" && recordedService != "" && recordedService != service {
		return "", ""
	}
	if conversationID != "" && recordedConversationID != "" && recordedConversationID != conversationID {
		return "", ""
	}
	return recordedURL, "workspace.latest"
}

func inferHealthURLFromText(rawText string) string {
	rawText = strings.TrimSpace(rawText)
	if rawText == "" {
		return ""
	}
	matches := healthURLFromTextPattern.FindAllString(rawText, -1)
	if len(matches) == 0 {
		return ""
	}
	cleaned := make([]string, 0, len(matches))
	for _, match := range matches {
		if value := sanitizeManagedHealthURL(match); value != "" {
			cleaned = append(cleaned, value)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}
	if len(cleaned) == 1 {
		return cleaned[0]
	}
	for _, candidate := range cleaned {
		lower := strings.ToLower(candidate)
		if strings.Contains(lower, "health") || strings.Contains(lower, "ready") {
			return candidate
		}
	}
	return cleaned[0]
}

func inferHealthURLFromLocalPortHint(rawText string) string {
	lower := strings.ToLower(strings.TrimSpace(rawText))
	if lower == "" {
		return ""
	}
	match := localhostPortFromTextPattern.FindStringSubmatch(lower)
	if len(match) < 2 {
		return ""
	}
	port := strings.TrimSpace(match[1])
	if port == "" {
		return ""
	}
	path := "/healthz"
	switch {
	case strings.Contains(lower, "/readyz"):
		path = "/readyz"
	case strings.Contains(lower, "/healthz"):
		path = "/healthz"
	case strings.Contains(lower, "/health"):
		path = "/health"
	case strings.Contains(lower, "ready"):
		path = "/readyz"
	}
	return fmt.Sprintf("http://127.0.0.1:%s%s", port, path)
}

func resolveManagedServiceHealthURLFromEnv(service string) string {
	service = strings.TrimSpace(strings.ToLower(service))
	if service == "" {
		return ""
	}
	if mapped := lookupManagedServiceHealthURLFromMapEnv(service); mapped != "" {
		return mapped
	}
	serviceKey := managedServiceEnvKeyToken(service)
	if serviceKey != "" {
		if value := sanitizeManagedHealthURL(os.Getenv("CLAWX_RUNTIME_HEALTH_URL_" + serviceKey)); value != "" {
			return value
		}
	}
	return sanitizeManagedHealthURL(os.Getenv("CLAWX_RUNTIME_HEALTH_URL_DEFAULT"))
}

func lookupManagedServiceHealthURLFromMapEnv(service string) string {
	raw := strings.TrimSpace(os.Getenv("CLAWX_RUNTIME_SERVICE_HEALTH_MAP"))
	if raw == "" {
		return ""
	}
	entries := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	defaultValue := ""
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := sanitizeManagedHealthURL(parts[1])
		if key == "" || value == "" {
			continue
		}
		if key == "default" {
			defaultValue = value
			continue
		}
		if strings.HasSuffix(key, "*") {
			prefix := strings.TrimSuffix(key, "*")
			if prefix != "" && strings.HasPrefix(service, prefix) {
				return value
			}
			continue
		}
		if key == service {
			return value
		}
	}
	return defaultValue
}

func managedServiceEnvKeyToken(service string) string {
	service = strings.TrimSpace(strings.ToUpper(service))
	if service == "" {
		return ""
	}
	var builder strings.Builder
	pendingUnderscore := false
	for _, r := range service {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			pendingUnderscore = false
			continue
		}
		if !pendingUnderscore {
			builder.WriteByte('_')
			pendingUnderscore = true
		}
	}
	return strings.Trim(builder.String(), "_")
}

func readTaskTrackingStringMap(snapshot map[string]interface{}, key string) map[string]string {
	if len(snapshot) == 0 {
		return nil
	}
	value, ok := snapshot[strings.TrimSpace(key)]
	if !ok {
		return nil
	}
	normalized := map[string]string{}
	switch typed := value.(type) {
	case map[string]string:
		for k, v := range typed {
			k = strings.TrimSpace(strings.ToLower(k))
			v = strings.TrimSpace(v)
			if k == "" || v == "" {
				continue
			}
			normalized[k] = v
		}
	case map[string]interface{}:
		for k, raw := range typed {
			k = strings.TrimSpace(strings.ToLower(k))
			v, ok := raw.(string)
			if !ok {
				continue
			}
			v = strings.TrimSpace(v)
			if k == "" || v == "" {
				continue
			}
			normalized[k] = v
		}
	default:
		return nil
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

type taskControlRouteHintRecord struct {
	Service        string
	HealthURL      string
	Operation      string
	UpdatedAt      string
	ConversationID string
}

func readTaskTrackingRouteHintMap(snapshot map[string]interface{}, key string) map[string]taskControlRouteHintRecord {
	if len(snapshot) == 0 {
		return nil
	}
	value, ok := snapshot[strings.TrimSpace(key)]
	if !ok {
		return nil
	}
	normalized := map[string]taskControlRouteHintRecord{}
	switch typed := value.(type) {
	case map[string]taskControlRouteHintRecord:
		for scope, record := range typed {
			scope = normalizeTaskControlRouteScopeKey(scope)
			record = normalizeTaskControlRouteHintRecord(record)
			if scope == "" || strings.TrimSpace(record.Service) == "" || strings.TrimSpace(record.HealthURL) == "" {
				continue
			}
			normalized[scope] = record
		}
	case map[string]interface{}:
		for scope, raw := range typed {
			scope = normalizeTaskControlRouteScopeKey(scope)
			record, ok := parseTaskControlRouteHintRecord(raw)
			if !ok || scope == "" {
				continue
			}
			normalized[scope] = record
		}
	default:
		return nil
	}
	if len(normalized) == 0 {
		return nil
	}
	return pruneTaskControlRouteHintMap(normalized, time.Now().UTC())
}

func parseTaskControlRouteHintRecord(raw interface{}) (taskControlRouteHintRecord, bool) {
	switch typed := raw.(type) {
	case taskControlRouteHintRecord:
		normalized := normalizeTaskControlRouteHintRecord(typed)
		if strings.TrimSpace(normalized.Service) == "" || strings.TrimSpace(normalized.HealthURL) == "" {
			return taskControlRouteHintRecord{}, false
		}
		return normalized, true
	case map[string]string:
		normalized := normalizeTaskControlRouteHintRecord(taskControlRouteHintRecord{
			Service:        typed["service"],
			HealthURL:      typed["health_url"],
			Operation:      typed["operation"],
			UpdatedAt:      typed["updated_at"],
			ConversationID: typed["conversation_id"],
		})
		if strings.TrimSpace(normalized.Service) == "" || strings.TrimSpace(normalized.HealthURL) == "" {
			return taskControlRouteHintRecord{}, false
		}
		return normalized, true
	case map[string]interface{}:
		normalized := normalizeTaskControlRouteHintRecord(taskControlRouteHintRecord{
			Service:        readInterfaceString(typed["service"]),
			HealthURL:      readInterfaceString(typed["health_url"]),
			Operation:      readInterfaceString(typed["operation"]),
			UpdatedAt:      readInterfaceString(typed["updated_at"]),
			ConversationID: readInterfaceString(typed["conversation_id"]),
		})
		if strings.TrimSpace(normalized.Service) == "" || strings.TrimSpace(normalized.HealthURL) == "" {
			return taskControlRouteHintRecord{}, false
		}
		return normalized, true
	default:
		return taskControlRouteHintRecord{}, false
	}
}

func encodeTaskTrackingRouteHintMap(hints map[string]taskControlRouteHintRecord) map[string]map[string]string {
	if len(hints) == 0 {
		return nil
	}
	encoded := map[string]map[string]string{}
	for scope, record := range hints {
		scope = normalizeTaskControlRouteScopeKey(scope)
		record = normalizeTaskControlRouteHintRecord(record)
		if scope == "" || strings.TrimSpace(record.Service) == "" || strings.TrimSpace(record.HealthURL) == "" {
			continue
		}
		item := map[string]string{
			"service":    strings.TrimSpace(record.Service),
			"health_url": strings.TrimSpace(record.HealthURL),
		}
		if strings.TrimSpace(record.Operation) != "" {
			item["operation"] = strings.TrimSpace(record.Operation)
		}
		if strings.TrimSpace(record.UpdatedAt) != "" {
			item["updated_at"] = strings.TrimSpace(record.UpdatedAt)
		}
		if strings.TrimSpace(record.ConversationID) != "" {
			item["conversation_id"] = strings.TrimSpace(record.ConversationID)
		}
		encoded[scope] = item
	}
	if len(encoded) == 0 {
		return nil
	}
	return encoded
}

type taskControlRouteHintPruneStats struct {
	InputEntries   int
	OutputEntries  int
	DroppedInvalid int
	DroppedExpired int
	DroppedTrimmed int
	TTL            time.Duration
	MaxEntries     int
}

func pruneTaskControlRouteHintMap(hints map[string]taskControlRouteHintRecord, now time.Time) map[string]taskControlRouteHintRecord {
	pruned, _ := pruneTaskControlRouteHintMapWithStats(hints, now)
	return pruned
}

func pruneTaskControlRouteHintMapWithStats(hints map[string]taskControlRouteHintRecord, now time.Time) (map[string]taskControlRouteHintRecord, taskControlRouteHintPruneStats) {
	stats := taskControlRouteHintPruneStats{
		InputEntries: len(hints),
		TTL:          resolveTaskControlHintTTL(),
		MaxEntries:   resolveTaskControlHintMaxEntries(),
	}
	if len(hints) == 0 {
		return nil, stats
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	type hintEntry struct {
		scope   string
		record  taskControlRouteHintRecord
		updated time.Time
	}
	entries := make([]hintEntry, 0, len(hints))
	for scope, record := range hints {
		scope = normalizeTaskControlRouteScopeKey(scope)
		record = normalizeTaskControlRouteHintRecord(record)
		if scope == "" || strings.TrimSpace(record.Service) == "" || strings.TrimSpace(record.HealthURL) == "" {
			stats.DroppedInvalid++
			continue
		}
		updatedAt, ok := parseTaskControlHintTime(record.UpdatedAt)
		if ok && stats.TTL > 0 && now.Sub(updatedAt) > stats.TTL {
			stats.DroppedExpired++
			continue
		}
		entries = append(entries, hintEntry{
			scope:   scope,
			record:  record,
			updated: updatedAt,
		})
	}
	if len(entries) == 0 {
		return nil, stats
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].updated.Equal(entries[j].updated) {
			return entries[i].scope < entries[j].scope
		}
		return entries[i].updated.After(entries[j].updated)
	})
	if stats.MaxEntries > 0 && len(entries) > stats.MaxEntries {
		stats.DroppedTrimmed = len(entries) - stats.MaxEntries
		entries = entries[:stats.MaxEntries]
	}
	stats.OutputEntries = len(entries)
	pruned := make(map[string]taskControlRouteHintRecord, len(entries))
	for _, entry := range entries {
		pruned[entry.scope] = entry.record
	}
	return pruned, stats
}

func resolveTaskControlHintTTL() time.Duration {
	raw := strings.TrimSpace(os.Getenv("CLAWX_RUNTIME_TASK_CONTROL_HINT_TTL"))
	if raw == "" {
		return defaultTaskControlHintTTL
	}
	ttl, err := time.ParseDuration(raw)
	if err != nil || ttl <= 0 {
		return defaultTaskControlHintTTL
	}
	return ttl
}

func resolveTaskControlHintMaxEntries() int {
	raw := strings.TrimSpace(os.Getenv("CLAWX_RUNTIME_TASK_CONTROL_HINT_MAX_ENTRIES"))
	if raw == "" {
		return defaultTaskControlHintMax
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultTaskControlHintMax
	}
	return value
}

func parseTaskControlHintTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func normalizeTaskControlRouteHintRecord(record taskControlRouteHintRecord) taskControlRouteHintRecord {
	record.Service = strings.TrimSpace(strings.ToLower(strings.TrimSuffix(record.Service, ".service")))
	record.HealthURL = sanitizeManagedHealthURL(record.HealthURL)
	record.Operation = strings.TrimSpace(strings.ToLower(record.Operation))
	record.UpdatedAt = strings.TrimSpace(record.UpdatedAt)
	record.ConversationID = strings.TrimSpace(record.ConversationID)
	return record
}

func normalizeTaskControlRouteScopeKey(raw string) string {
	return strings.TrimSpace(raw)
}

func inferRoutingScopeKeyFromScopedConversationID(conversationID string) string {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return ""
	}
	metaIndex := strings.Index(conversationID, "|ch=")
	if metaIndex < 0 {
		return ""
	}
	baseConversationID := strings.TrimSpace(conversationID[:metaIndex])
	if baseConversationID == "" {
		return ""
	}
	metadata := strings.TrimSpace(conversationID[metaIndex+1:])
	if metadata == "" {
		return ""
	}
	channel := ""
	instanceID := ""
	for _, token := range strings.Split(metadata, "|") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "ch=") {
			channel = strings.TrimSpace(strings.TrimPrefix(token, "ch="))
			continue
		}
		if strings.HasPrefix(token, "inst=") {
			instanceID = strings.TrimSpace(strings.TrimPrefix(token, "inst="))
		}
	}
	if channel == "" || instanceID == "" {
		return ""
	}
	return routingScopeKey(channel, instanceID, baseConversationID)
}

func readInterfaceString(raw interface{}) string {
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func sanitizeManagedHealthURL(raw string) string {
	value := strings.TrimSpace(strings.Trim(raw, "`'\"，,。.!！?？;；)）]】"))
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return ""
	}
	return value
}

func looksLikeTaskStatusQuery(normalized string) bool {
	if normalized == "" {
		return false
	}
	if looksLikeTaskDelegatesQuery(normalized) {
		return false
	}
	if containsAnyReleasePhrase(normalized,
		"请继续执行", "继续执行", "马上执行", "开始执行", "去执行", "帮我执行", "执行这个任务", "实现", "写代码", "开始开发",
	) {
		return false
	}
	hasTaskDomain := containsAnyReleasePhrase(normalized,
		"任务", "task", "执行", "进度", "goal", "步骤", "工作流", "autonomy",
	)
	hasStatusIntent := containsAnyReleasePhrase(normalized,
		"到哪里", "做到哪", "现在怎样", "状态", "情况", "进度", "当前", "最新", "status", "progress", "where",
	)
	if containsAnyReleasePhrase(normalized,
		"任务执行到哪里", "任务做到哪", "当前任务进度", "progress", "task status", "现在做到哪一步", "完成到哪了",
	) {
		return true
	}
	return hasTaskDomain && hasStatusIntent
}

func looksLikeTaskDelegatesQuery(normalized string) bool {
	if normalized == "" {
		return false
	}
	if containsAnyReleasePhrase(normalized,
		"重试子任务", "取消子任务", "创建子任务", "委派子任务", "委派给", "重试这个子任务", "取消这个子任务",
		"retry subtask", "cancel subtask", "delegate task", "assign task", "retry child task", "cancel child task",
	) {
		return false
	}
	hasDelegateDomain := containsAnyReleasePhrase(normalized,
		"子任务", "subtask", "sub-task", "child task", "child tasks", "delegate", "delegates", "delegated",
	)
	hasStatusIntent := containsAnyReleasePhrase(normalized,
		"状态", "进度", "查询", "查看", "看下", "汇总", "列表", "有哪些", "多少", "当前", "最新",
		"status", "progress", "list", "summary", "where", "show",
	)
	if containsAnyReleasePhrase(normalized,
		"查一下子任务状态", "子任务状态", "子任务进度", "子任务列表", "子任务执行到哪里",
		"delegates status", "delegate status", "child task status", "subtask status", "child task progress",
	) {
		return true
	}
	return hasDelegateDomain && hasStatusIntent
}

func looksLikeReleaseStatusQuery(normalized string) bool {
	if normalized == "" {
		return false
	}
	if containsAnyReleasePhrase(normalized,
		"怎么发布", "如何发布", "执行发布", "现在发布", "开始发布", "发布一下", "部署一下",
		"发布脚本", "部署脚本", "实现发布", "补齐发布", "升级发布流程",
	) {
		return false
	}
	hasReleaseDomain := containsAnyReleasePhrase(normalized,
		"发布", "版本", "上线", "部署", "release", "deploy", "rollback",
	)
	hasStatusIntent := containsAnyReleasePhrase(normalized,
		"状态", "进度", "当前", "现在", "最近", "最新", "记录", "历史", "history", "current", "status", "什么版本",
	)
	if containsAnyReleasePhrase(normalized,
		"当前版本", "现在版本", "最新版本", "最近发布", "发布历史", "release status", "deploy history", "回滚记录",
	) {
		return true
	}
	return hasReleaseDomain && hasStatusIntent
}

func containsAnyReleasePhrase(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword == "" {
			continue
		}
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func inferManagedReleaseServiceFromText(normalized string, runtime agentRuntime, runtimes map[string]agentRuntime, defaultAgentID string) string {
	if match := strings.TrimSpace(releaseServiceFromTextPattern.FindString(normalized)); match != "" {
		return strings.ToLower(strings.TrimSpace(match))
	}
	agentID := inferManagedReleaseAgentFromText(normalized, runtime, runtimes, defaultAgentID)
	if strings.TrimSpace(agentID) == "" || strings.EqualFold(strings.TrimSpace(agentID), "main") {
		return "clawx"
	}
	return "clawx-" + strings.ToLower(strings.TrimSpace(agentID))
}

func inferManagedReleaseAgentFromText(normalized string, runtime agentRuntime, runtimes map[string]agentRuntime, defaultAgentID string) string {
	normalized = strings.ToLower(strings.TrimSpace(normalized))
	if serviceMatch := strings.TrimSpace(releaseServiceFromTextPattern.FindString(normalized)); serviceMatch != "" {
		serviceID := strings.TrimSpace(strings.ToLower(strings.TrimSuffix(serviceMatch, ".service")))
		if serviceID == "clawx" {
			return "main"
		}
		if strings.HasPrefix(serviceID, "clawx-") {
			candidate := strings.TrimPrefix(serviceID, "clawx-")
			if _, ok := runtimes[candidate]; ok {
				return candidate
			}
			if strings.EqualFold(strings.TrimSpace(runtime.agentID), candidate) {
				return strings.TrimSpace(runtime.agentID)
			}
			if strings.EqualFold(strings.TrimSpace(defaultAgentID), candidate) {
				return strings.TrimSpace(defaultAgentID)
			}
		}
	}
	for agentID := range runtimes {
		if matchAgentTokenInReleaseText(normalized, agentID) {
			return strings.TrimSpace(agentID)
		}
	}
	if matchAgentTokenInReleaseText(normalized, runtime.agentID) {
		return strings.TrimSpace(runtime.agentID)
	}
	if matchAgentTokenInReleaseText(normalized, defaultAgentID) {
		return strings.TrimSpace(defaultAgentID)
	}
	if strings.TrimSpace(runtime.agentID) != "" {
		return strings.TrimSpace(runtime.agentID)
	}
	return strings.TrimSpace(defaultAgentID)
}

func matchAgentTokenInReleaseText(normalized string, agentID string) bool {
	agentID = strings.ToLower(strings.TrimSpace(agentID))
	if agentID == "" {
		return false
	}
	candidates := []string{
		agentID,
		strings.ReplaceAll(agentID, "-", " "),
		strings.ReplaceAll(agentID, "_", " "),
		strings.ReplaceAll(agentID, "-", ""),
		strings.ReplaceAll(agentID, "_", ""),
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if strings.Contains(normalized, candidate) {
			return true
		}
	}
	return false
}

func inferManagedReleaseHistoryLimitFromText(normalized string) int {
	normalized = strings.ToLower(strings.TrimSpace(normalized))
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`最近\s*([0-9]{1,2})\s*次`),
		regexp.MustCompile(`last\s*([0-9]{1,2})`),
		regexp.MustCompile(`([0-9]{1,2})\s*条`),
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(normalized)
		if len(match) < 2 {
			continue
		}
		value, err := strconv.Atoi(strings.TrimSpace(match[1]))
		if err != nil {
			continue
		}
		return normalizeManagedReleaseHistoryLimit(value)
	}
	return normalizeManagedReleaseHistoryLimit(5)
}

func normalizeManagedReleaseStatusOperation(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", "current":
		return "current", nil
	case "history":
		return "history", nil
	default:
		return "", fmt.Errorf("runtime.release.status operation 不支持: %q（仅支持 current|history）", raw)
	}
}

func normalizeManagedReleaseHistoryLimit(limit int) int {
	if limit <= 0 {
		return 5
	}
	if limit > 20 {
		return 20
	}
	return limit
}

func fallbackManagedReleaseValue(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func formatManagedReleaseCurrentSummary(service string, record managedReleaseRecord) string {
	return fmt.Sprintf("runtime.release.status：service=%s current version=%s status=%s release_id=%s finished_at=%s artifact_sha256=%s",
		service,
		fallbackManagedReleaseValue(record.Version, "-"),
		fallbackManagedReleaseValue(record.Status, "-"),
		fallbackManagedReleaseValue(record.ReleaseID, "-"),
		fallbackManagedReleaseValue(record.FinishedAt, "-"),
		fallbackManagedReleaseValue(record.ArtifactSHA256, "-"),
	)
}

func parseManagedReleaseHistory(raw []byte) ([]managedReleaseRecord, error) {
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	records := make([]managedReleaseRecord, 0, len(lines))
	for idx, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var record managedReleaseRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, fmt.Errorf("解析发布历史失败（line=%d）: %w", idx+1, err)
		}
		records = append(records, record)
	}
	return records, nil
}

func normalizeManagedReleaseVersion(rawVersion string, artifactPath string) string {
	version := strings.TrimSpace(rawVersion)
	if version != "" {
		return version
	}
	if strings.TrimSpace(artifactPath) != "" {
		return filepath.Base(strings.TrimSpace(artifactPath))
	}
	return "release-" + time.Now().UTC().Format("20060102T150405Z")
}

func buildManagedReleaseID(service string) string {
	return fmt.Sprintf("%s-%d-%d", service, time.Now().UTC().UnixNano(), os.Getpid())
}

func computeManagedFileSHA256(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()
	hasher := sha256.New()
	size, err := io.Copy(hasher, file)
	if err != nil {
		return "", 0, fmt.Errorf("读取文件失败: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), size, nil
}

func fillManagedReleaseRecordFromDeployOutput(record *managedReleaseRecord, output string) {
	if record == nil {
		return
	}
	for _, token := range strings.Fields(strings.ReplaceAll(output, "\n", " ")) {
		parts := strings.SplitN(token, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])
		if value == "" {
			continue
		}
		switch key {
		case "target":
			record.TargetBinary = value
		case "backup":
			record.BackupBinary = value
		case "source":
			record.SourceBinary = value
		}
	}
}

func managedReleaseStateDir() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("CLAWX_RUNTIME_RELEASE_STATE_DIR")); custom != "" {
		return filepath.Clean(custom), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("解析 HOME 失败: %w", err)
	}
	home = strings.TrimSpace(home)
	if home == "" {
		return "", fmt.Errorf("HOME 为空，无法写入发布元数据")
	}
	return filepath.Join(home, ".clawx", "releases"), nil
}

func persistManagedReleaseRecord(record managedReleaseRecord, updateCurrent bool) error {
	stateDir, err := managedReleaseStateDir()
	if err != nil {
		return err
	}
	serviceDir := filepath.Join(stateDir, record.Service)
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		return fmt.Errorf("创建发布目录失败: %w", err)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("序列化发布元数据失败: %w", err)
	}
	historyPath := filepath.Join(serviceDir, "history.jsonl")
	file, err := os.OpenFile(historyPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("打开发布历史失败: %w", err)
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		_ = file.Close()
		return fmt.Errorf("写入发布历史失败: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("关闭发布历史失败: %w", err)
	}
	if updateCurrent {
		currentPath := filepath.Join(serviceDir, "current.json")
		tmpPath := fmt.Sprintf("%s.tmp-%d", currentPath, os.Getpid())
		if err := os.WriteFile(tmpPath, encoded, 0o644); err != nil {
			return fmt.Errorf("写入当前发布元数据失败: %w", err)
		}
		if err := os.Rename(tmpPath, currentPath); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("更新当前发布元数据失败: %w", err)
		}
	}
	return nil
}

func resolveManagedScriptPath(runtime agentRuntime, actionCWD string, fallbackCWD string, rawPath string, fieldName string) (string, error) {
	pathValue := strings.TrimSpace(rawPath)
	if pathValue == "" {
		return "", fmt.Errorf("runtime.release 缺少 %s", fieldName)
	}

	baseCWD := resolveManagedActionBaseCWD(actionCWD, fallbackCWD, runtime.cwd)

	resolved := pathValue
	if !filepath.IsAbs(pathValue) {
		if baseCWD == "" {
			return "", fmt.Errorf("runtime.release 无法解析相对路径 %q：缺少可用 cwd", pathValue)
		}
		resolved = filepath.Join(baseCWD, pathValue)
	}
	resolved = filepath.Clean(resolved)

	if err := runtime.cfgSnapshot.ValidateWorkingDirectory(filepath.Dir(resolved)); err != nil {
		return "", fmt.Errorf("runtime.release %s 超出允许范围: %w", fieldName, err)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("runtime.release %s 不可访问: %w", fieldName, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("runtime.release %s 不能是目录: %s", fieldName, resolved)
	}
	if info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("runtime.release %s 不可执行: %s", fieldName, resolved)
	}
	return resolved, nil
}

func resolveManagedArtifactPath(runtime agentRuntime, actionCWD string, fallbackCWD string, rawPath string) (string, error) {
	pathValue := strings.TrimSpace(rawPath)
	if pathValue == "" {
		return "", fmt.Errorf("runtime.release 缺少 artifact_path")
	}

	baseCWD := resolveManagedActionBaseCWD(actionCWD, fallbackCWD, runtime.cwd)

	resolved := pathValue
	if !filepath.IsAbs(pathValue) {
		if baseCWD == "" {
			return "", fmt.Errorf("runtime.release 无法解析相对 artifact_path %q：缺少可用 cwd", pathValue)
		}
		resolved = filepath.Join(baseCWD, pathValue)
	}
	resolved = filepath.Clean(resolved)

	if err := runtime.cfgSnapshot.ValidateWorkingDirectory(filepath.Dir(resolved)); err != nil {
		return "", fmt.Errorf("runtime.release artifact_path 超出允许范围: %w", err)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("runtime.release artifact_path 不可访问: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("runtime.release artifact_path 不能是目录: %s", resolved)
	}
	return resolved, nil
}

func runManagedScript(ctx context.Context, timeout time.Duration, scriptPath string, extraEnv map[string]string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, scriptPath)
	cmd.Dir = filepath.Dir(scriptPath)
	if len(extraEnv) > 0 {
		env := os.Environ()
		for key, value := range extraEnv {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			env = append(env, key+"="+value)
		}
		cmd.Env = env
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("脚本执行超时（%s）: %s", timeout, scriptPath)
		}
		return "", fmt.Errorf("脚本执行失败: %s", strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func finalizeManagedReleaseFailureWithRollback(ctx context.Context, timeout time.Duration, rollbackPath string, service string, healthURL string, releaseErr error) (error, string, bool) {
	if strings.TrimSpace(rollbackPath) == "" {
		return releaseErr, "", false
	}
	rollbackNote, rollbackErr := attemptManagedReleaseRollback(ctx, timeout, rollbackPath, service, healthURL)
	if rollbackErr != nil {
		return fmt.Errorf("%v；回滚失败：%v", releaseErr, rollbackErr), "", false
	}
	return fmt.Errorf("%v；已自动回滚：%s", releaseErr, rollbackNote), rollbackNote, true
}

func attemptManagedReleaseRollback(ctx context.Context, timeout time.Duration, rollbackPath string, service string, healthURL string) (string, error) {
	rollbackOutput, err := runManagedScript(ctx, timeout, rollbackPath, nil)
	if err != nil {
		return "", err
	}
	if _, err := runManagedSystemctlUser(ctx, timeout, "restart", service+".service"); err != nil {
		return "", err
	}
	if strings.TrimSpace(healthURL) != "" {
		if err := waitManagedServiceHealthy(ctx, strings.TrimSpace(healthURL), timeout); err != nil {
			return "", err
		}
	}
	if strings.TrimSpace(rollbackOutput) == "" {
		return fmt.Sprintf("rollback_script=%s", rollbackPath), nil
	}
	return fmt.Sprintf("rollback_script=%s output=%s", rollbackPath, summarizeActionPlanCommandOutput(rollbackOutput)), nil
}

func managedReleaseRequiresArtifact() bool {
	return parseManagedBool(os.Getenv("CLAWX_RUNTIME_RELEASE_REQUIRE_ARTIFACT"))
}

func enforceManagedActionApproval(action actionPlanItem, actionKind string, service string) error {
	requireApproval := parseManagedBool(os.Getenv("CLAWX_RUNTIME_REQUIRE_APPROVAL")) || isManagedProductionEnv()
	expected := strings.TrimSpace(os.Getenv("CLAWX_RUNTIME_APPROVAL_TOKEN"))
	if !requireApproval && expected == "" {
		return nil
	}
	if expected == "" {
		return fmt.Errorf("%s 需要审批，但未配置 CLAWX_RUNTIME_APPROVAL_TOKEN（service=%s）", actionKind, service)
	}
	provided := strings.TrimSpace(action.ApprovalToken)
	if provided == "" {
		return fmt.Errorf("%s 需要 approval_token（service=%s）", actionKind, service)
	}
	if provided != expected {
		return fmt.Errorf("%s approval_token 无效（service=%s）", actionKind, service)
	}
	return nil
}

func isManagedProductionEnv() bool {
	env := strings.TrimSpace(strings.ToLower(os.Getenv("CLAWX_ENV")))
	return env == "prod" || env == "production"
}

func parseManagedBool(raw string) bool {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case "1", "true", "yes", "on", "y":
		return true
	default:
		return false
	}
}

func acquireManagedServiceLock(ctx context.Context, service string, timeout time.Duration) (*managedServiceLock, error) {
	wait := timeout
	if wait <= 0 {
		wait = defaultManagedServiceLockWait
	}
	lockDir := filepath.Join(os.TempDir(), "clawx-service-locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建服务锁目录失败: %w", err)
	}
	lockPath := filepath.Join(lockDir, service+".lock")
	deadline := time.Now().Add(wait)

	for {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = file.WriteString(fmt.Sprintf("pid=%d acquired_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339)))
			_ = file.Close()
			return &managedServiceLock{path: lockPath}, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("创建服务锁失败: %w", err)
		}
		stale, staleErr := isManagedServiceLockStale(lockPath)
		if staleErr == nil && stale {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("service %q 正在执行其他发布/重启任务，请稍后重试", service)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func isManagedServiceLockStale(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return time.Since(info.ModTime()) > staleManagedServiceLockDuration, nil
}

func (l *managedServiceLock) Release() {
	if l == nil || strings.TrimSpace(l.path) == "" {
		return
	}
	_ = os.Remove(l.path)
}
