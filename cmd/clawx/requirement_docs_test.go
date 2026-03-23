package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clawx/internal/application/service"
	"clawx/internal/infrastructure/config"
	"clawx/internal/interfaces/chat"
)

func TestEnsureWorkspaceDocsScaffold(t *testing.T) {
	root := filepath.Join(t.TempDir(), "agent-workspace")
	if err := ensureWorkspaceDocsScaffold(root); err != nil {
		t.Fatalf("ensureWorkspaceDocsScaffold: %v", err)
	}
	required := []string{
		"AGENTS.md",
		"BOOTSTRAP.md",
		"HEARTBEAT.md",
		"IDENTITY.md",
		"SOUL.md",
		"TOOLS.md",
		"USER.md",
	}
	for _, name := range required {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing scaffold file %s: %v", name, err)
		}
	}
}

func TestMaybeSyncRequirementDocs_AppendsAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	rt := agentRuntime{
		agentID: "bid-all",
		cwd:     root,
	}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-1",
		Message: chat.Message{
			Text: "请更新项目需求：增加招投标抓取去重和验收标准",
		},
	}
	ctx := context.Background()
	if err := maybeSyncRequirementDocs(ctx, rt, decision); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}
	if err := maybeSyncRequirementDocs(ctx, rt, decision); err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	userBody := mustReadFile(t, filepath.Join(root, "USER.md"))
	if count := strings.Count(userBody, "增加招投标抓取去重和验收标准"); count != 1 {
		t.Fatalf("expected deduped requirement entry once, got %d", count)
	}
	agentBody := mustReadFile(t, filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(agentBody, "## Requirement Sync") {
		t.Fatalf("AGENTS.md missing Requirement Sync section")
	}
	heartbeatBody := mustReadFile(t, filepath.Join(root, "HEARTBEAT.md"))
	if !strings.Contains(heartbeatBody, "requirement_synced_by=bid-all") {
		t.Fatalf("HEARTBEAT.md missing sync record")
	}
	identityBody := mustReadFile(t, filepath.Join(root, "IDENTITY.md"))
	if !strings.Contains(identityBody, "## Mission Updates") {
		t.Fatalf("IDENTITY.md missing mission updates section")
	}
	soulBody := mustReadFile(t, filepath.Join(root, "SOUL.md"))
	if !strings.Contains(soulBody, "## Constraints & Principles") {
		t.Fatalf("SOUL.md missing principles section")
	}
	toolsBody := mustReadFile(t, filepath.Join(root, "TOOLS.md"))
	if !strings.Contains(toolsBody, "## Capability Targets") {
		t.Fatalf("TOOLS.md missing capability targets section")
	}
	bootstrapBody := mustReadFile(t, filepath.Join(root, "BOOTSTRAP.md"))
	if !strings.Contains(bootstrapBody, "## Requirement Intake") {
		t.Fatalf("BOOTSTRAP.md missing requirement intake section")
	}
}

func TestLooksLikeRequirementUpdate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{name: "chinese keyword", in: "这个项目需求要新增验收标准", want: true},
		{name: "english keyword", in: "please refine requirement and milestone", want: true},
		{name: "plain question", in: "现在有多少个智能体", want: false},
	}
	for _, tc := range cases {
		if got := looksLikeRequirementUpdate(tc.in); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestMaybeAutoApplyRequirementSyncFromModel(t *testing.T) {
	root := t.TempDir()
	rt := agentRuntime{
		agentID: "bid-all",
		cwd:     root,
	}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-2",
		Message: chat.Message{
			Text: "请帮我更新项目需求",
		},
	}
	modelOutput := "```json\n{\"type\":\"requirement_sync\",\"intent\":\"requirement.update\",\"mode\":\"execute\",\"requirement\":\"新增公告抓取频率配置\"}\n```"
	result, applied, err := maybeAutoApplyRequirementSyncFromModel(
		context.Background(),
		rt,
		decision,
		modelOutput,
		"discord",
		"main",
		"scope:discord:default:conv",
		newConversationAgentOverrides(),
		map[string]agentRuntime{"bid-all": rt},
		"bid-all",
	)
	if err != nil {
		t.Fatalf("auto apply failed: %v", err)
	}
	if !applied {
		t.Fatalf("expected requirement sync to be applied")
	}
	if result.Source != "llm_plan" {
		t.Fatalf("unexpected source: %s", result.Source)
	}
	if strings.TrimSpace(result.Workspace) != root {
		t.Fatalf("unexpected workspace: %q", result.Workspace)
	}
	userBody := mustReadFile(t, filepath.Join(root, "USER.md"))
	if !strings.Contains(userBody, "新增公告抓取频率配置") {
		t.Fatalf("USER.md missing requirement from model plan")
	}
}

func TestMaybeAutoApplyRequirementSyncFromModel_NoWorkspace(t *testing.T) {
	rt := agentRuntime{
		agentID: "bid-all",
	}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-3",
		Message: chat.Message{
			Text: "请帮我更新项目需求",
		},
	}
	modelOutput := `{"type":"requirement_sync","intent":"requirement.update","mode":"execute","requirement":"补充招标来源站点"}`
	result, applied, err := maybeAutoApplyRequirementSyncFromModel(
		context.Background(),
		rt,
		decision,
		modelOutput,
		"discord",
		"main",
		"scope:discord:default:conv",
		newConversationAgentOverrides(),
		map[string]agentRuntime{"bid-all": rt},
		"bid-all",
	)
	if err != nil {
		t.Fatalf("auto apply failed: %v", err)
	}
	if !applied {
		t.Fatalf("expected requirement sync to be handled")
	}
	if result.Status != "skipped" {
		t.Fatalf("expected skipped status, got %s", result.Status)
	}
}

func TestStripRequirementSyncPlanPayload(t *testing.T) {
	in := "```json\n{\"type\":\"requirement_sync\",\"intent\":\"requirement.update\",\"mode\":\"execute\",\"requirement\":\"x\"}\n```\n\n已记录需求。"
	out := stripRequirementSyncPlanPayload(in)
	if strings.Contains(out, "requirement_sync") {
		t.Fatalf("expected plan payload removed, got %q", out)
	}
	if !strings.Contains(out, "已记录需求") {
		t.Fatalf("expected normal content retained, got %q", out)
	}
}

func TestFormatRequirementSyncResult_IncludesTargetAgent(t *testing.T) {
	out := formatRequirementSyncResult(requirementSyncResult{
		Status:  "suggest",
		Source:  "llm_plan",
		AgentID: "bid-all",
		Message: "建议先确认是否切换到 bid-all",
	})
	if !strings.Contains(out, "bid-all") || !strings.Contains(out, "建议先确认目标智能体") {
		t.Fatalf("expected concise suggest output, got: %s", out)
	}
}

func TestMaybeAutoApplyRequirementSyncFromModel_NoSwitchNoiseWhenSameAgent(t *testing.T) {
	root := t.TempDir()
	rt := agentRuntime{agentID: "bid-all", cwd: root}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-same-agent",
		Message: chat.Message{
			Text: "更新需求",
		},
	}
	modelOutput := `{"type":"requirement_sync","intent":"requirement.update","mode":"execute","agent_id":"bid-all","requirement":"补充抓取频率"}`
	result, applied, err := maybeAutoApplyRequirementSyncFromModel(
		context.Background(),
		rt,
		decision,
		modelOutput,
		"discord",
		"default",
		routingScopeKey("discord", "default", decision.ConversationID),
		newConversationAgentOverrides(),
		map[string]agentRuntime{"bid-all": rt},
		"bid-all",
	)
	if err != nil {
		t.Fatalf("auto apply failed: %v", err)
	}
	if !applied || result.Status != "applied" {
		t.Fatalf("expected applied result, got applied=%v status=%q", applied, result.Status)
	}
	if strings.Contains(result.Message, "无需切换") {
		t.Fatalf("unexpected switch noise in message: %s", result.Message)
	}
}

func TestResolveRequirementWorkspacePrefersAgentConfigWorkspace(t *testing.T) {
	root := t.TempDir()
	agentWorkspace := filepath.Join(root, "agent-main")
	if err := os.MkdirAll(agentWorkspace, 0o755); err != nil {
		t.Fatalf("mkdir agent workspace: %v", err)
	}
	rt := agentRuntime{
		agentID: "bid-all",
		cwd:     filepath.Join(root, "runtime-cwd"),
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{root},
			Agents: map[string]config.Agent{
				"bid-all": {
					ID:        "bid-all",
					Workspace: agentWorkspace,
				},
			},
		},
	}
	decision := service.Decision{
		Kind:      service.DecisionExecute,
		ProjectID: "image_tools",
	}
	got, source := resolveRequirementWorkspace(context.Background(), rt, decision)
	if got != agentWorkspace {
		t.Fatalf("expected agent config workspace, got %q", got)
	}
	if source != "agent_config" {
		t.Fatalf("expected agent_config source, got %q", source)
	}
}

func TestMaybeAutoApplyRequirementSyncFromModel_WithTargetAgentSwitch(t *testing.T) {
	mainRoot := t.TempDir()
	bidRoot := t.TempDir()
	runtimes := map[string]agentRuntime{
		"main": {
			agentID: "main",
			cwd:     mainRoot,
		},
		"bid-all": {
			agentID: "bid-all",
			cwd:     bidRoot,
		},
	}
	overrides := newConversationAgentOverrides()
	scopeKey := routingScopeKey("discord", "default", "conv-9")
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-9",
		Message: chat.Message{
			Text: "更新 bid-all 的需求",
		},
	}
	modelOutput := `{"type":"requirement_sync","intent":"requirement.update","mode":"execute","agent_id":"bid-all","requirement":"新增按公司查询功能"}`
	result, applied, err := maybeAutoApplyRequirementSyncFromModel(
		context.Background(),
		runtimes["main"],
		decision,
		modelOutput,
		"discord",
		"default",
		scopeKey,
		overrides,
		runtimes,
		"main",
	)
	if err != nil {
		t.Fatalf("auto apply failed: %v", err)
	}
	if !applied {
		t.Fatalf("expected requirement sync to be applied")
	}
	if result.AgentID != "bid-all" {
		t.Fatalf("expected result agent bid-all, got %q", result.AgentID)
	}
	current, ok := overrides.Get(scopeKey)
	if !ok || current != "bid-all" {
		t.Fatalf("expected session switched to bid-all, got ok=%v current=%q", ok, current)
	}
	userBody := mustReadFile(t, filepath.Join(bidRoot, "USER.md"))
	if !strings.Contains(userBody, "新增按公司查询功能") {
		t.Fatalf("expected requirement written to bid-all workspace")
	}
}

func TestMaybeAutoApplyRequirementSyncFromModel_ModeSuggest(t *testing.T) {
	mainRoot := t.TempDir()
	bidRoot := t.TempDir()
	runtimes := map[string]agentRuntime{
		"main": {
			agentID: "main",
			cwd:     mainRoot,
		},
		"bid-all": {
			agentID: "bid-all",
			cwd:     bidRoot,
		},
		"local-smoke": {
			agentID: "local-smoke",
			cwd:     t.TempDir(),
		},
	}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-suggest-mode",
		Message: chat.Message{
			Text: "请确认是否更新需求",
		},
	}
	modelOutput := `{"type":"requirement_sync","intent":"requirement.update","mode":"suggest","agent_id":"bid-all","requirement":"建设一个招标信息助手，支持抓取与通知","reason":"目标需求更适合写入 bid-all 智能体"}`
	result, applied, err := maybeAutoApplyRequirementSyncFromModel(
		context.Background(),
		runtimes["main"],
		decision,
		modelOutput,
		"discord",
		"default",
		routingScopeKey("discord", "default", decision.ConversationID),
		newConversationAgentOverrides(),
		runtimes,
		"main",
	)
	if err != nil {
		t.Fatalf("auto apply failed: %v", err)
	}
	if !applied {
		t.Fatalf("expected requirement sync to be handled")
	}
	if result.Status != "suggest" {
		t.Fatalf("expected suggest status, got %q", result.Status)
	}
	if !strings.Contains(result.Message, "bid-all") || !strings.Contains(result.Message, "更适合") {
		t.Fatalf("expected suggest reason in message, got: %s", result.Message)
	}
	if _, err := os.Stat(filepath.Join(mainRoot, "USER.md")); !os.IsNotExist(err) {
		t.Fatalf("main workspace should not be written on suggest mode")
	}
	if _, err := os.Stat(filepath.Join(bidRoot, "USER.md")); !os.IsNotExist(err) {
		t.Fatalf("target workspace should not be written on suggest mode")
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}
