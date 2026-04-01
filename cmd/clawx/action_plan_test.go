package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clawx/internal/application/service"
	"clawx/internal/infrastructure/config"
	chatiface "clawx/internal/interfaces/chat"
)

func TestParseActionPlanFromText(t *testing.T) {
	in := "```json\n{\"type\":\"action_plan\",\"mode\":\"execute\",\"actions\":[{\"kind\":\"runtime.exec\",\"cmd\":\"echo hi\"}]}\n```"
	plan, ok := parseActionPlanFromText(in)
	if !ok {
		t.Fatalf("expected action plan parse success")
	}
	if plan.Type != "action_plan" || plan.Mode != "execute" {
		t.Fatalf("unexpected parsed plan: %+v", plan)
	}
	if len(plan.Actions) != 1 || strings.TrimSpace(plan.Actions[0].Cmd) != "echo hi" {
		t.Fatalf("unexpected actions: %+v", plan.Actions)
	}
}

func TestStripActionPlanPayload(t *testing.T) {
	in := "```json\n{\"type\":\"action_plan\",\"mode\":\"execute\",\"actions\":[{\"kind\":\"runtime.exec\",\"cmd\":\"echo hi\"}]}\n```\n\n计划已生成。"
	out := stripActionPlanPayload(in)
	if strings.Contains(out, "\"type\":\"action_plan\"") {
		t.Fatalf("expected payload removed, got: %s", out)
	}
}

func TestMaybeAutoApplyActionPlanRuntimeExec(t *testing.T) {
	tmp := t.TempDir()
	runtime := agentRuntime{
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{tmp},
			DefaultCWD:   tmp,
		},
	}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-1",
		Message: chatiface.Message{
			Text:   "执行命令",
			UserID: "u1",
		},
	}
	output := `{"type":"action_plan","mode":"execute","actions":[{"kind":"runtime.exec","cmd":"echo hello","cwd":"` + tmp + `"}]}`
	result, ok, err := maybeAutoApplyActionPlan(context.Background(), runtime, decision, output, "discord", "default", "scope:discord:default:conv-1", newConversationAgentOverrides(), map[string]agentRuntime{"main": runtime}, "main", tmp)
	if err != nil {
		t.Fatalf("unexpected apply error: %v", err)
	}
	if !ok || !result.Applied {
		t.Fatalf("unexpected apply result: ok=%v result=%+v", ok, result)
	}
	if !strings.Contains(result.Message, "已执行完成") {
		t.Fatalf("expected concise runtime exec summary, got: %s", result.Message)
	}
}

func TestMaybeAutoApplyActionPlanSpecKitCompletenessGate(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "spec-kit", "source-keyword-monitoring")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "SPEC.md"), []byte("# SPEC\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "TASKS.md"), []byte("# TASKS\n"), 0o644); err != nil {
		t.Fatalf("write tasks: %v", err)
	}

	runtime := agentRuntime{
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{tmp},
			DefaultCWD:   tmp,
		},
	}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-spec-kit",
		Message: chatiface.Message{
			Text:   "请先用 Spec Kit 生成规范，不要直接写代码。",
			UserID: "u1",
		},
	}
	cmd := "printf '# SPEC\\n' > " + filepath.Join(specDir, "SPEC.md") + " && printf '# TASKS\\n' > " + filepath.Join(specDir, "TASKS.md")
	plan := actionPlan{
		Type: "action_plan",
		Mode: "execute",
		Actions: []actionPlanItem{
			{
				Kind: "runtime.exec",
				Cmd:  cmd,
				CWD:  tmp,
			},
		},
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	output := string(encoded)
	result, ok, err := maybeAutoApplyActionPlan(context.Background(), runtime, decision, output, "discord", "default", "scope:discord:default:conv-spec-kit", newConversationAgentOverrides(), map[string]agentRuntime{"main": runtime}, "main", tmp)
	if err != nil {
		t.Fatalf("unexpected apply error: %v", err)
	}
	if !ok {
		t.Fatalf("expected action plan handled")
	}
	if result.Status != "partial" {
		t.Fatalf("expected partial due to missing PLAN/ANALYZE, got: %s", result.Status)
	}
	if !strings.Contains(result.Message, "Spec Kit 文档未完整") {
		t.Fatalf("expected spec kit completeness warning, got: %s", result.Message)
	}
	if !strings.Contains(result.Message, "暂不进入实现阶段") {
		t.Fatalf("expected implementation blocked hint, got: %s", result.Message)
	}
}

func TestMaybeAutoApplyActionPlanSpecKitCompletenessGateFromCommandPath(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "spec-kit", "source-keyword-monitoring")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}

	runtime := agentRuntime{
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{tmp},
			DefaultCWD:   tmp,
		},
	}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-spec-kit-path",
		Message: chatiface.Message{
			Text:   "请先用 Spec Kit 生成规范，不要直接写代码。",
			UserID: "u1",
		},
	}
	cmd := "mkdir -p docs/spec-kit/source-keyword-monitoring && printf '# SPEC\\n' > docs/spec-kit/source-keyword-monitoring/SPEC.md"
	plan := actionPlan{
		Type: "action_plan",
		Mode: "execute",
		Actions: []actionPlanItem{
			{
				Kind: "runtime.exec",
				Cmd:  cmd,
				CWD:  tmp,
			},
		},
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	result, ok, err := maybeAutoApplyActionPlan(context.Background(), runtime, decision, string(encoded), "discord", "default", "scope:discord:default:conv-spec-kit-path", newConversationAgentOverrides(), map[string]agentRuntime{"main": runtime}, "main", tmp)
	if err != nil {
		t.Fatalf("unexpected apply error: %v", err)
	}
	if !ok {
		t.Fatalf("expected action plan handled")
	}
	if result.Status != "partial" {
		t.Fatalf("expected partial due to missing PLAN/TASKS/ANALYZE, got: %s", result.Status)
	}
	if !strings.Contains(result.Message, "Spec Kit 文档未完整") {
		t.Fatalf("expected spec gate warning, got: %s", result.Message)
	}
}

func TestMaybeAutoApplyActionPlanRuntimeBootstrap(t *testing.T) {
	tmp := t.TempDir()
	runtime := agentRuntime{
		agentID: "bid-all",
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{tmp},
			DefaultCWD:   tmp,
		},
		cwd: tmp,
	}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-bootstrap",
		Message: chatiface.Message{
			Text:   "启动",
			UserID: "u1",
		},
	}
	output := `{"type":"action_plan","mode":"execute","actions":[{"kind":"runtime.bootstrap","cwd":"` + tmp + `","worker_roles":["planner","executor","reviewer"]}]}`
	result, ok, err := maybeAutoApplyActionPlan(context.Background(), runtime, decision, output, "discord", "default", "scope:discord:default:conv-bootstrap", newConversationAgentOverrides(), map[string]agentRuntime{"bid-all": runtime}, "bid-all", tmp)
	if err != nil {
		t.Fatalf("unexpected apply error: %v", err)
	}
	if !ok || !result.Applied {
		t.Fatalf("expected applied bootstrap result: ok=%v result=%+v", ok, result)
	}
	runtimeDir := filepath.Join(tmp, ".clawx", "runtime")
	for _, p := range []string{
		runtimeDir,
		filepath.Join(runtimeDir, "tasks.jsonl"),
		filepath.Join(runtimeDir, "worker_states.json"),
		filepath.Join(runtimeDir, "heartbeats.json"),
		filepath.Join(runtimeDir, "runtime_meta.json"),
		filepath.Join(runtimeDir, "dispatch_state.json"),
	} {
		if _, statErr := os.Stat(p); statErr != nil {
			t.Fatalf("expected runtime bootstrap artifact: %s err=%v", p, statErr)
		}
	}
}

func TestMaybeAutoApplyActionPlanAgentUse(t *testing.T) {
	runtime := agentRuntime{
		agentID: "main",
	}
	runtimes := map[string]agentRuntime{
		"main":    runtime,
		"bid-all": {agentID: "bid-all"},
	}
	overrides := newConversationAgentOverrides()
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-2",
		Message: chatiface.Message{
			Text:   "切换到 bid-all",
			UserID: "u2",
		},
	}
	output := `{"type":"action_plan","mode":"execute","actions":[{"kind":"agent.use","agent_id":"bid-all"}]}`
	result, ok, err := maybeAutoApplyActionPlan(context.Background(), runtime, decision, output, "discord", "default", "scope:discord:default:conv-2", overrides, runtimes, "main", ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || !result.Applied {
		t.Fatalf("expected applied action plan: ok=%v result=%+v", ok, result)
	}
	if strings.TrimSpace(result.ResponseAgentID) != "bid-all" {
		t.Fatalf("expected response agent id bid-all, got: %s", result.ResponseAgentID)
	}
}

func TestMaybeAutoApplyActionPlanConfigExec(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	homeDir := filepath.Join(tempDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home dir: %v", err)
	}
	t.Setenv("CLAWX_CONFIG", configPath)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAWX_CONFIG_ADMIN_USERS", "u3")
	clearPendingConfigPlans()

	runtime := agentRuntime{
		agentID: "main",
		cfgSnapshot: config.Snapshot{
			AllowedRoots: []string{tempDir},
			DefaultCWD:   tempDir,
		},
	}
	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: "conv-3",
		Message: chatiface.Message{
			Text:   "创建并应用配置",
			UserID: "u3",
			ContextFlags: chatiface.ContextFlags{
				IsAllowed: true,
			},
		},
	}
	output := `{"type":"action_plan","mode":"execute","actions":[{"kind":"config.exec","command":"/config plan 创建 agent review 使用 codex"},{"kind":"config.exec","command":"/config apply"}]}`
	result, ok, err := maybeAutoApplyActionPlan(context.Background(), runtime, decision, output, "discord", "default", "scope:discord:default:conv-3", newConversationAgentOverrides(), map[string]agentRuntime{"main": runtime}, "main", tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || !result.Applied {
		t.Fatalf("expected config action plan applied: ok=%v result=%+v", ok, result)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if _, exists := cfg.Agents["review"]; !exists {
		t.Fatalf("expected review agent after config.apply")
	}
}
