package backend

import (
	"strings"
	"testing"

	"clawx/internal/domain/execution"
)

func TestComposeCodexExecutionInputIncludesWorkspaceGuardrails(t *testing.T) {
	input := composeCodexExecutionInput(execution.Request{
		CWD:   "/home/ubuntu/.clawx/workspaces/image_tools/.agents/main/workspace",
		Input: "implement /image commands",
	})
	if !strings.Contains(input, "Workspace Guardrails:") {
		t.Fatalf("missing workspace guardrails block")
	}
	if !strings.Contains(input, "Current workspace: /home/ubuntu/.clawx/workspaces/image_tools/.agents/main/workspace") {
		t.Fatalf("missing workspace path in guardrails")
	}
	if !strings.Contains(input, "implement /image commands") {
		t.Fatalf("missing original user input")
	}
}

func TestBuildCodexExecArgsEnforcesWorkspaceWriteAndCD(t *testing.T) {
	args := buildCodexExecArgs(
		[]string{"--foo", "bar"},
		"gpt-5-codex",
		"/home/ubuntu/.clawx/workspaces/image_tools/.agents/main/workspace",
		"/tmp/last.txt",
		"thread-123",
		"do task",
	)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--sandbox workspace-write") {
		t.Fatalf("missing workspace-write sandbox: %q", joined)
	}
	if !strings.Contains(joined, "--cd /home/ubuntu/.clawx/workspaces/image_tools/.agents/main/workspace") {
		t.Fatalf("missing --cd arg: %q", joined)
	}
	if !strings.Contains(joined, "resume thread-123") {
		t.Fatalf("missing resume thread id: %q", joined)
	}
}
