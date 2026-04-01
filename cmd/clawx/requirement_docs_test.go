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
	required := []string{"AGENTS.md", "BOOTSTRAP.md", "HEARTBEAT.md", "IDENTITY.md", "SOUL.md", "TOOLS.md", "USER.md"}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("missing scaffold file %s: %v", name, err)
		}
	}
}

func TestMaybeSyncRequirementDocs_AppendsAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	rt := agentRuntime{agentID: "bid-all", cwd: root}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-1",
		Message:        chat.Message{Text: "请更新项目需求：增加招投标抓取去重和验收标准"},
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
	if !strings.Contains(mustReadFile(t, filepath.Join(root, "AGENTS.md")), "## Requirement Sync") {
		t.Fatalf("AGENTS.md missing Requirement Sync section")
	}
	if !strings.Contains(mustReadFile(t, filepath.Join(root, "HEARTBEAT.md")), "requirement_synced_by=bid-all") {
		t.Fatalf("HEARTBEAT.md missing sync record")
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

func TestFormatRequirementSyncResult_IncludesTargetAgent(t *testing.T) {
	out := formatRequirementSyncResult(requirementSyncResult{
		Status:  "suggest",
		Source:  "action_plan",
		AgentID: "bid-all",
		Message: "建议先确认是否切换到 bid-all",
	})
	if !strings.Contains(out, "bid-all") || !strings.Contains(out, "建议先确认目标智能体") {
		t.Fatalf("expected concise suggest output, got: %s", out)
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
				"bid-all": {ID: "bid-all", Workspace: agentWorkspace},
			},
		},
	}
	decision := service.Decision{Kind: service.DecisionExecute, ProjectID: "image_tools"}
	got, source := resolveRequirementWorkspace(context.Background(), rt, decision)
	if got != agentWorkspace {
		t.Fatalf("expected agent config workspace, got %q", got)
	}
	if source != "agent_config" {
		t.Fatalf("expected agent_config source, got %q", source)
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
