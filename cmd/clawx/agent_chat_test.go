package main

import (
	"strings"
	"testing"

	"clawx/internal/application/skillorchestrator"
	chatiface "clawx/internal/interfaces/chat"
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

func TestMaybeAutoApplyAgentSwitchFromModelOutput(t *testing.T) {
	overrides := newConversationAgentOverrides()
	runtimes := map[string]agentRuntime{
		"main":    {agentID: "main"},
		"bid-all": {agentID: "bid-all"},
	}
	scopeKey := routingScopeKey("discord", "discord-main", "discord:-:u1")
	audit := skillorchestrator.NewAuditService()
	applyMsg := `{
  "type": "control_plan",
  "intent": "agent.use",
  "target": {"agent_id": "bid-all"},
  "mode": "execute",
  "risk": "low",
  "reason": "用户请求切换智能体"
}`

	applied, ok, err := maybeAutoApplyAgentSwitch("我现在需要切换到智能体bid-all", applyMsg, "conv-1", "user-1", scopeKey, overrides, runtimes, "main", audit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected auto apply to run")
	}
	if applied.Action != "agent_use" || applied.Target != "bid-all" || applied.Status != "applied" {
		t.Fatalf("unexpected apply result: %#v", applied)
	}
	if applied.Message == "" {
		t.Fatalf("expected non-empty response")
	}
	gotAgent, exists := overrides.Get(scopeKey)
	if !exists || gotAgent != "bid-all" {
		t.Fatalf("unexpected override after auto apply: exists=%v agent=%q", exists, gotAgent)
	}
	records := audit.ListByConversation("conv-1", 20)
	if len(records) == 0 {
		t.Fatalf("expected control audit records")
	}
	assertHasControlPhase(t, records, "plan")
	assertHasControlPhase(t, records, "validate")
	assertHasControlPhase(t, records, "execute")
	assertHasControlPhase(t, records, "verify")
}

func TestMaybeAutoApplyAgentSwitchFromMarkdownCommand(t *testing.T) {
	overrides := newConversationAgentOverrides()
	runtimes := map[string]agentRuntime{
		"main":    {agentID: "main"},
		"bid-all": {agentID: "bid-all"},
	}
	scopeKey := routingScopeKey("discord", "discord-main", "discord:-:u1")
	audit := skillorchestrator.NewAuditService()
	applyMsg := "请执行：\n```json\n{\n  \"type\": \"control_plan\",\n  \"intent\": \"agent.use\",\n  \"target\": {\"agent_id\": \"bid-all\"},\n  \"mode\": \"execute\",\n  \"risk\": \"low\"\n}\n```"

	applied, ok, err := maybeAutoApplyAgentSwitch("我现在需要切换到智能体bid-all", applyMsg, "conv-1", "user-1", scopeKey, overrides, runtimes, "main", audit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected auto apply to run")
	}
	if applied.Target != "bid-all" || applied.Status != "applied" {
		t.Fatalf("unexpected apply result: %#v", applied)
	}
}

func TestMaybeAutoApplyAgentSwitchAppliesWhenPlanValidEvenIfUserTextNeutral(t *testing.T) {
	overrides := newConversationAgentOverrides()
	runtimes := map[string]agentRuntime{
		"main":    {agentID: "main"},
		"bid-all": {agentID: "bid-all"},
	}
	scopeKey := routingScopeKey("discord", "discord-main", "discord:-:u1")
	applied, ok, err := maybeAutoApplyAgentSwitch("现在有多少个智能体？", `{"type":"control_plan","intent":"agent.use","target":{"agent_id":"bid-all"},"mode":"execute","risk":"low"}`, "conv-1", "user-1", scopeKey, overrides, runtimes, "main", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected auto apply")
	}
	if applied.Target != "bid-all" || applied.Status != "applied" {
		t.Fatalf("unexpected apply result: %#v", applied)
	}
}

func TestMaybeAutoApplyAgentSwitchDoesNotParsePlainCommandText(t *testing.T) {
	overrides := newConversationAgentOverrides()
	runtimes := map[string]agentRuntime{
		"main":    {agentID: "main"},
		"bid-all": {agentID: "bid-all"},
	}
	scopeKey := routingScopeKey("discord", "discord-main", "discord:-:u1")
	_, ok, err := maybeAutoApplyAgentSwitch("我现在需要切换到智能体bid-all", "请执行 /agent use bid-all", "conv-1", "user-1", scopeKey, overrides, runtimes, "main", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("did not expect auto apply from plain text command")
	}
}

func TestMaybeAutoApplyAgentSwitchNoopStatus(t *testing.T) {
	overrides := newConversationAgentOverrides()
	runtimes := map[string]agentRuntime{
		"main": {agentID: "main"},
	}
	scopeKey := routingScopeKey("discord", "discord-main", "discord:-:u1")
	audit := skillorchestrator.NewAuditService()
	applyMsg := `{
  "type": "control_plan",
  "intent": "agent.use",
  "target": {"agent_id": "main"},
  "mode": "execute",
  "risk": "low"
}`
	applied, ok, err := maybeAutoApplyAgentSwitch("切换到智能体 main", applyMsg, "conv-2", "user-2", scopeKey, overrides, runtimes, "main", audit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected auto apply to run")
	}
	if applied.Status != "noop" {
		t.Fatalf("expected noop status, got: %#v", applied)
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
	handled, response, err := handleAgentChatCommand(chatiface.Message{
		Text: "/agent use bid-all",
	}, scopeKey, overrides, runtimes, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled")
	}
	if response == "" || !containsNoop(response) {
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
	if strings.Contains(text, "[ClawX Control Apply]") || strings.Contains(text, "action=agent_use") {
		t.Fatalf("unexpected formatted text: %q", text)
	}
	if !strings.Contains(text, "当前会话已切换到 Agent: bid-all") {
		t.Fatalf("unexpected formatted text: %q", text)
	}
}

func TestStripControlPlanPayload_JSONOnly(t *testing.T) {
	in := `{"type":"control_plan","intent":"agent.use","target":{"agent_id":"bid-all"},"mode":"execute","risk":"low"}`
	out := stripControlPlanPayload(in)
	if out != "" {
		t.Fatalf("expected empty after stripping pure json plan, got %q", out)
	}
}

func TestStripControlPlanPayload_FencedJSON(t *testing.T) {
	in := "```json\n{\"type\":\"control_plan\",\"intent\":\"agent.use\",\"target\":{\"agent_id\":\"bid-all\"},\"mode\":\"execute\",\"risk\":\"low\"}\n```\n\n已切换。"
	out := stripControlPlanPayload(in)
	if strings.Contains(out, "control_plan") {
		t.Fatalf("expected plan payload removed, got %q", out)
	}
	if !strings.Contains(out, "已切换") {
		t.Fatalf("expected normal text retained, got %q", out)
	}
}

func containsNoop(value string) bool {
	return strings.Contains(value, "无需切换")
}

func assertHasControlPhase(t *testing.T, records []skillorchestrator.AuditRecord, phase string) {
	t.Helper()
	for _, record := range records {
		if record.Source == "control_plan" && record.Phase == phase {
			return
		}
	}
	t.Fatalf("missing control audit phase %q", phase)
}
