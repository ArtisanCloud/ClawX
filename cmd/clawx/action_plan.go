package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"clawx/internal/application/runtimeorchestrator"
	"clawx/internal/application/service"
	chatiface "clawx/internal/interfaces/chat"
)

var actionPlanBlockPattern = regexp.MustCompile("(?is)```(?:json)?\\s*(\\{.*?\"type\"\\s*:\\s*\"action_plan\".*?\\})\\s*```")

type actionPlan struct {
	Type    string           `json:"type"`
	Mode    string           `json:"mode"`
	Reason  string           `json:"reason,omitempty"`
	Actions []actionPlanItem `json:"actions"`
}

type actionPlanItem struct {
	Kind        string   `json:"kind"`
	Cmd         string   `json:"cmd,omitempty"`
	CWD         string   `json:"cwd,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Command     string   `json:"command,omitempty"`
	AgentID     string   `json:"agent_id,omitempty"`
	Requirement string   `json:"requirement,omitempty"`
	Mode        string   `json:"mode,omitempty"`
	WorkerRoles []string `json:"worker_roles,omitempty"`
}

func parseActionPlanFromText(text string) (actionPlan, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return actionPlan{}, false
	}
	candidates := make([]string, 0, 2)
	if matches := actionPlanBlockPattern.FindAllStringSubmatch(trimmed, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) >= 2 {
				candidates = append(candidates, strings.TrimSpace(match[1]))
			}
		}
	}
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		candidates = append(candidates, trimmed)
	}
	for _, candidate := range candidates {
		var plan actionPlan
		if err := json.Unmarshal([]byte(candidate), &plan); err != nil {
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
	return strings.TrimSpace(cleaned)
}

type actionPlanApplyResult struct {
	Applied         bool
	Status          string
	Message         string
	ResponseAgentID string
}

var specKitDocPathPattern = regexp.MustCompile(`(?i)(/[^,\s]+/(SPEC|PLAN|TASKS|ANALYZE)\.md)`)
var specKitDirPathPattern = regexp.MustCompile(`(?i)(/[^,\s'"]+/docs/spec-kit/[^/\s'"]+)`)
var specKitDirRelPattern = regexp.MustCompile(`(?i)docs/spec-kit/([^/\s'"]+)`)

func formatActionPlanApplyResult(result actionPlanApplyResult) string {
	if msg := strings.TrimSpace(result.Message); msg != "" {
		return msg
	}
	return fmt.Sprintf("action_plan status=%s", strings.TrimSpace(result.Status))
}

func renderActionPlanUserMessage(status string, notes []string, appliedCount, failedCount int) string {
	status = strings.TrimSpace(strings.ToLower(status))
	if len(notes) == 1 {
		only := strings.TrimSpace(notes[0])
		if strings.HasPrefix(only, "我已在当前环境执行") || strings.HasPrefix(only, "我在当前环境尝试") {
			return only
		}
	}
	lines := make([]string, 0, len(notes)+2)
	switch status {
	case "applied":
		lines = append(lines, fmt.Sprintf("我已完成本轮自动执行，共成功 %d 项。", appliedCount))
	case "partial":
		lines = append(lines, fmt.Sprintf("我已完成本轮自动执行，成功 %d 项，失败 %d 项。", appliedCount, failedCount))
	case "failed":
		lines = append(lines, fmt.Sprintf("本轮自动执行未成功（失败 %d 项）。", failedCount))
	default:
		lines = append(lines, "本轮自动执行没有产生可落地动作。")
	}
	for i, note := range notes {
		note = strings.TrimSpace(note)
		if note == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, note))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
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
	plan, ok := parseActionPlanFromText(output)
	if !ok {
		return actionPlanApplyResult{}, false, nil
	}
	if strings.TrimSpace(plan.Reason) != "" {
		setExecutionGoalState(decision.ConversationID, executionGoalState{
			Goal:   strings.TrimSpace(plan.Reason),
			Status: "running",
		})
		appendTrackingGoal(runtime, fallbackCWD, decision.ConversationID, strings.TrimSpace(plan.Reason))
	}
	if strings.TrimSpace(plan.Mode) == "" {
		plan.Mode = "execute"
	}
	notes := make([]string, 0, len(plan.Actions)+1)
	appliedCount := 0
	failedCount := 0
	responseAgentID := strings.TrimSpace(runtime.agentID)

	execPlan := runtimeExecPlan{Type: "runtime.exec", Mode: plan.Mode, Reason: plan.Reason}
	for _, action := range plan.Actions {
		kind := strings.TrimSpace(strings.ToLower(action.Kind))
		switch kind {
		case "runtime.exec":
			cmd := strings.TrimSpace(action.Cmd)
			if cmd == "" {
				continue
			}
			execPlan.Commands = append(execPlan.Commands, runtimeExecCommand{
				Cmd:    cmd,
				CWD:    strings.TrimSpace(action.CWD),
				Reason: strings.TrimSpace(action.Reason),
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
				notes = append(notes, "- runtime.bootstrap 缺少可用 workspace，已跳过")
				continue
			}
			if err := runtime.cfgSnapshot.ValidateWorkingDirectory(workspace); err != nil {
				failedCount++
				notes = append(notes, "- runtime.bootstrap workspace 超出允许范围: "+err.Error())
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
				notes = append(notes, "- runtime.bootstrap 执行失败: "+err.Error())
				continue
			}
			appliedCount++
			notes = append(notes, fmt.Sprintf("- runtime 初始化完成：workers=%d runtime_dir=%s", result.WorkerCount, result.RuntimeDir))
		case "agent.use":
			agentID := cleanAgentIDToken(action.AgentID)
			if agentID == "" {
				failedCount++
				notes = append(notes, "- agent.use 缺少 agent_id，已跳过")
				continue
			}
			applied, ok, err := applyActionPlanAgentUse(agentID, scopeKey, overrides, runtimes, defaultAgentID)
			if err != nil {
				failedCount++
				notes = append(notes, "- agent.use 执行失败: "+err.Error())
				continue
			}
			if ok {
				appliedCount++
				notes = append(notes, "- "+formatControlApplyResult(applied))
				if strings.TrimSpace(applied.Target) != "" {
					responseAgentID = strings.TrimSpace(applied.Target)
				}
			}
		case "requirement.sync":
			req := strings.TrimSpace(action.Requirement)
			if req == "" {
				failedCount++
				notes = append(notes, "- requirement.sync 缺少 requirement，已跳过")
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
				notes = append(notes, "- requirement.sync 执行失败: "+reqErr.Error())
				continue
			}
			if reqApplied {
				appliedCount++
				notes = append(notes, "- "+strings.ReplaceAll(formatRequirementSyncResult(reqResult), "\n", " "))
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
				notes = append(notes, "- config.exec 缺少 command/cmd，已跳过")
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
				notes = append(notes, "- config.exec 执行失败: "+cfgErr.Error())
				continue
			}
			if !handled {
				failedCount++
				notes = append(notes, "- config.exec 未被处理: "+commandText)
				continue
			}
			appliedCount++
			notes = append(notes, "- "+strings.ReplaceAll(strings.TrimSpace(response), "\n", " "))
		default:
			failedCount++
			notes = append(notes, "- 未支持的 action.kind: "+kind)
			continue
		}
	}
	if len(execPlan.Commands) > 0 || strings.TrimSpace(plan.Mode) == "suggest" {
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
				if specDir != "" {
					notes = append(notes, fmt.Sprintf("- Spec Kit 文档未完整（目录：%s），缺少：%s", specDir, strings.Join(missing, ", ")))
				} else {
					notes = append(notes, fmt.Sprintf("- Spec Kit 文档未完整，缺少：%s", strings.Join(missing, ", ")))
				}
				notes = append(notes, "- 结论：当前暂不进入实现阶段。")
				notes = append(notes, fmt.Sprintf("- 下一步：请先补齐 %s，再进入实现阶段。", strings.Join(missing, "、")))
				notes = append(notes, fmt.Sprintf("- 你可以直接回复：继续补齐 %s", strings.Join(missing, " 和 ")))
			} else {
				setSpecKitFlowActive(decision.ConversationID, false)
			}
		}
	} else if len(plan.Actions) == 0 {
		failedCount++
		notes = append(notes, "- action_plan 未包含 actions")
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
	if strings.TrimSpace(plan.Reason) != "" {
		setExecutionGoalState(decision.ConversationID, executionGoalState{
			Goal:       strings.TrimSpace(plan.Reason),
			Status:     status,
			LastResult: summarizeText(msg, 220),
		})
	}
	return actionPlanApplyResult{
		Applied:         appliedCount > 0,
		Status:          status,
		Message:         strings.TrimSpace(msg),
		ResponseAgentID: strings.TrimSpace(responseAgentID),
	}, true, nil
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
