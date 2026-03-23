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
	if !strings.Contains(input, "[ControlPlan Response Contract]") {
		t.Fatalf("expected control plan response contract in input, got: %q", input)
	}
	if !strings.Contains(input, "\"type\": \"control_plan\"") {
		t.Fatalf("expected control plan schema in input, got: %q", input)
	}
	if !strings.Contains(input, "\"type\": \"requirement_sync\"") {
		t.Fatalf("expected requirement sync schema in input, got: %q", input)
	}
	if !strings.Contains(input, "\"agent_id\": \"可选；目标智能体 ID（从 agent_inventory 选择）\"") {
		t.Fatalf("expected requirement sync agent_id schema in input, got: %q", input)
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
}
