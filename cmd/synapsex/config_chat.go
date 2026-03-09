package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"synapsex/internal/infrastructure/config"
	chatiface "synapsex/internal/interfaces/chat"
)

type configPlanKind string

const (
	configPlanUpsertAgent configPlanKind = "upsert-agent"
	configPlanSetDefault  configPlanKind = "set-default-agent"
)

type pendingConfigPlan struct {
	Kind            configPlanKind
	ConversationID  string
	CreatedBy       string
	CreatedAt       time.Time
	Summary         string
	AgentUpsertOpts config.AgentUpsertOptions
	DefaultAgentID  string
}

type parsedConfigCommand struct {
	Action      string
	Instruction string
}

var (
	pendingConfigPlansMu sync.Mutex
	pendingConfigPlans   = make(map[string]pendingConfigPlan)
)

func handleConfigChatCommand(message chatiface.Message) (bool, string, error) {
	cmd, isConfigCommand, err := parseConfigChatCommand(message.Text)
	if err != nil {
		return true, "", err
	}
	if !isConfigCommand {
		return false, "", nil
	}

	if !isConfigAdmin(message.UserID) {
		return true, "你没有配置权限。", nil
	}

	switch cmd.Action {
	case "help":
		return true, configHelpText(), nil
	case "show":
		plan, ok := getPendingConfigPlan(message.ConversationID)
		if !ok {
			return true, "当前没有待确认的配置计划。", nil
		}
		return true, fmt.Sprintf("待确认计划:\n- %s\n- 创建时间: %s\n发送 `/config apply` 执行，或 `/config cancel` 取消。", plan.Summary, plan.CreatedAt.Format(time.RFC3339)), nil
	case "cancel":
		if deletePendingConfigPlan(message.ConversationID) {
			return true, "已取消待确认的配置计划。", nil
		}
		return true, "当前没有待确认的配置计划。", nil
	case "apply":
		plan, ok := popPendingConfigPlan(message.ConversationID)
		if !ok {
			return true, "当前没有待确认的配置计划，请先发送 `/config plan ...`。", nil
		}
		response, err := applyPendingConfigPlan(plan)
		return true, response, err
	case "plan":
		plan, err := buildConfigPlanFromInstruction(cmd.Instruction, message)
		if err != nil {
			return true, "", err
		}
		setPendingConfigPlan(message.ConversationID, plan)
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

func buildConfigPlanFromInstruction(instruction string, message chatiface.Message) (pendingConfigPlan, error) {
	trimmed := strings.TrimSpace(instruction)
	if trimmed == "" {
		return pendingConfigPlan{}, fmt.Errorf("空指令")
	}
	tokens := strings.Fields(trimmed)
	if len(tokens) == 0 {
		return pendingConfigPlan{}, fmt.Errorf("空指令")
	}

	lowered := make([]string, len(tokens))
	for i, token := range tokens {
		lowered[i] = strings.ToLower(strings.TrimSpace(token))
	}

	if isSetDefaultInstruction(lowered, trimmed) {
		agentID := parseSetDefaultAgentID(tokens, lowered)
		if agentID == "" {
			return pendingConfigPlan{}, fmt.Errorf("无法识别默认 agent。示例：`/config plan default agent review`")
		}
		agentID = strings.TrimSpace(agentID)
		return pendingConfigPlan{
			Kind:           configPlanSetDefault,
			ConversationID: message.ConversationID,
			CreatedBy:      message.UserID,
			CreatedAt:      time.Now().UTC(),
			Summary:        fmt.Sprintf("设置默认 Agent 为 `%s`", agentID),
			DefaultAgentID: agentID,
		}, nil
	}

	if isCreateAgentInstruction(lowered, trimmed) {
		opts, err := parseAgentUpsertOptions(tokens, lowered)
		if err != nil {
			return pendingConfigPlan{}, err
		}
		summary := fmt.Sprintf("新增/更新 Agent `%s` (profile=`%s`, workspace=`%s`, timeout=%ds, default=%t)",
			opts.ID,
			opts.ProfileID,
			firstNonEmpty(opts.Workspace, config.SuggestedAgentWorkspace(opts.ID)),
			defaultInt(opts.TimeoutSeconds, 600),
			opts.SetAsDefault,
		)
		return pendingConfigPlan{
			Kind:            configPlanUpsertAgent,
			ConversationID:  message.ConversationID,
			CreatedBy:       message.UserID,
			CreatedAt:       time.Now().UTC(),
			Summary:         summary,
			AgentUpsertOpts: opts,
		}, nil
	}

	return pendingConfigPlan{}, fmt.Errorf("暂不支持该自然语言配置。当前支持：创建/更新 agent、切换默认 agent。发送 `/config help` 查看示例")
}

func parseAgentUpsertOptions(tokens, lowered []string) (config.AgentUpsertOptions, error) {
	var opts config.AgentUpsertOptions

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
		return config.AgentUpsertOptions{}, fmt.Errorf("无法识别 agent id。示例：`/config plan 创建 agent review 使用 claude`")
	}
	if strings.TrimSpace(opts.ProfileID) == "" {
		opts.ProfileID = "codex"
	}
	if opts.TimeoutSeconds <= 0 {
		opts.TimeoutSeconds = 600
	}
	return opts, nil
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

func applyPendingConfigPlan(plan pendingConfigPlan) (string, error) {
	switch plan.Kind {
	case configPlanUpsertAgent:
		path, err := config.UpsertAgent(plan.AgentUpsertOpts)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("配置已写入：%s\n- %s\n请重启服务使运行时加载新配置。", path, plan.Summary), nil
	case configPlanSetDefault:
		path, err := config.SetDefaultAgent(plan.DefaultAgentID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("配置已写入：%s\n- %s\n请重启服务使运行时加载新配置。", path, plan.Summary), nil
	default:
		return "", fmt.Errorf("未知配置计划类型")
	}
}

func isConfigAdmin(userID string) bool {
	allowed := strings.TrimSpace(os.Getenv("SYNAPSEX_CONFIG_ADMIN_USERS"))
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
- /config show
- /config apply
- /config cancel

说明：
- plan 只生成待确认变更，不会立即写文件
- apply 才会写入 config.json
- 写入后需要重启服务生效`)
}

func setPendingConfigPlan(conversationID string, plan pendingConfigPlan) {
	pendingConfigPlansMu.Lock()
	defer pendingConfigPlansMu.Unlock()
	pendingConfigPlans[strings.TrimSpace(conversationID)] = plan
}

func getPendingConfigPlan(conversationID string) (pendingConfigPlan, bool) {
	pendingConfigPlansMu.Lock()
	defer pendingConfigPlansMu.Unlock()
	plan, ok := pendingConfigPlans[strings.TrimSpace(conversationID)]
	return plan, ok
}

func popPendingConfigPlan(conversationID string) (pendingConfigPlan, bool) {
	pendingConfigPlansMu.Lock()
	defer pendingConfigPlansMu.Unlock()
	key := strings.TrimSpace(conversationID)
	plan, ok := pendingConfigPlans[key]
	if ok {
		delete(pendingConfigPlans, key)
	}
	return plan, ok
}

func deletePendingConfigPlan(conversationID string) bool {
	pendingConfigPlansMu.Lock()
	defer pendingConfigPlansMu.Unlock()
	key := strings.TrimSpace(conversationID)
	if _, ok := pendingConfigPlans[key]; ok {
		delete(pendingConfigPlans, key)
		return true
	}
	return false
}

func defaultInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func normalizeConfigToken(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	return strings.TrimPrefix(value, "/")
}
