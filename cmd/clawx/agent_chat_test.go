package main

import (
	"strings"
	"testing"

	chatiface "clawx/internal/interfaces/chat"
)

func TestHandleAgentChatCommandUseAndCurrent(t *testing.T) {
	overrides := newConversationAgentOverrides()
	runtimes := map[string]agentRuntime{
		"main":     {agentID: "main"},
		"reviewer": {agentID: "reviewer"},
	}
	scopeKey := routingScopeKey("discord", "discord-main", "discord:-:123:u1")

	handled, _, err := handleAgentChatCommand(chatiface.Message{Text: "/agent use reviewer"}, scopeKey, overrides, runtimes, "main")
	if err != nil {
		t.Fatalf("use command failed: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled")
	}

	gotAgent, ok := overrides.Get(scopeKey)
	if !ok || gotAgent != "reviewer" {
		t.Fatalf("unexpected override: ok=%v agent=%q", ok, gotAgent)
	}

	handled, response, err := handleAgentChatCommand(chatiface.Message{Text: "/agent current"}, scopeKey, overrides, runtimes, "main")
	if err != nil {
		t.Fatalf("current command failed: %v", err)
	}
	if !handled || strings.TrimSpace(response) == "" {
		t.Fatalf("expected non-empty current response")
	}
}

func TestHandleAgentChatCommandUnknownAgent(t *testing.T) {
	overrides := newConversationAgentOverrides()
	runtimes := map[string]agentRuntime{"main": {agentID: "main"}}
	scopeKey := routingScopeKey("telegram", "tg-main", "telegram:-:111:u1")

	handled, _, err := handleAgentChatCommand(chatiface.Message{Text: "/agent use missing"}, scopeKey, overrides, runtimes, "main")
	if !handled {
		t.Fatalf("expected handled")
	}
	if err == nil {
		t.Fatalf("expected unknown agent error")
	}
}

func TestHandleAgentChatCommandUseNoopWhenAlreadyCurrent(t *testing.T) {
	overrides := newConversationAgentOverrides()
	runtimes := map[string]agentRuntime{
		"main":    {agentID: "main"},
		"bid-all": {agentID: "bid-all"},
	}
	scopeKey := routingScopeKey("discord", "discord-main", "discord:-:u1")
	overrides.Set(scopeKey, "bid-all")

	handled, response, err := handleAgentChatCommand(chatiface.Message{Text: "/agent use bid-all"}, scopeKey, overrides, runtimes, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled")
	}
	if response == "" || !strings.Contains(response, "无需切换") {
		t.Fatalf("expected noop response, got: %q", response)
	}
}

func TestFormatControlApplyResult(t *testing.T) {
	text := formatControlApplyResult(controlApplyResult{
		Action:  "agent_use",
		Target:  "bid-all",
		Status:  "applied",
		Message: "当前会话已切换到 Agent: bid-all",
	})
	if text == "" {
		t.Fatalf("expected formatted text")
	}
	if strings.Contains(text, "action=agent_use") {
		t.Fatalf("unexpected formatted text: %q", text)
	}
	if !strings.Contains(text, "当前会话已切换到 Agent: bid-all") {
		t.Fatalf("unexpected formatted text: %q", text)
	}
}
