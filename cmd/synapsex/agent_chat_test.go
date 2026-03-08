package main

import (
	"testing"

	chatiface "synapsex/internal/interfaces/chat"
)

func TestHandleAgentChatCommandUseAndCurrent(t *testing.T) {
	overrides := newConversationAgentOverrides()
	runtimes := map[string]agentRuntime{
		"main":     {agentID: "main"},
		"reviewer": {agentID: "reviewer"},
	}
	scopeKey := routingScopeKey("discord", "discord-main", "discord:-:123:u1")

	handled, _, err := handleAgentChatCommand(chatiface.Message{
		Text: "/agent use reviewer",
	}, scopeKey, overrides, runtimes, "main")
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

	handled, response, err := handleAgentChatCommand(chatiface.Message{
		Text: "/agent current",
	}, scopeKey, overrides, runtimes, "main")
	if err != nil {
		t.Fatalf("current command failed: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled")
	}
	if response == "" {
		t.Fatalf("expected current response")
	}
}

func TestHandleAgentChatCommandUnknownAgent(t *testing.T) {
	overrides := newConversationAgentOverrides()
	runtimes := map[string]agentRuntime{
		"main": {agentID: "main"},
	}
	scopeKey := routingScopeKey("telegram", "tg-main", "telegram:-:111:u1")

	handled, _, err := handleAgentChatCommand(chatiface.Message{
		Text: "/agent use missing",
	}, scopeKey, overrides, runtimes, "main")
	if !handled {
		t.Fatalf("expected handled")
	}
	if err == nil {
		t.Fatalf("expected unknown agent error")
	}
}
