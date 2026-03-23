package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"clawx/internal/application/service"
	"clawx/internal/application/skillorchestrator"
	chatiface "clawx/internal/interfaces/chat"
)

var requirementSyncBlockPattern = regexp.MustCompile("(?is)```(?:json)?\\s*(\\{.*?\\})\\s*```")

var defaultWorkspaceDocTemplates = map[string]string{
	"AGENTS.md":    "# AGENTS\n\n- status: initialized\n",
	"BOOTSTRAP.md": "# BOOTSTRAP\n\n- status: initialized\n",
	"HEARTBEAT.md": "# HEARTBEAT\n\n- status: active\n",
	"IDENTITY.md":  "# IDENTITY\n\n- role: workspace context\n",
	"SOUL.md":      "# SOUL\n\n- principle: keep tasks focused\n",
	"TOOLS.md":     "# TOOLS\n\n- workflow: command and skill driven\n",
	"USER.md":      "# USER\n\n- preferences: concise and actionable\n",
}

func ensureWorkspaceDocsScaffold(root string) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	for relative, content := range defaultWorkspaceDocTemplates {
		path := filepath.Join(root, relative)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func maybeSyncRequirementDocs(ctx context.Context, runtime agentRuntime, decision service.Decision) error {
	requirement := strings.TrimSpace(decision.Message.Text)
	return maybeSyncRequirementDocsWithText(ctx, runtime, decision, requirement, false)
}

func maybeSyncRequirementDocsWithText(ctx context.Context, runtime agentRuntime, decision service.Decision, requirement string, force bool) error {
	if decision.Kind != service.DecisionExecute && decision.Kind != service.DecisionSkill {
		return nil
	}
	requirement = strings.TrimSpace(requirement)
	if !force && !looksLikeRequirementUpdate(requirement) {
		return nil
	}
	root, source := resolveRequirementWorkspace(ctx, runtime, decision)
	if strings.TrimSpace(root) == "" {
		return nil
	}
	if err := ensureWorkspaceDocsScaffold(root); err != nil {
		return fmt.Errorf("ensure workspace doc scaffold: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	userDoc := filepath.Join(root, "USER.md")
	agentDoc := filepath.Join(root, "AGENTS.md")
	heartbeatDoc := filepath.Join(root, "HEARTBEAT.md")
	if contains, err := markdownContains(userDoc, requirement); err != nil {
		return err
	} else if contains {
		emitTrace("requirement_doc_sync", map[string]any{
			"agent_id":        strings.TrimSpace(runtime.agentID),
			"conversation_id": strings.TrimSpace(decision.ConversationID),
			"project_id":      strings.TrimSpace(decision.ProjectID),
			"workspace_root":  root,
			"workspace_src":   source,
			"status":          "skipped_duplicate",
			"requirement":     tracePreview(requirement, 240),
		})
		return nil
	}
	entry := fmt.Sprintf("- [%s] %s", now, requirement)
	if err := appendUniqueMarkdownLine(userDoc, "## Requirement Updates", entry); err != nil {
		return err
	}
	agent := strings.TrimSpace(runtime.agentID)
	if agent == "" {
		agent = "main"
	}
	agentEntry := fmt.Sprintf("- [%s] agent=%s source=%s requirement=%s", now, agent, source, requirement)
	if err := appendUniqueMarkdownLine(agentDoc, "## Requirement Sync", agentEntry); err != nil {
		return err
	}
	heartbeatLine := fmt.Sprintf("- [%s] requirement_synced_by=%s", now, agent)
	if err := appendUniqueMarkdownLine(heartbeatDoc, "## Updates", heartbeatLine); err != nil {
		return err
	}
	if err := appendRequirementMemoryDocs(root, now, requirement, agent, source); err != nil {
		return err
	}
	emitTrace("requirement_doc_sync", map[string]any{
		"agent_id":        agent,
		"conversation_id": strings.TrimSpace(decision.ConversationID),
		"project_id":      strings.TrimSpace(decision.ProjectID),
		"workspace_root":  root,
		"workspace_src":   source,
		"requirement":     tracePreview(requirement, 240),
	})
	return nil
}

func appendRequirementMemoryDocs(root, now, requirement, agent, source string) error {
	if err := appendUniqueMarkdownLine(
		filepath.Join(root, "BOOTSTRAP.md"),
		"## Requirement Intake",
		fmt.Sprintf("- [%s] status=captured agent=%s source=%s", now, agent, source),
	); err != nil {
		return err
	}
	if err := appendUniqueMarkdownLine(
		filepath.Join(root, "IDENTITY.md"),
		"## Mission Updates",
		fmt.Sprintf("- [%s] %s", now, summarizeRequirementForIdentity(requirement)),
	); err != nil {
		return err
	}
	principles := deriveSoulPrinciples(requirement)
	for _, line := range principles {
		if err := appendUniqueMarkdownLine(
			filepath.Join(root, "SOUL.md"),
			"## Constraints & Principles",
			fmt.Sprintf("- [%s] %s", now, line),
		); err != nil {
			return err
		}
	}
	targets := deriveToolTargets(requirement)
	for _, line := range targets {
		if err := appendUniqueMarkdownLine(
			filepath.Join(root, "TOOLS.md"),
			"## Capability Targets",
			fmt.Sprintf("- [%s] %s", now, line),
		); err != nil {
			return err
		}
	}
	return nil
}

func summarizeRequirementForIdentity(requirement string) string {
	text := strings.Join(strings.Fields(strings.TrimSpace(requirement)), " ")
	if text == "" {
		return "需求更新已记录"
	}
	runes := []rune(text)
	if len(runes) > 120 {
		return string(runes[:120]) + "..."
	}
	return text
}

func deriveSoulPrinciples(requirement string) []string {
	text := strings.ToLower(strings.TrimSpace(requirement))
	lines := make([]string, 0, 5)
	if strings.Contains(text, "周期") || strings.Contains(text, "定期") {
		lines = append(lines, "原则：支持周期性监控与持续更新。")
	}
	if strings.Contains(text, "主动") || strings.Contains(text, "查询") {
		lines = append(lines, "原则：支持用户主动查询与即时反馈。")
	}
	if strings.Contains(text, "去重") || strings.Contains(text, "结构化") {
		lines = append(lines, "原则：结果去重并结构化输出。")
	}
	if strings.Contains(text, "企业微信") || strings.Contains(text, "钉钉") || strings.Contains(text, "wecom") || strings.Contains(text, "dingding") {
		lines = append(lines, "原则：通知触达办公协同平台。")
	}
	if strings.Contains(text, "ubuntu") || strings.Contains(text, "linux") || strings.Contains(text, "playwright") {
		lines = append(lines, "约束：Linux 环境下支持浏览器自动化抓取。")
	}
	if len(lines) == 0 {
		lines = append(lines, "原则：需求更新需可追踪、可执行。")
	}
	return lines
}

func deriveToolTargets(requirement string) []string {
	text := strings.ToLower(strings.TrimSpace(requirement))
	lines := make([]string, 0, 6)
	if strings.Contains(text, "抓取") || strings.Contains(text, "爬取") {
		lines = append(lines, "能力：网页抓取与公告抽取流水线。")
	}
	if strings.Contains(text, "关键词") || strings.Contains(text, "公司") {
		lines = append(lines, "能力：关键词/公司维度过滤与检索。")
	}
	if strings.Contains(text, "企业微信") || strings.Contains(text, "wecom") {
		lines = append(lines, "集成：企业微信通知通道。")
	}
	if strings.Contains(text, "钉钉") || strings.Contains(text, "dingding") {
		lines = append(lines, "集成：钉钉通知通道。")
	}
	if strings.Contains(text, "playwright") || strings.Contains(text, "浏览器") {
		lines = append(lines, "技术：Playwright 浏览器自动化采集。")
	}
	if len(lines) == 0 {
		lines = append(lines, "能力：需求相关工具链待补充。")
	}
	return lines
}

type requirementSyncResult struct {
	Status    string
	Source    string
	Workspace string
	AgentID   string
	Message   string
}

func maybeAutoApplyRequirementSyncFromModel(
	ctx context.Context,
	runtime agentRuntime,
	decision service.Decision,
	modelOutput string,
	channel string,
	instanceID string,
	scopeKey string,
	overrides *conversationAgentOverrides,
	runtimes map[string]agentRuntime,
	defaultAgentID string,
) (requirementSyncResult, bool, error) {
	plan, ok, err := skillorchestrator.ParseRequirementSyncPlanFromText(modelOutput)
	if err != nil {
		emitTrace("requirement_doc_sync", map[string]any{
			"channel":         strings.TrimSpace(channel),
			"instance":        strings.TrimSpace(instanceID),
			"agent_id":        strings.TrimSpace(runtime.agentID),
			"conversation_id": strings.TrimSpace(decision.ConversationID),
			"project_id":      strings.TrimSpace(decision.ProjectID),
			"status":          "error",
			"source":          "llm_plan",
			"error":           tracePreview(err.Error(), 240),
		})
		return requirementSyncResult{}, false, err
	}
	if ok {
		targetRuntime := runtime
		targetAgentID := cleanAgentIDToken(plan.AgentID)
		if strings.TrimSpace(plan.Mode) == "suggest" {
			suggestAgent := targetAgentID
			if suggestAgent == "" {
				suggestAgent = strings.TrimSpace(runtime.agentID)
			}
			message := strings.TrimSpace(plan.Reason)
			if message == "" {
				if suggestAgent == strings.TrimSpace(runtime.agentID) || suggestAgent == "" {
					message = "已识别需求更新建议。请确认是否继续执行 requirement_sync。"
				} else {
					message = fmt.Sprintf("已识别需求更新建议。请确认是否切换到 `%s` 并执行 requirement_sync。", suggestAgent)
				}
			}
			return requirementSyncResult{
				Status:  "suggest",
				Source:  "llm_plan",
				AgentID: suggestAgent,
				Message: message,
			}, true, nil
		}
		switchMsg := ""
		if targetAgentID != "" {
			if _, exists := runtimes[targetAgentID]; !exists {
				return requirementSyncResult{
					Status:  "skipped",
					Source:  "llm_plan",
					Message: "requirement_sync 指定的 agent_id 不存在，未更新文档。",
				}, true, nil
			}
			if targetAgentID != strings.TrimSpace(runtime.agentID) {
				command := "/agent use " + targetAgentID
				handled, response, switchErr := handleAgentChatCommand(chatiface.Message{Text: command}, scopeKey, overrides, runtimes, defaultAgentID)
				if switchErr != nil {
					return requirementSyncResult{}, true, switchErr
				}
				if handled {
					switchMsg = strings.TrimSpace(response)
				}
			}
			targetRuntime = selectRuntime(runtimes, defaultAgentID, targetAgentID)
		}

		root, _ := resolveRequirementWorkspace(ctx, targetRuntime, decision)
		if strings.TrimSpace(root) == "" {
			return requirementSyncResult{
				Status:  "skipped",
				Source:  "llm_plan",
				AgentID: strings.TrimSpace(targetRuntime.agentID),
				Message: "检测到 requirement_sync，但当前未解析到可写 workspace，未落盘。请检查 agent workspace 配置。",
			}, true, nil
		}
		if err := maybeSyncRequirementDocsWithText(ctx, targetRuntime, decision, plan.Requirement, true); err != nil {
			emitTrace("requirement_doc_sync", map[string]any{
				"channel":         strings.TrimSpace(channel),
				"instance":        strings.TrimSpace(instanceID),
				"agent_id":        strings.TrimSpace(runtime.agentID),
				"conversation_id": strings.TrimSpace(decision.ConversationID),
				"project_id":      strings.TrimSpace(decision.ProjectID),
				"status":          "error",
				"source":          "llm_plan",
				"error":           tracePreview(err.Error(), 240),
			})
			return requirementSyncResult{}, true, err
		}
		if contains, err := markdownContains(filepath.Join(root, "USER.md"), plan.Requirement); err == nil && !contains {
			return requirementSyncResult{
				Status:    "skipped",
				Source:    "llm_plan",
				Workspace: root,
				AgentID:   strings.TrimSpace(targetRuntime.agentID),
				Message:   "requirement_sync 已识别，但未检测到 USER.md 内容变化。",
			}, true, nil
		}
		msg := "已根据结构化 requirement_sync 计划更新需求文档"
		if switchMsg != "" && !strings.Contains(switchMsg, "无需切换") {
			msg = switchMsg + "\n" + msg
		}
		return requirementSyncResult{
			Status:    "applied",
			Source:    "llm_plan",
			Workspace: root,
			AgentID:   strings.TrimSpace(targetRuntime.agentID),
			Message:   msg,
		}, true, nil
	}

	if err := maybeSyncRequirementDocs(ctx, runtime, decision); err != nil {
		emitTrace("requirement_doc_sync", map[string]any{
			"channel":         strings.TrimSpace(channel),
			"instance":        strings.TrimSpace(instanceID),
			"agent_id":        strings.TrimSpace(runtime.agentID),
			"conversation_id": strings.TrimSpace(decision.ConversationID),
			"project_id":      strings.TrimSpace(decision.ProjectID),
			"status":          "error",
			"source":          "keyword_fallback",
			"error":           tracePreview(err.Error(), 240),
		})
		return requirementSyncResult{}, false, err
	}
	return requirementSyncResult{}, false, nil
}

func resolveRequirementWorkspace(ctx context.Context, runtime agentRuntime, decision service.Decision) (string, string) {
	agentID := strings.TrimSpace(runtime.agentID)
	if agentID != "" {
		if agent, ok := runtime.cfgSnapshot.Agents[agentID]; ok {
			if workspace := strings.TrimSpace(agent.Workspace); workspace != "" {
				if runtime.cfgSnapshot.ValidateWorkingDirectory(workspace) == nil {
					return filepath.Clean(workspace), "agent_config"
				}
			}
		}
	}
	if root := strings.TrimSpace(runtime.cwd); root != "" {
		return filepath.Clean(root), "runtime_cwd"
	}
	projectID := strings.TrimSpace(decision.ProjectID)
	if projectID != "" && runtime.projectCWD != nil {
		record, err := runtime.projectCWD.GetProject(ctx, projectID)
		if err == nil {
			root := strings.TrimSpace(record.WorkspacePath)
			if root != "" {
				return filepath.Clean(root), "project_registry"
			}
		}
	}
	return "", ""
}

func looksLikeRequirementUpdate(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	keywords := []string{
		"需求", "项目需求", "用户故事", "验收标准", "范围", "约束", "prd", "requirement",
		"spec", "roadmap", "milestone", "任务分解", "实现计划", "功能设计",
	}
	for _, keyword := range keywords {
		if strings.Contains(normalized, keyword) || strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func appendUniqueMarkdownLine(path, section, line string) error {
	path = strings.TrimSpace(path)
	section = strings.TrimSpace(section)
	line = strings.TrimSpace(line)
	if path == "" || section == "" || line == "" {
		return nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}
	text := string(body)
	if strings.Contains(text, line) {
		return nil
	}
	if !strings.Contains(text, section) {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += "\n" + section + "\n\n"
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += line + "\n"
	return os.WriteFile(path, []byte(text), 0o644)
}

func markdownContains(path, token string) (bool, error) {
	path = strings.TrimSpace(path)
	token = strings.TrimSpace(token)
	if path == "" || token == "" {
		return false, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return strings.Contains(string(body), token), nil
}

func formatRequirementSyncResult(result requirementSyncResult) string {
	status := strings.TrimSpace(result.Status)
	if status == "" {
		status = "applied"
	}
	message := strings.TrimSpace(result.Message)
	if message == "" {
		message = "需求文档已更新"
	}
	targetAgent := strings.TrimSpace(result.AgentID)
	workspace := strings.TrimSpace(result.Workspace)

	switch status {
	case "applied":
		if targetAgent != "" && workspace != "" {
			return fmt.Sprintf("已更新需求文档（agent=%s，workspace=%s）。", targetAgent, workspace)
		}
		if targetAgent != "" {
			return fmt.Sprintf("已更新需求文档（agent=%s）。", targetAgent)
		}
		return "已更新需求文档。"
	case "suggest":
		if targetAgent != "" {
			return fmt.Sprintf("建议先确认目标智能体 `%s` 再执行需求更新。%s", targetAgent, message)
		}
		return "建议先确认后再执行需求更新。"
	case "skipped":
		return message
	default:
		if workspace != "" {
			return fmt.Sprintf("%s（workspace=%s）", message, workspace)
		}
		return message
	}
}

func stripRequirementSyncPlanPayload(output string) string {
	text := strings.TrimSpace(output)
	if text == "" {
		return text
	}
	if _, ok, err := skillorchestrator.ParseRequirementSyncPlanFromText(text); err == nil && ok {
		if strings.HasPrefix(text, "{") && strings.HasSuffix(text, "}") {
			return ""
		}
		cleaned := requirementSyncBlockPattern.ReplaceAllString(text, "")
		return strings.TrimSpace(cleaned)
	}
	return output
}
