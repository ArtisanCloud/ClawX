package backend

import (
	"strings"
	"testing"

	"clawx/internal/domain/execution"
)

func TestComposeCodexExecutionInputIncludesWorkspaceGuardrails(t *testing.T) {
	input := composeCodexExecutionInput(execution.Request{
		CWD:          "/home/ubuntu/.clawx/workspaces/image_tools/.agents/main/workspace",
		AllowedRoots: []string{"/home/ubuntu/.clawx"},
		Input:        "implement /image commands",
	})
	if !strings.Contains(input, "Workspace Guardrails:") {
		t.Fatalf("missing workspace guardrails block")
	}
	if !strings.Contains(input, "Current workspace: /home/ubuntu/.clawx/workspaces/image_tools/.agents/main/workspace") {
		t.Fatalf("missing workspace path in guardrails")
	}
	if !strings.Contains(input, "Writable roots:") || !strings.Contains(input, "/home/ubuntu/.clawx") {
		t.Fatalf("missing writable roots in guardrails: %q", input)
	}
	if !strings.Contains(input, "under writable roots, execute directly without extra confirmation") {
		t.Fatalf("missing direct execute rule: %q", input)
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
		[]string{"/home/ubuntu/.clawx"},
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
	if !strings.Contains(joined, "--add-dir /home/ubuntu/.clawx") {
		t.Fatalf("missing --add-dir arg: %q", joined)
	}
	if !strings.Contains(joined, "resume thread-123") {
		t.Fatalf("missing resume thread id: %q", joined)
	}
}
