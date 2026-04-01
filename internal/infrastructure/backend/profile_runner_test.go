package backend

import (
	"strings"
	"testing"
	"unicode/utf8"

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
		"clawx:nl:execute:main:image_tools:route_1",
		"in_memory",
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
	if !strings.Contains(joined, `-c prompt_cache_key="clawx:nl:execute:main:image_tools:route_1"`) {
		t.Fatalf("missing prompt_cache_key arg: %q", joined)
	}
	if !strings.Contains(joined, `-c prompt_cache_retention="in_memory"`) {
		t.Fatalf("missing prompt_cache_retention arg: %q", joined)
	}
	if !strings.Contains(joined, "resume thread-123") {
		t.Fatalf("missing resume thread id: %q", joined)
	}
}

func TestParseCodexPromptUsage(t *testing.T) {
	raw := strings.Join([]string{
		`{"type":"thread.started","thread_id":"thread-1"}`,
		`{"type":"response.completed","response":{"usage":{"prompt_tokens":2048,"completion_tokens":512,"total_tokens":2560,"prompt_tokens_details":{"cached_tokens":1024}}}}`,
	}, "\n")
	cached, prompt, completion, total := parseCodexPromptUsage(raw)
	if cached != 1024 {
		t.Fatalf("unexpected cached tokens: %d", cached)
	}
	if prompt != 2048 {
		t.Fatalf("unexpected prompt tokens: %d", prompt)
	}
	if completion != 512 {
		t.Fatalf("unexpected completion tokens: %d", completion)
	}
	if total != 2560 {
		t.Fatalf("unexpected total tokens: %d", total)
	}
}

func TestBuildCodexExecArgsSanitizesInvalidUTF8(t *testing.T) {
	invalid := string([]byte{'o', 'k', 0xff, 'x'})
	args := buildCodexExecArgs(
		nil,
		"gpt-5-codex",
		"/tmp/work",
		[]string{"/tmp/work"},
		"cache-key",
		"in_memory",
		"/tmp/out.txt",
		"thread-1",
		invalid,
	)
	for i, arg := range args {
		if !utf8.ValidString(arg) {
			t.Fatalf("arg[%d] is not valid utf8: %q", i, arg)
		}
	}
}
