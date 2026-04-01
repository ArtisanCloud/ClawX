package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clawx/internal/application/service"
	chatiface "clawx/internal/interfaces/chat"
)

func TestBuildExecutionInputIncludesAttachmentsForDirectExecution(t *testing.T) {
	decision := service.Decision{
		Kind: service.DecisionExecute,
		Message: chatiface.Message{
			Text: "转换成长图",
			Attachments: []chatiface.Attachment{
				{
					Name:        "test.pdf",
					URL:         "https://cdn.discordapp.com/test.pdf",
					LocalPath:   "/home/ubuntu/.clawx/workspaces/image_tools/.agents/main/context/attachments/files/abc/test.pdf",
					ContentType: "application/pdf",
					SizeBytes:   9195520,
				},
			},
		},
	}

	input := buildExecutionInput(decision, agentRuntime{})
	if !strings.Contains(input, "[Attachments]") {
		t.Fatalf("expected attachments section, got: %q", input)
	}
	if !strings.Contains(input, "test.pdf") {
		t.Fatalf("expected attachment name in input, got: %q", input)
	}
	if !strings.Contains(input, "application/pdf") {
		t.Fatalf("expected content type in input, got: %q", input)
	}
	if !strings.Contains(input, "local_path=") {
		t.Fatalf("expected local path in input, got: %q", input)
	}
}

func TestBuildExecutionInputInjectsAgentInventoryContext(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stateDir := filepath.Join(home, ".clawx")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	configBody := `{
  "providers": {
    "profiles": {
      "codex": {
        "kind": "codex-cli",
        "command": "codex"
      }
    }
  },
  "agents": {
    "default": "main",
    "list": [
      {"id":"main","profile":"codex","workspace":"/home/ubuntu/.clawx/workspaces/main","timeoutSeconds":600,"default":true},
      {"id":"local-smoke","profile":"codex","workspace":"/home/ubuntu/.clawx/workspaces/local-smoke","timeoutSeconds":600},
      {"id":"bid-all智能体","profile":"codex","workspace":"/home/ubuntu/.clawx/workspaces/bid-all智能体","timeoutSeconds":600}
    ]
  }
}`
	if err := os.WriteFile(filepath.Join(stateDir, "config.json"), []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	decision := service.Decision{
		Kind: service.DecisionExecute,
		Message: chatiface.Message{
			Text: "现在我们有多少个智能体？",
		},
	}
	input := buildExecutionInput(decision, agentRuntime{agentID: "main"})
	if !strings.Contains(input, "tool.agent_inventory") {
		t.Fatalf("expected agent inventory tool context, got: %q", input)
	}
	if !strings.Contains(input, "total_registered_agents=3") {
		t.Fatalf("expected total agent count in context, got: %q", input)
	}
	if !strings.Contains(input, "id=bid-all智能体") {
		t.Fatalf("expected bid-all智能体 listed in context, got: %q", input)
	}
	if !strings.Contains(input, "[Staged Routing Snapshot]") {
		t.Fatalf("expected staged routing snapshot in input, got: %q", input)
	}
	if !strings.Contains(input, "route.use_route_planner=") {
		t.Fatalf("expected staged routing plan fields in input, got: %q", input)
	}
	if !strings.Contains(input, "fallback.enabled=") {
		t.Fatalf("expected staged fallback fields in input, got: %q", input)
	}
	if !strings.Contains(input, "execution.can_execute=") {
		t.Fatalf("expected staged execution gate field in input, got: %q", input)
	}
	if !strings.Contains(input, "[Autonomous Recovery Playbook]") {
		t.Fatalf("expected autonomous recovery playbook in input, got: %q", input)
	}
	if !strings.Contains(input, "禁止第一轮直接向用户索要环境参数/镜像/离线包") {
		t.Fatalf("expected autonomy-first recovery rule in input, got: %q", input)
	}
	if !strings.Contains(input, "[Execution Blocker Contract]") {
		t.Fatalf("expected execution blocker contract in input, got: %q", input)
	}
	if !strings.Contains(input, "\"type\": \"execution_blocker\"") {
		t.Fatalf("expected execution blocker schema in input, got: %q", input)
	}
	if !strings.Contains(input, "\"evidence_exec_ids\"") {
		t.Fatalf("expected execution blocker attestation ids in input, got: %q", input)
	}
	if !strings.Contains(input, "[Unified Action Plan Contract]") {
		t.Fatalf("expected unified action plan contract in input, got: %q", input)
	}
	if !strings.Contains(input, "\"type\": \"action_plan\"") {
		t.Fatalf("expected action plan schema in input, got: %q", input)
	}
	if !strings.Contains(input, "旧版 control_plan / requirement_sync / runtime_exec_plan 已废弃") {
		t.Fatalf("expected deprecated protocol warning in input, got: %q", input)
	}
}

func TestBuildExecutionInputIncludesContinuationSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".clawx"), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}

	convID := "discord:-:c1:u1|ch=discord|inst=default|agent=bid-all"
	if err := appendRuntimeExecAttestation(runtimeExecAttestationRecord{
		ExecID:         "rexec-test-1",
		ConversationID: convID,
		AgentID:        "bid-all",
		CWD:            "/home/ubuntu/.clawx/workspaces/bid-all",
		Command:        "python3.11 -m venv .venv311",
		Success:        true,
		OutputPreview:  "ok",
	}); err != nil {
		t.Fatalf("append attestation #1: %v", err)
	}
	if err := appendRuntimeExecAttestation(runtimeExecAttestationRecord{
		ExecID:         "rexec-test-2",
		ConversationID: convID,
		AgentID:        "bid-all",
		CWD:            "/home/ubuntu/.clawx/workspaces/bid-all",
		Command:        "source .venv311/bin/activate && pip install -e . --no-build-isolation",
		Success:        false,
		OutputPreview:  "build backend failed",
		ErrorSummary:   "exit status 1",
	}); err != nil {
		t.Fatalf("append attestation #2: %v", err)
	}

	decision := service.Decision{
		Kind:           service.DecisionExecute,
		ConversationID: convID,
		Message: chatiface.Message{
			Text: "继续",
		},
	}
	input := buildExecutionInput(decision, agentRuntime{agentID: "bid-all"})
	if !strings.Contains(input, "[Execution Continuation Snapshot]") {
		t.Fatalf("expected continuation snapshot, got: %q", input)
	}
	if !strings.Contains(input, "continuation_hint=advance_from_recent_runtime_exec") {
		t.Fatalf("expected continuation hint, got: %q", input)
	}
	if !strings.Contains(input, "不要重复最近已成功命令") {
		t.Fatalf("expected continuation rule in input, got: %q", input)
	}
}
