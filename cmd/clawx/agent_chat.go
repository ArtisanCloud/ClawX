package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	chatiface "clawx/internal/interfaces/chat"
)

type conversationAgentOverrides struct {
	mu              sync.RWMutex
	byScope         map[string]string
	persistencePath string
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
	_ = o.persistLocked()
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
		_ = o.persistLocked()
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

func cleanAgentIDToken(raw string) string {
	return strings.Trim(strings.TrimSpace(raw), "`'\"，,。.!！?？:：;；)）]】")
}

func formatControlApplyResult(result controlApplyResult) string {
	if strings.TrimSpace(result.Action) == "" {
		return ""
	}
	if msg := strings.TrimSpace(result.Message); msg != "" {
		return msg
	}
	status := strings.TrimSpace(result.Status)
	target := strings.TrimSpace(result.Target)
	if target == "" {
		target = "unknown"
	}
	if status == "noop" {
		return "当前会话已是目标智能体，无需切换。"
	}
	return "当前会话已切换到 Agent: " + target
}
