package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	chatiface "clawx/internal/interfaces/chat"
)

type conversationAgentOverrides struct {
	mu      sync.RWMutex
	byScope map[string]string
}

type controlApplyResult struct {
	Action  string
	Target  string
	Status  string
	Message string
}

func newConversationAgentOverrides() *conversationAgentOverrides {
	return &conversationAgentOverrides{
		byScope: make(map[string]string),
	}
}

func (o *conversationAgentOverrides) Set(scopeKey, agentID string) {
	scopeKey = strings.TrimSpace(scopeKey)
	agentID = strings.TrimSpace(agentID)
	if scopeKey == "" || agentID == "" {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.byScope[scopeKey] = agentID
}

func (o *conversationAgentOverrides) Get(scopeKey string) (string, bool) {
	scopeKey = strings.TrimSpace(scopeKey)
	if scopeKey == "" {
		return "", false
	}
	o.mu.RLock()
	defer o.mu.RUnlock()
	agentID, ok := o.byScope[scopeKey]
	return strings.TrimSpace(agentID), ok
}

func (o *conversationAgentOverrides) Clear(scopeKey string) bool {
	scopeKey = strings.TrimSpace(scopeKey)
	if scopeKey == "" {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, ok := o.byScope[scopeKey]; ok {
		delete(o.byScope, scopeKey)
		return true
	}
	return false
}

func routingScopeKey(channel, instanceID, baseConversationID string) string {
	channel = sanitizeConversationSegment(channel, "channel")
	instanceID = sanitizeConversationSegment(instanceID, "default")
	baseConversationID = sanitizeConversationSegment(baseConversationID, "-")
	return fmt.Sprintf("scope:%s:%s:%s", channel, instanceID, baseConversationID)
}

func handleAgentChatCommand(message chatiface.Message, scopeKey string, overrides *conversationAgentOverrides, runtimes map[string]agentRuntime, defaultAgentID string) (bool, string, error) {
	fields := strings.Fields(strings.TrimSpace(message.Text))
	if len(fields) == 0 || normalizeConfigToken(fields[0]) != "agent" {
		return false, "", nil
	}

	if len(fields) == 1 {
		return true, agentHelpText(), nil
	}

	action := strings.ToLower(strings.TrimSpace(fields[1]))
	switch action {
	case "help":
		return true, agentHelpText(), nil
	case "list":
		ids := make([]string, 0, len(runtimes))
		for id := range runtimes {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		if len(ids) == 0 {
			return true, "当前没有可用 Agent。", nil
		}
		lines := make([]string, 0, len(ids)+1)
		lines = append(lines, "可用 Agent:")
		for _, id := range ids {
			line := "- " + id
			if strings.TrimSpace(id) == strings.TrimSpace(defaultAgentID) {
				line += " (default)"
			}
			lines = append(lines, line)
		}
		return true, strings.Join(lines, "\n"), nil
	case "current":
		if agentID, ok := overrides.Get(scopeKey); ok {
			return true, fmt.Sprintf("当前会话已固定 Agent: %s", agentID), nil
		}
		return true, fmt.Sprintf("当前会话使用默认路由（default=%s）", defaultAgentID), nil
	case "use", "switch", "open":
		if len(fields) < 3 {
			return true, "", fmt.Errorf("缺少 agent id。示例：`/agent use main`")
		}
		agentID := strings.TrimSpace(fields[2])
		if _, ok := runtimes[agentID]; !ok {
			return true, "", fmt.Errorf("agent %q 不存在。先用 `/agent list` 查看可用项", agentID)
		}
		if current, ok := overrides.Get(scopeKey); ok && strings.TrimSpace(current) == agentID {
			return true, fmt.Sprintf("当前会话已是 Agent: %s（无需切换）", agentID), nil
		}
		if _, ok := overrides.Get(scopeKey); !ok && strings.TrimSpace(defaultAgentID) == agentID {
			return true, fmt.Sprintf("当前会话已是默认 Agent: %s（无需切换）", agentID), nil
		}
		overrides.Set(scopeKey, agentID)
		return true, fmt.Sprintf("当前会话已切换到 Agent: %s\n后续消息会路由到该 Agent。", agentID), nil
	case "clear", "reset":
		if overrides.Clear(scopeKey) {
			return true, "已清除当前会话 Agent 固定设置，恢复默认路由。", nil
		}
		return true, "当前会话没有固定 Agent。", nil
	default:
		return true, "", fmt.Errorf("未知 agent 子命令 %q。发送 `/agent help` 查看用法", action)
	}
}

func agentHelpText() string {
	return strings.TrimSpace(`
Agent 指令：
- /agent list
- /agent current
- /agent use <agent_id>
- /agent clear

说明：
- use 只影响当前会话范围（当前 channel + bot instance + conversation）
- clear 后恢复配置路由规则`)
}

func maybeAutoApplyAgentSwitch(
	userText string,
	modelOutput string,
	scopeKey string,
	overrides *conversationAgentOverrides,
	runtimes map[string]agentRuntime,
	defaultAgentID string,
) (controlApplyResult, bool, error) {
	const action = "agent_use"
	if !looksLikeAgentSwitchRequest(userText) {
		return controlApplyResult{}, false, nil
	}
	command, ok := extractAgentSwitchCommandFromOutput(modelOutput)
	if !ok {
		return controlApplyResult{}, false, nil
	}
	if !isAllowedControlAutoApplyCommand(command) {
		return controlApplyResult{}, false, nil
	}
	handled, response, err := handleAgentChatCommand(chatiface.Message{Text: command}, scopeKey, overrides, runtimes, defaultAgentID)
	if err != nil {
		return controlApplyResult{}, false, err
	}
	if !handled {
		return controlApplyResult{}, false, nil
	}
	target := extractAgentIDFromAgentUseCommand(command)
	status := "applied"
	if strings.Contains(response, "无需切换") {
		status = "noop"
	}
	return controlApplyResult{
		Action:  action,
		Target:  target,
		Status:  status,
		Message: response,
	}, true, nil
}

func looksLikeAgentSwitchRequest(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	hasAgent := strings.Contains(normalized, "agent") || strings.Contains(text, "智能体")
	if !hasAgent {
		return false
	}
	return strings.Contains(normalized, "switch") ||
		strings.Contains(normalized, "use") ||
		strings.Contains(text, "切换") ||
		strings.Contains(text, "换到") ||
		strings.Contains(text, "改成")
}

func extractAgentSwitchCommandFromOutput(output string) (string, bool) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 {
			continue
		}
		head := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fields[0])), "/")
		action := strings.ToLower(strings.TrimSpace(fields[1]))
		if head != "agent" {
			continue
		}
		if action != "use" && action != "switch" && action != "open" {
			continue
		}
		agentID := cleanAgentIDToken(fields[2])
		if agentID == "" {
			continue
		}
		return "/agent use " + agentID, true
	}
	return "", false
}

func cleanAgentIDToken(raw string) string {
	return strings.Trim(strings.TrimSpace(raw), "`'\"，,。.!！?？:：;；)）]】")
}

func isAllowedControlAutoApplyCommand(command string) bool {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) < 3 {
		return false
	}
	head := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fields[0])), "/")
	action := strings.ToLower(strings.TrimSpace(fields[1]))
	return head == "agent" && (action == "use" || action == "switch" || action == "open")
}

func extractAgentIDFromAgentUseCommand(command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) < 3 {
		return ""
	}
	return cleanAgentIDToken(fields[2])
}

func formatControlApplyResult(result controlApplyResult) string {
	if strings.TrimSpace(result.Action) == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("[ClawX Control Apply]\n")
	b.WriteString("action=")
	b.WriteString(result.Action)
	b.WriteString(" target=")
	b.WriteString(strings.TrimSpace(result.Target))
	b.WriteString(" status=")
	b.WriteString(strings.TrimSpace(result.Status))
	if msg := strings.TrimSpace(result.Message); msg != "" {
		b.WriteString("\n")
		b.WriteString(msg)
	}
	return b.String()
}
