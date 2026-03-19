package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"clawx/internal/application/configplan"
	"clawx/internal/infrastructure/config"
	chatiface "clawx/internal/interfaces/chat"
)

type parsedConfigCommand struct {
	Action      string
	Instruction string
}

var pendingConfigPlans configplan.Store = configplan.NewMemoryStore()

var englishCreateAgentPattern = regexp.MustCompile(`(?i)\b(create|add|new)\b.*\b(agent)\b`)
var commandLikeTaskPattern = regexp.MustCompile(`(?i)\b(go\s+run|go\s+test|git\s+\w+|npm\s+\w+|python\s+\S+|curl\s+|/agent\b|/project\b|/schedule\b|/memory\b)\b`)

type pendingConfigClarification struct {
	Action string
	Plan   configplan.Plan
	Patch  configplan.Patch
}

var configClarificationStore = struct {
	mu    sync.RWMutex
	items map[string]pendingConfigClarification
}{
	items: make(map[string]pendingConfigClarification),
}

var configStepMetrics = struct {
	mu        sync.Mutex
	active    map[string]int
	completed map[string]int
}{
	active:    make(map[string]int),
	completed: make(map[string]int),
}

func handleConfigChatCommand(message chatiface.Message) (bool, string, error) {
	cmd, isConfigCommand, err := parseConfigChatCommand(message.Text)
	if err != nil {
		return true, "", err
	}
	if !isConfigCommand {
		if response, handled := handlePendingConfigClarification(message); handled {
			return true, response, nil
		}
		if response, handled := tryStartMixedIntentClarification(message); handled {
			return true, response, nil
		}
		if isPatchIntentMessage(message.Text) {
			if _, ok := getPendingConfigPlan(message.ConversationID); !ok {
				return true, "当前没有待确认的配置计划，请先发送 `/config plan ...`。", nil
			}
		}
		if patchedResponse, patched, patchErr := tryPatchPendingPlanFromNaturalLanguage(message); patched || patchErr != nil {
			if patchErr != nil {
				return true, "", patchErr
			}
			return true, patchedResponse, nil
		}
		plan, matched, nlErr := tryBuildConfigPlanFromNaturalLanguage(message)
		if nlErr != nil {
			if looksLikeLowConfidenceConfigIntent(message.Text) {
				return true, configLowConfidenceSuggestion(), nil
			}
			return true, "", nlErr
		}
		if !matched {
			if looksLikeLowConfidenceConfigIntent(message.Text) {
				return true, configLowConfidenceSuggestion(), nil
			}
			return false, "", nil
		}
		if err := setPendingConfigPlan(message.ConversationID, plan); err != nil {
			return true, "", err
		}
		recordConfigAudit(configplan.AuditEvent{EventType: configplan.EventPlanCreated, ConversationScope: message.ConversationID, PlanID: auditPlanID(message.ConversationID, plan.Version), Actor: message.UserID, Source: plan.Source, PlanVersion: plan.Version, SummaryVersion: plan.SummaryVersion})
		recordConfigStepCreated(message.ConversationID)
		return true, fmt.Sprintf("已生成配置计划:\n- %s\n发送 `/config apply` 执行，或 `/config cancel` 取消。", plan.Summary), nil
	}

	if cmd.Action == "apply" && !isConfigAdmin(message.UserID) {
		recordConfigAudit(configplan.AuditEvent{EventType: configplan.EventPlanApplyRejected, ConversationScope: message.ConversationID, PlanID: auditPlanID(message.ConversationID, 0), Actor: message.UserID, Source: configplan.SourceSlash, DiffSummary: "permission_denied"})
		recordConfigStepProgress(message.ConversationID)
		return true, "你没有配置权限（可创建/编辑/查看计划，但不可 apply）。", nil
	}

	switch cmd.Action {
	case "help":
		return true, configHelpText(), nil
	case "show":
		plan, ok := getPendingConfigPlan(message.ConversationID)
		if !ok {
			return true, "当前没有待确认的配置计划。", nil
		}
		plan, updated, err := ensurePlanSummaryAvailable(message.ConversationID, plan)
		if err != nil {
			return true, "", err
		}
		if updated {
			recordConfigAudit(configplan.AuditEvent{
				EventType:            configplan.EventPlanPatched,
				ConversationScope:    message.ConversationID,
				PlanID:               auditPlanID(message.ConversationID, plan.Version),
				Actor:                message.UserID,
				Source:               plan.Source,
				PlanVersion:          plan.Version,
				SummaryVersion:       plan.SummaryVersion,
				SummaryRebuildReason: "show_rebuild",
				DiffSummary:          "summary_rebuild_on_show",
			})
		}
		recordConfigAudit(configplan.AuditEvent{EventType: configplan.EventPlanShown, ConversationScope: message.ConversationID, PlanID: auditPlanID(message.ConversationID, plan.Version), Actor: message.UserID, Source: plan.Source, PlanVersion: plan.Version, SummaryVersion: plan.SummaryVersion})
		recordConfigStepProgress(message.ConversationID)
		return true, fmt.Sprintf("待确认计划:\n- %s\n- 来源: %s\n- 版本: v%d\n- 创建时间: %s\n%s\n%s\n发送 `/config apply` 执行，或 `/config cancel` 取消。",
			plan.Summary,
			plan.Source,
			plan.Version,
			plan.CreatedAt.Format(time.RFC3339),
			formatControlPlaneSummary(plan),
			formatRecentPatchTrails(plan, 5),
		), nil
	case "cancel":
		plan, existed := popPendingConfigPlan(message.ConversationID)
		if existed {
			plan, rebuilt, reason := ensurePlanSummaryConsistency(plan, "cancel_precheck")
			recordConfigAudit(configplan.AuditEvent{EventType: configplan.EventPlanCanceled, ConversationScope: message.ConversationID, PlanID: auditPlanID(message.ConversationID, plan.Version), Actor: message.UserID, Source: plan.Source, PlanVersion: plan.Version, SummaryVersion: plan.SummaryVersion, SummaryRebuildReason: reasonForAudit(rebuilt, reason)})
			recordConfigStepCompleted(message.ConversationID, "canceled")
			return true, "已取消待确认的配置计划。", nil
		}
		return true, "当前没有待确认的配置计划。", nil
	case "apply":
		plan, ok := popPendingConfigPlan(message.ConversationID)
		if !ok {
			return true, "当前没有待确认的配置计划，请先发送 `/config plan ...`。", nil
		}
		plan, rebuilt, reason := ensurePlanSummaryConsistency(plan, "apply_precheck")
		response, err := applyPendingConfigPlan(plan)
		if err != nil {
			recordConfigAudit(configplan.AuditEvent{EventType: configplan.EventPlanApplyRejected, ConversationScope: message.ConversationID, PlanID: auditPlanID(message.ConversationID, plan.Version), Actor: message.UserID, Source: plan.Source, PlanVersion: plan.Version, SummaryVersion: plan.SummaryVersion, SummaryRebuildReason: reasonForAudit(rebuilt, reason), DiffSummary: err.Error()})
			return true, "", err
		}
		recordConfigAudit(configplan.AuditEvent{EventType: configplan.EventPlanApplied, ConversationScope: message.ConversationID, PlanID: auditPlanID(message.ConversationID, plan.Version), Actor: message.UserID, Source: plan.Source, PlanVersion: plan.Version, SummaryVersion: plan.SummaryVersion, SummaryRebuildReason: reasonForAudit(rebuilt, reason)})
		recordConfigStepCompleted(message.ConversationID, "applied")
		return true, response, nil
	case "plan":
		plan, err := buildConfigPlanFromInstruction(cmd.Instruction, message)
		if err != nil {
			return true, "", err
		}
		if err := setPendingConfigPlan(message.ConversationID, plan); err != nil {
			return true, "", err
		}
		recordConfigAudit(configplan.AuditEvent{EventType: configplan.EventPlanCreated, ConversationScope: message.ConversationID, PlanID: auditPlanID(message.ConversationID, plan.Version), Actor: message.UserID, Source: plan.Source, PlanVersion: plan.Version, SummaryVersion: plan.SummaryVersion})
		recordConfigStepCreated(message.ConversationID)
		return true, fmt.Sprintf("已生成配置计划:\n- %s\n发送 `/config apply` 执行，或 `/config cancel` 取消。", plan.Summary), nil
	default:
		return true, "", fmt.Errorf("不支持的 config 指令，请发送 `/config help` 查看用法")
	}
}

func parseConfigChatCommand(raw string) (parsedConfigCommand, bool, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return parsedConfigCommand{}, false, nil
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return parsedConfigCommand{}, false, nil
	}

	if normalizeConfigToken(fields[0]) != "config" {
		return parsedConfigCommand{}, false, nil
	}

	if len(fields) == 1 {
		return parsedConfigCommand{Action: "help"}, true, nil
	}

	action := strings.ToLower(strings.TrimSpace(fields[1]))
	switch action {
	case "help", "show", "apply", "cancel":
		return parsedConfigCommand{Action: action}, true, nil
	case "plan":
		instruction := strings.TrimSpace(strings.Join(fields[2:], " "))
		if instruction == "" {
			return parsedConfigCommand{}, true, fmt.Errorf("缺少 plan 指令内容。示例：`/config plan 创建 agent review 使用 claude`")
		}
		return parsedConfigCommand{Action: "plan", Instruction: instruction}, true, nil
	default:
		return parsedConfigCommand{}, true, fmt.Errorf("未知 config 子命令 %q，请发送 `/config help` 查看用法", action)
	}
}

func buildConfigPlanFromInstruction(instruction string, message chatiface.Message) (configplan.Plan, error) {
	trimmed := strings.TrimSpace(instruction)
	if trimmed == "" {
		return configplan.Plan{}, fmt.Errorf("空指令")
	}
	tokens := strings.Fields(trimmed)
	if len(tokens) == 0 {
		return configplan.Plan{}, fmt.Errorf("空指令")
	}

	lowered := make([]string, len(tokens))
	for i, token := range tokens {
		lowered[i] = strings.ToLower(strings.TrimSpace(token))
	}

	if isSetDefaultInstruction(lowered, trimmed) {
		agentID := parseSetDefaultAgentID(tokens, lowered)
		if agentID == "" {
			return configplan.Plan{}, fmt.Errorf("无法识别默认 agent。示例：`/config plan default agent review`")
		}
		agentID = strings.TrimSpace(agentID)
		plan := configplan.Plan{
			Kind:           configplan.KindSetDefaultAgent,
			ConversationID: message.ConversationID,
			CreatedBy:      message.UserID,
			CreatedAt:      time.Now().UTC(),
			Summary:        fmt.Sprintf("设置默认 Agent 为 `%s`", agentID),
			DefaultAgentID: agentID,
			Source:         configplan.SourceSlash,
			Version:        1,
		}
		return plan.Normalize(), plan.Validate()
	}

	if isDeleteAgentInstruction(lowered, trimmed) {
		opts, err := parseAgentDeleteOptions(tokens, lowered, trimmed)
		if err != nil {
			return configplan.Plan{}, err
		}
		plan := configplan.Plan{
			Kind:           configplan.KindDeleteAgent,
			ConversationID: message.ConversationID,
			CreatedBy:      message.UserID,
			CreatedAt:      time.Now().UTC(),
			Source:         configplan.SourceSlash,
			Version:        1,
			AgentDeleteOpts: &configplan.DeleteAgentOptions{
				ID:              opts.ID,
				DeleteWorkspace: opts.DeleteWorkspace,
			},
		}
		plan.Summary = fmt.Sprintf("删除 Agent `%s` (delete_workspace=%t)", opts.ID, opts.DeleteWorkspace)
		plan = plan.Normalize()
		return plan, plan.Validate()
	}

	if isRenameAgentInstruction(lowered, trimmed) {
		opts, err := parseAgentRenameOptions(tokens, lowered, trimmed)
		if err != nil {
			return configplan.Plan{}, err
		}
		targetWorkspace := strings.TrimSpace(opts.TargetWorkspace)
		plan := configplan.Plan{
			Kind:           configplan.KindRenameAgent,
			ConversationID: message.ConversationID,
			CreatedBy:      message.UserID,
			CreatedAt:      time.Now().UTC(),
			Source:         configplan.SourceSlash,
			Version:        1,
			AgentRenameOpts: &configplan.RenameAgentOptions{
				FromID:           opts.FromID,
				ToID:             opts.ToID,
				TargetWorkspace:  targetWorkspace,
				MigrateWorkspace: opts.MigrateWorkspace,
			},
		}
		plan.Summary = fmt.Sprintf("重命名 Agent `%s` -> `%s` (migrate_workspace=%t, workspace=`%s`)",
			opts.FromID, opts.ToID, opts.MigrateWorkspace, firstNonEmpty(targetWorkspace, "<auto>"))
		plan = plan.Normalize()
		return plan, plan.Validate()
	}

	if isCreateAgentInstruction(lowered, trimmed) {
		opts, err := parseAgentUpsertOptions(tokens, lowered)
		if err != nil {
			return configplan.Plan{}, err
		}
		resolvedWorkspace := firstNonEmpty(opts.Workspace, config.SuggestedAgentWorkspace(opts.ID))
		plan := configplan.Plan{
			Kind:           configplan.KindUpsertAgent,
			ConversationID: message.ConversationID,
			CreatedBy:      message.UserID,
			CreatedAt:      time.Now().UTC(),
			Source:         configplan.SourceSlash,
			Version:        1,
			AgentUpsertOpts: &configplan.UpsertAgentOptions{
				ID:             opts.ID,
				ProfileID:      opts.ProfileID,
				Workspace:      resolvedWorkspace,
				TimeoutSeconds: defaultInt(opts.TimeoutSeconds, 600),
				SetAsDefault:   opts.SetAsDefault,
			},
		}
		plan.Summary = fmt.Sprintf("新增/更新 Agent `%s` (profile=`%s`, workspace=`%s`, timeout=%ds, default=%t)",
			plan.AgentUpsertOpts.ID,
			plan.AgentUpsertOpts.ProfileID,
			plan.AgentUpsertOpts.Workspace,
			plan.AgentUpsertOpts.TimeoutSeconds,
			plan.AgentUpsertOpts.SetAsDefault,
		)
		plan = plan.Normalize()
		return plan, plan.Validate()
	}

	return configplan.Plan{}, fmt.Errorf("暂不支持该自然语言配置。当前支持：创建/更新 agent、切换默认 agent、删除 agent、重命名 agent。发送 `/config help` 查看示例")
}

func tryBuildConfigPlanFromNaturalLanguage(message chatiface.Message) (configplan.Plan, bool, error) {
	raw := strings.TrimSpace(message.Text)
	if raw == "" {
		return configplan.Plan{}, false, nil
	}
	lowerRaw := strings.ToLower(raw)
	if strings.HasPrefix(strings.TrimSpace(lowerRaw), "/") {
		return configplan.Plan{}, false, nil
	}
	if looksLikeDeleteAgentIntent(raw, lowerRaw) {
		opts, err := parseAgentDeleteOptionsFromNaturalLanguage(raw)
		if err != nil {
			return configplan.Plan{}, true, err
		}
		plan := configplan.Plan{
			Kind:           configplan.KindDeleteAgent,
			ConversationID: message.ConversationID,
			CreatedBy:      message.UserID,
			CreatedAt:      time.Now().UTC(),
			Source:         configplan.SourceNL,
			Version:        1,
			AgentDeleteOpts: &configplan.DeleteAgentOptions{
				ID:              opts.ID,
				DeleteWorkspace: opts.DeleteWorkspace,
			},
		}
		plan.Summary = fmt.Sprintf("删除 Agent `%s` (delete_workspace=%t)", opts.ID, opts.DeleteWorkspace)
		plan = plan.Normalize()
		return plan, true, plan.Validate()
	}
	if looksLikeRenameAgentIntent(raw, lowerRaw) {
		opts, err := parseAgentRenameOptionsFromNaturalLanguage(raw)
		if err != nil {
			return configplan.Plan{}, true, err
		}
		targetWorkspace := strings.TrimSpace(opts.TargetWorkspace)
		plan := configplan.Plan{
			Kind:           configplan.KindRenameAgent,
			ConversationID: message.ConversationID,
			CreatedBy:      message.UserID,
			CreatedAt:      time.Now().UTC(),
			Source:         configplan.SourceNL,
			Version:        1,
			AgentRenameOpts: &configplan.RenameAgentOptions{
				FromID:           opts.FromID,
				ToID:             opts.ToID,
				TargetWorkspace:  targetWorkspace,
				MigrateWorkspace: opts.MigrateWorkspace,
			},
		}
		plan.Summary = fmt.Sprintf("重命名 Agent `%s` -> `%s` (migrate_workspace=%t, workspace=`%s`)",
			opts.FromID, opts.ToID, opts.MigrateWorkspace, firstNonEmpty(targetWorkspace, "<auto>"))
		plan = plan.Normalize()
		return plan, true, plan.Validate()
	}
	if !looksLikeCreateAgentIntent(raw, lowerRaw) {
		return configplan.Plan{}, false, nil
	}
	opts, err := parseAgentUpsertOptionsFromNaturalLanguage(raw)
	if err != nil {
		return configplan.Plan{}, true, err
	}
	opts.ProfileID = firstNonEmpty(opts.ProfileID, "codex")
	opts.TimeoutSeconds = defaultInt(opts.TimeoutSeconds, 600)
	opts.Workspace = firstNonEmpty(opts.Workspace, config.SuggestedAgentWorkspace(opts.ID))

	plan := configplan.Plan{
		Kind:           configplan.KindUpsertAgent,
		ConversationID: message.ConversationID,
		CreatedBy:      message.UserID,
		CreatedAt:      time.Now().UTC(),
		Source:         configplan.SourceNL,
		Version:        1,
		AgentUpsertOpts: &configplan.UpsertAgentOptions{
			ID:             opts.ID,
			ProfileID:      opts.ProfileID,
			Workspace:      opts.Workspace,
			TimeoutSeconds: opts.TimeoutSeconds,
			SetAsDefault:   opts.SetAsDefault,
		},
	}
	plan.Summary = fmt.Sprintf("新增/更新 Agent `%s` (profile=`%s`, workspace=`%s`, timeout=%ds, default=%t)",
		opts.ID, opts.ProfileID, opts.Workspace, opts.TimeoutSeconds, opts.SetAsDefault)
	plan = plan.Normalize()
	return plan, true, plan.Validate()
}

func tryPatchPendingPlanFromNaturalLanguage(message chatiface.Message) (string, bool, error) {
	plan, ok := getPendingConfigPlan(message.ConversationID)
	if !ok {
		return "", false, nil
	}
	patch, matched, err := parsePatchFromNaturalLanguage(message.Text)
	if err != nil {
		return "", true, err
	}
	if !matched {
		return "", false, nil
	}
	patch.By = message.UserID
	patch.Source = configplan.SourceNL
	patch.At = time.Now().UTC()

	updated, err := configplan.ApplyPatch(plan, patch)
	if err != nil {
		return "", true, err
	}
	updated, rebuilt, reason := ensurePlanSummaryConsistency(updated, "patch_postcheck")
	if err := setPendingConfigPlan(message.ConversationID, updated); err != nil {
		return "", true, err
	}
	recordConfigAudit(configplan.AuditEvent{
		EventType:            configplan.EventPlanPatched,
		ConversationScope:    message.ConversationID,
		PlanID:               auditPlanID(message.ConversationID, updated.Version),
		Actor:                message.UserID,
		Source:               configplan.SourceNL,
		PlanVersion:          updated.Version,
		SummaryVersion:       updated.SummaryVersion,
		SummaryRebuildReason: reasonForAudit(rebuilt, reason),
		DiffSummary:          fmt.Sprintf("%s=%s", patch.Operation, strings.TrimSpace(patch.Value)),
	})
	recordConfigStepProgress(message.ConversationID)
	return fmt.Sprintf("已更新待确认计划（v%d）：\n- %s\n发送 `/config show` 查看详情，或 `/config apply` 执行。", updated.Version, updated.Summary), true, nil
}

func tryStartMixedIntentClarification(message chatiface.Message) (string, bool) {
	text := strings.TrimSpace(message.Text)
	if text == "" {
		return "", false
	}
	if strings.HasPrefix(strings.ToLower(text), "/") {
		return "", false
	}
	if !looksLikeTaskIntent(text) {
		return "", false
	}

	if plan, matched, err := tryBuildConfigPlanFromNaturalLanguage(message); matched && err == nil {
		setPendingConfigClarification(message.ConversationID, pendingConfigClarification{
			Action: "create",
			Plan:   plan,
		})
		return "这条消息同时包含配置和任务意图。请回复“配置”继续改配置，或回复“任务”走普通执行。", true
	}

	pendingPlan, ok := getPendingConfigPlan(message.ConversationID)
	if !ok {
		return "", false
	}
	patch, matched, err := parsePatchFromNaturalLanguage(text)
	if !matched || err != nil {
		return "", false
	}
	if pendingPlan.Kind != configplan.KindUpsertAgent {
		return "", false
	}
	setPendingConfigClarification(message.ConversationID, pendingConfigClarification{
		Action: "patch",
		Patch:  patch,
	})
	return "这条消息同时包含配置和任务意图。请回复“配置”继续改配置，或回复“任务”走普通执行。", true
}

func handlePendingConfigClarification(message chatiface.Message) (string, bool) {
	clarification, ok := getPendingConfigClarification(message.ConversationID)
	if !ok {
		return "", false
	}
	decision := parseClarificationDecision(message.Text)
	switch decision {
	case "config":
		deletePendingConfigClarification(message.ConversationID)
		switch clarification.Action {
		case "create":
			plan := clarification.Plan.Normalize()
			if err := setPendingConfigPlan(message.ConversationID, plan); err != nil {
				return fmt.Sprintf("配置计划创建失败：%v", err), true
			}
			recordConfigAudit(configplan.AuditEvent{EventType: configplan.EventPlanCreated, ConversationScope: message.ConversationID, PlanID: auditPlanID(message.ConversationID, plan.Version), Actor: message.UserID, Source: plan.Source, PlanVersion: plan.Version, SummaryVersion: plan.SummaryVersion})
			recordConfigStepCreated(message.ConversationID)
			return fmt.Sprintf("已生成配置计划:\n- %s\n发送 `/config apply` 执行，或 `/config cancel` 取消。", plan.Summary), true
		case "patch":
			plan, ok := getPendingConfigPlan(message.ConversationID)
			if !ok {
				return "当前没有待确认的配置计划。", true
			}
			patch := clarification.Patch
			patch.By = message.UserID
			patch.Source = configplan.SourceNL
			patch.At = time.Now().UTC()
			updated, err := configplan.ApplyPatch(plan, patch)
			if err != nil {
				return fmt.Sprintf("配置计划更新失败：%v", err), true
			}
			updated, rebuilt, reason := ensurePlanSummaryConsistency(updated, "clarification_patch_postcheck")
			if err := setPendingConfigPlan(message.ConversationID, updated); err != nil {
				return fmt.Sprintf("配置计划更新失败：%v", err), true
			}
			recordConfigAudit(configplan.AuditEvent{
				EventType:            configplan.EventPlanPatched,
				ConversationScope:    message.ConversationID,
				PlanID:               auditPlanID(message.ConversationID, updated.Version),
				Actor:                message.UserID,
				Source:               configplan.SourceNL,
				PlanVersion:          updated.Version,
				SummaryVersion:       updated.SummaryVersion,
				SummaryRebuildReason: reasonForAudit(rebuilt, reason),
				DiffSummary:          fmt.Sprintf("%s=%s", patch.Operation, strings.TrimSpace(patch.Value)),
			})
			recordConfigStepProgress(message.ConversationID)
			return fmt.Sprintf("已更新待确认计划（v%d）：\n- %s\n发送 `/config show` 查看详情，或 `/config apply` 执行。", updated.Version, updated.Summary), true
		default:
			return "澄清状态异常，已忽略本次确认。", true
		}
	case "task":
		deletePendingConfigClarification(message.ConversationID)
		return "已切换到任务通道，请重新发送任务内容。", true
	case "cancel":
		deletePendingConfigClarification(message.ConversationID)
		return "已取消本次配置/任务澄清。", true
	default:
		return "请回复“配置”或“任务”（也可回复“取消”）。", true
	}
}

func looksLikeTaskIntent(raw string) bool {
	text := strings.TrimSpace(raw)
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	if commandLikeTaskPattern.MatchString(text) {
		return true
	}
	if strings.Contains(text, "顺便") || strings.Contains(text, "同时") || strings.Contains(text, "然后") || strings.Contains(text, "并且") || strings.Contains(lower, " and ") {
		taskHints := []string{"执行", "跑一下", "run", "test", "pull", "查", "分析", "总结", "修复", "写", "生成"}
		for _, hint := range taskHints {
			if strings.Contains(lower, strings.ToLower(hint)) || strings.Contains(text, hint) {
				return true
			}
		}
	}
	return false
}

func looksLikeLowConfidenceConfigIntent(raw string) bool {
	text := strings.TrimSpace(raw)
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "/") {
		return false
	}
	hasConfigHint := strings.Contains(lower, "agent") || strings.Contains(text, "智能体") || strings.Contains(lower, "workspace") || strings.Contains(text, "工作目录") || strings.Contains(lower, "profile") || strings.Contains(lower, "timeout") || strings.Contains(text, "默认")
	if !hasConfigHint {
		return false
	}
	if looksLikeCreateAgentIntent(text, lower) {
		return false
	}
	if strings.Contains(lower, "改成") || strings.Contains(lower, "改为") || strings.Contains(lower, "设置") || strings.Contains(lower, "=") {
		return false
	}
	if strings.Contains(text, "?") || strings.Contains(text, "？") || strings.Contains(text, "能不能") || strings.Contains(text, "可不可以") {
		return true
	}
	if strings.Contains(text, "怎么") || strings.Contains(text, "如何") {
		return true
	}
	return false
}

func configLowConfidenceSuggestion() string {
	return "我理解你可能想改配置，但信息还不够明确。可直接发送：`/config plan 创建 agent <id> 使用 <profile>`，或明确说“把 workspace 改成 <path>”。"
}

func parseClarificationDecision(raw string) string {
	text := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case text == "配置" || text == "config" || text == "1" || strings.Contains(text, "改配置"):
		return "config"
	case text == "任务" || text == "task" || text == "2" || strings.Contains(text, "执行任务"):
		return "task"
	case text == "取消" || text == "cancel":
		return "cancel"
	default:
		return ""
	}
}

func parseAgentUpsertOptions(tokens, lowered []string) (configplan.UpsertAgentOptions, error) {
	var opts configplan.UpsertAgentOptions

	for idx := 0; idx < len(tokens); idx++ {
		switch lowered[idx] {
		case "agent":
			if idx+1 < len(tokens) {
				opts.ID = strings.TrimSpace(tokens[idx+1])
			}
		case "profile", "使用", "用":
			if idx+1 < len(tokens) {
				opts.ProfileID = strings.TrimSpace(tokens[idx+1])
			}
		case "workspace", "工作目录", "目录":
			if idx+1 < len(tokens) {
				opts.Workspace = strings.TrimSpace(tokens[idx+1])
			}
		case "timeout", "超时":
			if idx+1 < len(tokens) {
				parsed, err := strconv.Atoi(strings.TrimSpace(tokens[idx+1]))
				if err == nil && parsed > 0 {
					opts.TimeoutSeconds = parsed
				}
			}
		case "default", "默认":
			opts.SetAsDefault = true
		}
	}

	if strings.TrimSpace(opts.ID) == "" {
		return configplan.UpsertAgentOptions{}, fmt.Errorf("无法识别 agent id。示例：`/config plan 创建 agent review 使用 claude`")
	}
	if strings.TrimSpace(opts.ProfileID) == "" {
		opts.ProfileID = "codex"
	}
	if opts.TimeoutSeconds <= 0 {
		opts.TimeoutSeconds = 600
	}
	return opts, nil
}

func parseAgentDeleteOptions(tokens, lowered []string, raw string) (configplan.DeleteAgentOptions, error) {
	var opts configplan.DeleteAgentOptions
	for idx := 0; idx < len(tokens); idx++ {
		switch lowered[idx] {
		case "agent":
			if idx+1 < len(tokens) {
				opts.ID = strings.TrimSpace(tokens[idx+1])
			}
		}
	}
	if strings.TrimSpace(opts.ID) == "" {
		opts.ID = extractAgentIDFromNaturalLanguage(tokens)
	}
	if strings.TrimSpace(opts.ID) == "" {
		return configplan.DeleteAgentOptions{}, fmt.Errorf("无法识别 agent id。示例：`/config plan 删除 agent review`")
	}

	lowerRaw := strings.ToLower(strings.TrimSpace(raw))
	if strings.Contains(raw, "删除目录") || strings.Contains(raw, "删除工作目录") || strings.Contains(raw, "包含目录") || strings.Contains(raw, "包括目录") ||
		strings.Contains(lowerRaw, "delete workspace") || strings.Contains(lowerRaw, "with workspace") || strings.Contains(lowerRaw, "remove workspace") {
		opts.DeleteWorkspace = true
	}
	return opts, nil
}

func parseAgentDeleteOptionsFromNaturalLanguage(raw string) (configplan.DeleteAgentOptions, error) {
	tokens := strings.Fields(strings.TrimSpace(raw))
	lowered := make([]string, len(tokens))
	for i, token := range tokens {
		lowered[i] = strings.ToLower(strings.TrimSpace(token))
	}
	return parseAgentDeleteOptions(tokens, lowered, raw)
}

func parseAgentRenameOptions(tokens, lowered []string, raw string) (configplan.RenameAgentOptions, error) {
	var opts configplan.RenameAgentOptions

	for idx := 0; idx < len(tokens); idx++ {
		switch lowered[idx] {
		case "agent":
			if idx+1 < len(tokens) {
				opts.FromID = strings.TrimSpace(tokens[idx+1])
			}
		case "workspace", "工作目录", "目录":
			if idx+1 < len(tokens) {
				opts.TargetWorkspace = strings.TrimSpace(tokens[idx+1])
			}
		case "to", "为", "改成", "改为":
			if idx+1 < len(tokens) {
				opts.ToID = strings.TrimSpace(tokens[idx+1])
			}
		}
	}

	trimmed := strings.TrimSpace(raw)
	if strings.Contains(trimmed, "->") {
		parts := strings.SplitN(trimmed, "->", 2)
		if len(parts) == 2 {
			right := sanitizeAgentIDToken(firstToken(parts[1]))
			if right != "" {
				opts.ToID = right
			}
		}
	}
	if opts.ToID == "" {
		opts.ToID = sanitizeAgentIDToken(extractPatchValue(raw, "为", "to", "改成", "改为", "叫"))
	}
	if opts.FromID == "" {
		opts.FromID = extractAgentIDFromNaturalLanguage(tokens)
	}
	opts.FromID = sanitizeAgentIDToken(opts.FromID)
	opts.ToID = sanitizeAgentIDToken(opts.ToID)
	if opts.FromID == "" || opts.ToID == "" {
		return configplan.RenameAgentOptions{}, fmt.Errorf("无法识别重命名 agent。示例：`/config plan 重命名 agent old 为 new`")
	}
	if opts.FromID == opts.ToID {
		return configplan.RenameAgentOptions{}, fmt.Errorf("重命名前后 agent id 不能相同")
	}

	lowerRaw := strings.ToLower(trimmed)
	opts.MigrateWorkspace = true
	if strings.Contains(trimmed, "不迁移目录") || strings.Contains(trimmed, "仅改名") || strings.Contains(lowerRaw, "without workspace") || strings.Contains(lowerRaw, "no workspace migrate") {
		opts.MigrateWorkspace = false
	}
	if opts.TargetWorkspace != "" {
		opts.MigrateWorkspace = true
	}
	return opts, nil
}

func parseAgentRenameOptionsFromNaturalLanguage(raw string) (configplan.RenameAgentOptions, error) {
	tokens := strings.Fields(strings.TrimSpace(raw))
	lowered := make([]string, len(tokens))
	for i, token := range tokens {
		lowered[i] = strings.ToLower(strings.TrimSpace(token))
	}
	return parseAgentRenameOptions(tokens, lowered, raw)
}

func parseAgentUpsertOptionsFromNaturalLanguage(raw string) (configplan.UpsertAgentOptions, error) {
	tokens := strings.Fields(strings.TrimSpace(raw))
	lowered := make([]string, len(tokens))
	for i, token := range tokens {
		lowered[i] = strings.ToLower(strings.TrimSpace(token))
	}
	opts, _ := parseAgentUpsertOptions(tokens, lowered)
	if strings.TrimSpace(opts.ID) == "" {
		opts.ID = extractAgentIDFromNaturalLanguage(tokens)
	}
	if strings.TrimSpace(opts.ID) == "" {
		return configplan.UpsertAgentOptions{}, fmt.Errorf("无法识别 agent id。示例：创建 agent bid-all")
	}
	if strings.TrimSpace(opts.ProfileID) == "" {
		switch {
		case strings.Contains(strings.ToLower(raw), "claude"):
			opts.ProfileID = "claude"
		default:
			opts.ProfileID = "codex"
		}
	}
	return opts, nil
}

func parsePatchFromNaturalLanguage(raw string) (configplan.Patch, bool, error) {
	text := strings.TrimSpace(raw)
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "/") {
		return configplan.Patch{}, false, nil
	}
	if looksLikeAgentIDCorrectionIntent(text, lower) {
		value := extractAgentIDPatchValue(text)
		if value == "" {
			return configplan.Patch{}, true, fmt.Errorf("无法识别 agent id 值")
		}
		return configplan.Patch{Operation: configplan.PatchSetAgentID, Value: value}, true, nil
	}
	if strings.Contains(lower, "workspace") || strings.Contains(text, "工作目录") || strings.Contains(text, "目录") {
		value := extractPatchValue(text, "workspace", "工作目录", "目录")
		if value == "" {
			return configplan.Patch{}, true, fmt.Errorf("无法识别 workspace 值")
		}
		return configplan.Patch{Operation: configplan.PatchSetWorkspace, Value: value}, true, nil
	}
	if strings.Contains(lower, "timeout") || strings.Contains(text, "超时") {
		value := extractPatchValue(text, "timeout", "超时")
		if value == "" {
			if digits := extractDigits(text); digits != "" {
				value = digits
			}
		}
		if value == "" {
			return configplan.Patch{}, true, fmt.Errorf("无法识别 timeout 值")
		}
		return configplan.Patch{Operation: configplan.PatchSetTimeout, Value: value}, true, nil
	}
	if strings.Contains(lower, "profile") || strings.Contains(text, "使用") || strings.Contains(text, "用") {
		value := extractPatchValue(text, "profile", "使用", "用")
		if value == "" {
			if strings.Contains(lower, "claude") {
				value = "claude"
			} else if strings.Contains(lower, "codex") {
				value = "codex"
			}
		}
		if value == "" {
			return configplan.Patch{}, true, fmt.Errorf("无法识别 profile 值")
		}
		return configplan.Patch{Operation: configplan.PatchSetProfile, Value: value}, true, nil
	}
	if strings.Contains(lower, "default") || strings.Contains(text, "默认") {
		value := "true"
		if strings.Contains(lower, "false") || strings.Contains(text, "不默认") {
			value = "false"
		}
		return configplan.Patch{Operation: configplan.PatchSetDefault, Value: value}, true, nil
	}
	return configplan.Patch{}, false, nil
}

func looksLikeAgentIDCorrectionIntent(text, lower string) bool {
	if strings.Contains(lower, "agent id") || strings.Contains(lower, "agent") && (strings.Contains(lower, "改成") || strings.Contains(lower, "改为") || strings.Contains(lower, "should be")) {
		return true
	}
	if strings.Contains(text, "智能体") && (strings.Contains(text, "应该是") || strings.Contains(text, "不是") || strings.Contains(text, "改成") || strings.Contains(text, "改为") || strings.Contains(text, "叫")) {
		return true
	}
	if strings.Contains(text, "项目应该是") || strings.Contains(text, "不是") && strings.Contains(text, "是") {
		return true
	}
	return false
}

func extractAgentIDPatchValue(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}

	captureCandidates := []string{
		extractPatchValue(trimmed, "应该是", "改成", "改为", "叫", "id", "ID"),
	}
	for _, candidate := range captureCandidates {
		value := normalizeAgentIDCorrectionCandidate(candidate)
		if strings.Contains(trimmed, "而不是") || strings.Contains(trimmed, "不是") {
			value = strings.TrimSuffix(value, "智能体")
		}
		if value != "" {
			return value
		}
	}

	agentPattern := regexp.MustCompile(`(?i)\bagent\s+([A-Za-z0-9._-]+)`)
	if m := agentPattern.FindStringSubmatch(trimmed); len(m) == 2 {
		value := sanitizeAgentIDToken(m[1])
		if value != "" {
			return value
		}
	}

	if idx := strings.LastIndex(trimmed, "是"); idx >= 0 && idx+len("是") < len(trimmed) {
		tail := strings.TrimSpace(trimmed[idx+len("是"):])
		tail = strings.Trim(tail, "，。:：,`\"")
		value := normalizeAgentIDCorrectionCandidate(firstToken(tail))
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeAgentIDCorrectionCandidate(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	for _, sep := range []string{"而不是", "不是", "，", ",", "。"} {
		if idx := strings.Index(value, sep); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
	}
	return sanitizeAgentIDToken(value)
}

func firstToken(text string) string {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func extractPatchValue(text string, markers ...string) string {
	normalized := strings.TrimSpace(text)
	for _, marker := range markers {
		idx := strings.Index(strings.ToLower(normalized), strings.ToLower(marker))
		if idx < 0 {
			continue
		}
		tail := strings.TrimSpace(normalized[idx+len(marker):])
		tail = strings.TrimPrefix(tail, "是")
		tail = strings.TrimPrefix(tail, "为")
		tail = strings.TrimPrefix(tail, "改成")
		tail = strings.TrimPrefix(tail, "改为")
		tail = strings.TrimPrefix(tail, "=")
		tail = strings.TrimPrefix(tail, ":")
		tail = strings.TrimSpace(tail)
		tail = strings.Trim(tail, "，。`\"")
		if tail != "" {
			fields := strings.Fields(tail)
			if len(fields) > 0 {
				return strings.TrimSpace(fields[0])
			}
		}
	}
	return ""
}

func extractDigits(text string) string {
	var b strings.Builder
	for _, r := range text {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func looksLikeCreateAgentIntent(raw, lowerRaw string) bool {
	if englishCreateAgentPattern.MatchString(raw) {
		return true
	}
	hasCreateVerb := strings.Contains(raw, "创建") || strings.Contains(raw, "新建") || strings.Contains(raw, "新增")
	hasAgentWord := strings.Contains(lowerRaw, "agent") || strings.Contains(raw, "智能体")
	return hasCreateVerb && hasAgentWord
}

func looksLikeDeleteAgentIntent(raw, lowerRaw string) bool {
	hasDeleteVerb := strings.Contains(raw, "删除") || strings.Contains(lowerRaw, "delete") || strings.Contains(lowerRaw, "remove")
	hasAgentWord := strings.Contains(lowerRaw, "agent") || strings.Contains(raw, "智能体")
	return hasDeleteVerb && hasAgentWord
}

func looksLikeRenameAgentIntent(raw, lowerRaw string) bool {
	hasRenameVerb := strings.Contains(raw, "重命名") || strings.Contains(raw, "改名") || strings.Contains(lowerRaw, "rename")
	hasAgentWord := strings.Contains(lowerRaw, "agent") || strings.Contains(raw, "智能体")
	return hasRenameVerb && hasAgentWord
}

func extractAgentIDFromNaturalLanguage(tokens []string) string {
	for i, token := range tokens {
		trimmed := strings.Trim(strings.TrimSpace(token), "，。:：,")
		lowered := strings.ToLower(trimmed)
		if lowered == "agent" && i+1 < len(tokens) {
			candidate := sanitizeAgentIDToken(tokens[i+1])
			if candidate != "" {
				return candidate
			}
		}
		if strings.HasSuffix(trimmed, "智能体") {
			candidate := sanitizeAgentIDToken(strings.TrimSuffix(trimmed, "智能体"))
			if candidate != "" {
				return candidate
			}
		}
		if trimmed == "智能体" && i > 0 {
			candidate := sanitizeAgentIDToken(tokens[i-1])
			if candidate != "" {
				return candidate
			}
		}
	}
	return ""
}

func sanitizeAgentIDToken(raw string) string {
	value := strings.Trim(strings.TrimSpace(raw), "，。:：,`\"")
	value = strings.TrimPrefix(value, "为")
	value = strings.TrimPrefix(value, "叫")
	value = strings.Trim(value, "，。:：,`\"")
	if value == "" {
		return ""
	}
	if strings.EqualFold(value, "agent") || value == "智能体" {
		return ""
	}
	return value
}

func parseSetDefaultAgentID(tokens, lowered []string) string {
	for idx := 0; idx < len(tokens)-1; idx++ {
		if lowered[idx] == "agent" {
			return strings.TrimSpace(tokens[idx+1])
		}
	}
	if len(tokens) > 0 {
		return strings.TrimSpace(tokens[len(tokens)-1])
	}
	return ""
}

func isCreateAgentInstruction(lowered []string, raw string) bool {
	if strings.Contains(raw, "创建") || strings.Contains(raw, "新增") {
		return strings.Contains(strings.ToLower(raw), "agent")
	}
	if len(lowered) == 0 {
		return false
	}
	first := lowered[0]
	switch first {
	case "add", "create", "upsert":
		for _, token := range lowered {
			if token == "agent" {
				return true
			}
		}
	}
	return false
}

func isDeleteAgentInstruction(lowered []string, raw string) bool {
	lcRaw := strings.ToLower(raw)
	if strings.Contains(raw, "删除") && (strings.Contains(lcRaw, "agent") || strings.Contains(raw, "智能体")) {
		return true
	}
	if len(lowered) == 0 {
		return false
	}
	return lowered[0] == "delete" || lowered[0] == "remove"
}

func isRenameAgentInstruction(lowered []string, raw string) bool {
	lcRaw := strings.ToLower(raw)
	if (strings.Contains(raw, "重命名") || strings.Contains(raw, "改名")) && (strings.Contains(lcRaw, "agent") || strings.Contains(raw, "智能体")) {
		return true
	}
	if len(lowered) == 0 {
		return false
	}
	return lowered[0] == "rename"
}

func isSetDefaultInstruction(lowered []string, raw string) bool {
	lcRaw := strings.ToLower(raw)
	if strings.Contains(raw, "默认agent") || strings.Contains(raw, "切换默认") {
		return true
	}
	if strings.HasPrefix(lcRaw, "default agent") || strings.HasPrefix(lcRaw, "set default agent") {
		return true
	}
	if len(lowered) == 0 {
		return false
	}
	return lowered[0] == "default"
}

func applyPendingConfigPlan(plan configplan.Plan) (string, error) {
	switch plan.Kind {
	case configplan.KindUpsertAgent:
		if plan.AgentUpsertOpts == nil {
			return "", fmt.Errorf("缺少 agent 配置")
		}
		workspace := strings.TrimSpace(plan.AgentUpsertOpts.Workspace)
		if workspace != "" {
			if err := ensureWorkspacePath(workspace); err != nil {
				return "", err
			}
		}
		path, err := config.UpsertAgent(config.AgentUpsertOptions{
			ID:             plan.AgentUpsertOpts.ID,
			ProfileID:      plan.AgentUpsertOpts.ProfileID,
			Workspace:      plan.AgentUpsertOpts.Workspace,
			TimeoutSeconds: plan.AgentUpsertOpts.TimeoutSeconds,
			SetAsDefault:   plan.AgentUpsertOpts.SetAsDefault,
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("配置已写入：%s\n- %s\n- workspace 已就绪：%s", path, plan.Summary, firstNonEmpty(workspace, plan.AgentUpsertOpts.Workspace)), nil
	case configplan.KindSetDefaultAgent:
		path, err := config.SetDefaultAgent(plan.DefaultAgentID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("配置已写入：%s\n- %s", path, plan.Summary), nil
	case configplan.KindDeleteAgent:
		if plan.AgentDeleteOpts == nil {
			return "", fmt.Errorf("缺少 delete-agent 配置")
		}
		path, workspace, err := config.DeleteAgent(config.AgentDeleteOptions{
			ID: plan.AgentDeleteOpts.ID,
		})
		if err != nil {
			return "", err
		}
		workspaceNote := "workspace 未删除"
		if plan.AgentDeleteOpts.DeleteWorkspace {
			if err := deleteWorkspaceDirectory(workspace); err != nil {
				return "", err
			}
			workspaceNote = fmt.Sprintf("workspace 已删除：%s", workspace)
		}
		return fmt.Sprintf("配置已写入：%s\n- %s\n- %s", path, plan.Summary, workspaceNote), nil
	case configplan.KindRenameAgent:
		if plan.AgentRenameOpts == nil {
			return "", fmt.Errorf("缺少 rename-agent 配置")
		}
		path, oldWorkspace, newWorkspace, err := config.RenameAgent(config.AgentRenameOptions{
			FromID:           plan.AgentRenameOpts.FromID,
			ToID:             plan.AgentRenameOpts.ToID,
			Workspace:        plan.AgentRenameOpts.TargetWorkspace,
			MigrateWorkspace: plan.AgentRenameOpts.MigrateWorkspace,
		})
		if err != nil {
			return "", err
		}
		if err := migrateWorkspaceDirectory(oldWorkspace, newWorkspace, plan.AgentRenameOpts.MigrateWorkspace); err != nil {
			return "", err
		}
		return fmt.Sprintf("配置已写入：%s\n- %s\n- workspace：%s -> %s", path, plan.Summary, oldWorkspace, newWorkspace), nil
	default:
		return "", fmt.Errorf("未知配置计划类型")
	}
}

func deleteWorkspaceDirectory(workspace string) error {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return nil
	}
	clean := filepath.Clean(workspace)
	if clean == "." || clean == string(os.PathSeparator) {
		return fmt.Errorf("拒绝删除不安全目录: %s", clean)
	}
	info, err := os.Stat(clean)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("检查 workspace 失败: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace %q 不是目录", clean)
	}
	if err := os.RemoveAll(clean); err != nil {
		return fmt.Errorf("删除 workspace %q 失败: %w", clean, err)
	}
	return nil
}

func migrateWorkspaceDirectory(oldWorkspace, newWorkspace string, enabled bool) error {
	if !enabled {
		return nil
	}
	oldWorkspace = strings.TrimSpace(oldWorkspace)
	newWorkspace = strings.TrimSpace(newWorkspace)
	if newWorkspace == "" {
		return fmt.Errorf("迁移 workspace 失败：目标目录为空")
	}
	if oldWorkspace == "" || oldWorkspace == newWorkspace {
		return ensureWorkspacePath(newWorkspace)
	}
	oldInfo, err := os.Stat(oldWorkspace)
	if err != nil {
		if os.IsNotExist(err) {
			return ensureWorkspacePath(newWorkspace)
		}
		return fmt.Errorf("检查旧 workspace 失败: %w", err)
	}
	if !oldInfo.IsDir() {
		return fmt.Errorf("旧 workspace %q 不是目录", oldWorkspace)
	}

	newExists := false
	if info, statErr := os.Stat(newWorkspace); statErr == nil {
		if !info.IsDir() {
			return fmt.Errorf("目标 workspace %q 不是目录", newWorkspace)
		}
		newExists = true
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("检查目标 workspace 失败: %w", statErr)
	}
	if newExists {
		empty, emptyErr := directoryEmptyOrMissing(newWorkspace)
		if emptyErr != nil {
			return fmt.Errorf("检查目标 workspace 是否为空失败: %w", emptyErr)
		}
		if !empty {
			return fmt.Errorf("目标 workspace %q 已存在且非空，拒绝覆盖", newWorkspace)
		}
		if err := os.Remove(newWorkspace); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("清理空目录失败: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(newWorkspace), 0o755); err != nil {
		return fmt.Errorf("创建目标父目录失败: %w", err)
	}
	if err := os.Rename(oldWorkspace, newWorkspace); err != nil {
		return fmt.Errorf("迁移 workspace 失败: %w", err)
	}
	return ensureWorkspacePath(newWorkspace)
}

func isConfigAdmin(userID string) bool {
	allowed := strings.TrimSpace(os.Getenv("CLAWX_CONFIG_ADMIN_USERS"))
	if allowed == "" {
		return true
	}
	for _, item := range strings.Split(allowed, ",") {
		if strings.TrimSpace(item) == strings.TrimSpace(userID) {
			return true
		}
	}
	return false
}

func configHelpText() string {
	return strings.TrimSpace(`
配置指令：
- /config plan 创建 agent review 使用 claude
- /config plan add agent review profile claude workspace /path/to/project timeout 900 default
- /config plan default agent review
- /config plan 删除 agent review [删除目录]
- /config plan 重命名 agent review 为 bid-all [迁移目录]
- /config show
- /config apply
- /config cancel

说明：
- plan 只生成待确认变更，不会立即写文件
- 可用自然语言继续编辑当前计划（例如：把 workspace 改成 /path）
- apply 才会写入 config.json
- 非管理员可创建/编辑/查看/取消计划，但不可 apply
- 删除/重命名可选择是否同步处理 workspace 目录`)
}

func setPendingConfigPlan(conversationID string, plan configplan.Plan) error {
	plan = plan.Normalize()
	if configplan.ShouldRebuildSummary(plan) {
		plan = configplan.RebuildSummary(plan, "set_pending")
	}
	return pendingConfigPlans.Set(strings.TrimSpace(conversationID), plan)
}

func getPendingConfigPlan(conversationID string) (configplan.Plan, bool) {
	return pendingConfigPlans.Get(strings.TrimSpace(conversationID))
}

func popPendingConfigPlan(conversationID string) (configplan.Plan, bool) {
	return pendingConfigPlans.Pop(strings.TrimSpace(conversationID))
}

func deletePendingConfigPlan(conversationID string) bool {
	return pendingConfigPlans.Delete(strings.TrimSpace(conversationID))
}

func setPendingConfigClarification(conversationID string, state pendingConfigClarification) {
	key := strings.TrimSpace(conversationID)
	if key == "" {
		return
	}
	configClarificationStore.mu.Lock()
	defer configClarificationStore.mu.Unlock()
	configClarificationStore.items[key] = state
}

func getPendingConfigClarification(conversationID string) (pendingConfigClarification, bool) {
	key := strings.TrimSpace(conversationID)
	if key == "" {
		return pendingConfigClarification{}, false
	}
	configClarificationStore.mu.RLock()
	defer configClarificationStore.mu.RUnlock()
	state, ok := configClarificationStore.items[key]
	return state, ok
}

func deletePendingConfigClarification(conversationID string) bool {
	key := strings.TrimSpace(conversationID)
	if key == "" {
		return false
	}
	configClarificationStore.mu.Lock()
	defer configClarificationStore.mu.Unlock()
	if _, ok := configClarificationStore.items[key]; ok {
		delete(configClarificationStore.items, key)
		return true
	}
	return false
}

func recordConfigStepCreated(conversationID string) {
	key := strings.TrimSpace(conversationID)
	if key == "" {
		return
	}
	configStepMetrics.mu.Lock()
	defer configStepMetrics.mu.Unlock()
	configStepMetrics.active[key] = 1
}

func recordConfigStepProgress(conversationID string) {
	key := strings.TrimSpace(conversationID)
	if key == "" {
		return
	}
	configStepMetrics.mu.Lock()
	defer configStepMetrics.mu.Unlock()
	if current, ok := configStepMetrics.active[key]; ok && current > 0 {
		configStepMetrics.active[key] = current + 1
	}
}

func recordConfigStepCompleted(conversationID, outcome string) {
	key := strings.TrimSpace(conversationID)
	if key == "" {
		return
	}
	configStepMetrics.mu.Lock()
	defer configStepMetrics.mu.Unlock()
	if current, ok := configStepMetrics.active[key]; ok && current > 0 {
		total := current + 1
		configStepMetrics.completed[key] = total
		delete(configStepMetrics.active, key)
		log.Printf("config_metrics conversation=%s outcome=%s interaction_steps=%d", key, strings.TrimSpace(outcome), total)
	}
}

func getLastCompletedConfigStepCountForTests(conversationID string) int {
	key := strings.TrimSpace(conversationID)
	if key == "" {
		return 0
	}
	configStepMetrics.mu.Lock()
	defer configStepMetrics.mu.Unlock()
	return configStepMetrics.completed[key]
}

func resetPendingConfigPlansForTests() {
	pendingConfigPlans = configplan.NewMemoryStore()
	configClarificationStore.mu.Lock()
	configClarificationStore.items = make(map[string]pendingConfigClarification)
	configClarificationStore.mu.Unlock()
	configStepMetrics.mu.Lock()
	configStepMetrics.active = make(map[string]int)
	configStepMetrics.completed = make(map[string]int)
	configStepMetrics.mu.Unlock()
}

func defaultInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func ensurePlanSummaryAvailable(conversationID string, plan configplan.Plan) (configplan.Plan, bool, error) {
	normalized := plan.Normalize()
	if !configplan.ShouldRebuildSummary(normalized) {
		return normalized, false, nil
	}
	rebuilt := configplan.RebuildSummary(normalized, "show_rebuild")
	if err := setPendingConfigPlan(conversationID, rebuilt); err != nil {
		return configplan.Plan{}, false, err
	}
	return rebuilt, true, nil
}

func ensurePlanSummaryConsistency(plan configplan.Plan, reason string) (configplan.Plan, bool, string) {
	normalized := plan.Normalize()
	if !configplan.ShouldRebuildSummary(normalized) {
		return normalized, false, ""
	}
	rebuilt := configplan.RebuildSummary(normalized, reason)
	return rebuilt, true, reason
}

func reasonForAudit(rebuilt bool, reason string) string {
	if !rebuilt {
		return ""
	}
	return strings.TrimSpace(reason)
}

func isPatchIntentMessage(raw string) bool {
	text := strings.TrimSpace(raw)
	lower := strings.ToLower(text)
	hasActionCue := strings.Contains(text, "改成") ||
		strings.Contains(text, "改为") ||
		strings.Contains(text, "设置") ||
		strings.Contains(text, "设为") ||
		strings.Contains(text, "调整") ||
		strings.Contains(text, "改一下") ||
		strings.Contains(text, "修改") ||
		strings.Contains(lower, "=")
	if !hasActionCue {
		return false
	}
	_, matched, err := parsePatchFromNaturalLanguage(raw)
	return matched || err != nil
}

func auditPlanID(conversationID string, version int64) string {
	key := strings.TrimSpace(conversationID)
	if key == "" {
		return ""
	}
	return fmt.Sprintf("%s:v%d", key, version)
}

func formatControlPlaneSummary(plan configplan.Plan) string {
	normalized := plan.Normalize()
	if normalized.CompressedSummary == nil {
		return "- 摘要: 无"
	}
	summary := normalized.CompressedSummary
	values := []string{
		fmt.Sprintf("agent=%s", configplan.FieldValueFromPlan(normalized, configplan.SummaryFieldAgentID)),
		fmt.Sprintf("profile=%s", configplan.FieldValueFromPlan(normalized, configplan.SummaryFieldProfile)),
		fmt.Sprintf("workspace=%s", configplan.FieldValueFromPlan(normalized, configplan.SummaryFieldWorkspace)),
		fmt.Sprintf("timeout=%s", configplan.FieldValueFromPlan(normalized, configplan.SummaryFieldTimeout)),
		fmt.Sprintf("default=%s", configplan.FieldValueFromPlan(normalized, configplan.SummaryFieldDefault)),
	}
	return fmt.Sprintf("- 摘要版本: sv%d (patch=%d)\n- 摘要字段: %s", summary.Version, summary.PatchCount, strings.Join(values, ", "))
}

func formatRecentPatchTrails(plan configplan.Plan, limit int) string {
	history := plan.Normalize().PatchHistory
	if len(history) == 0 {
		return "- 最近变更: 无"
	}
	if limit <= 0 {
		limit = 5
	}
	start := len(history) - limit
	if start < 0 {
		start = 0
	}
	lines := make([]string, 0, len(history)-start)
	for i := start; i < len(history); i++ {
		patch := history[i]
		lines = append(lines, fmt.Sprintf("%d) %s=%s by=%s at=%s source=%s",
			i+1,
			patch.Operation,
			strings.TrimSpace(patch.Value),
			strings.TrimSpace(patch.By),
			patch.At.Format(time.RFC3339),
			patch.Source,
		))
	}
	return "- 最近变更:\n  " + strings.Join(lines, "\n  ")
}

func normalizeConfigToken(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	return strings.TrimPrefix(value, "/")
}

func recordConfigAudit(event configplan.AuditEvent) {
	log.Print(event.FormatForLog())
}
