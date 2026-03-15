package contract

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	memoryapp "clawx/internal/application/memory"
	memorydomain "clawx/internal/domain/memory"
)

func TestMemoryAgentACLContractRejectsCrossAgentAndEscape(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "workspaces", "memo")
	if err := os.MkdirAll(filepath.Join(projectRoot, ".agents", "agent-a"), 0o755); err != nil {
		t.Fatalf("mkdir agent-a: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, ".agents", "agent-b"), 0o755); err != nil {
		t.Fatalf("mkdir agent-b: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".agents", "agent-b", "MEMORY.md"), []byte("B"), 0o644); err != nil {
		t.Fatalf("write agent-b memory: %v", err)
	}

	guard, err := memoryapp.NewPathGuard(projectRoot)
	if err != nil {
		t.Fatalf("new path guard: %v", err)
	}
	scope := memorydomain.MemoryScopeKey{
		AgentID:   "agent-a",
		ProjectID: "memo",
		RouteKey:  "telegram:default:direct:user-1",
		ChatMode:  memorydomain.ChatModeMain,
	}

	crossAgentPath := filepath.Join(projectRoot, ".agents", "agent-b", "MEMORY.md")
	err = guard.ValidateCandidate(scope, memorydomain.LayerAgentPrivate, crossAgentPath)
	if !errors.Is(err, memoryapp.ErrCrossAgentAccess) {
		t.Fatalf("expected ErrCrossAgentAccess, got %v", err)
	}

	_, err = guard.ResolveAgentPrivatePath(scope, filepath.ToSlash(filepath.Join("..", "agent-b", "MEMORY.md")))
	if !errors.Is(err, memoryapp.ErrPathEscape) {
		t.Fatalf("expected ErrPathEscape, got %v", err)
	}
}

func TestMemoryAgentACLContractRejectsCrossProjectSymlink(t *testing.T) {
	tempDir := t.TempDir()
	projectA := filepath.Join(tempDir, "workspaces", "project-a")
	projectB := filepath.Join(tempDir, "workspaces", "project-b")
	if err := os.MkdirAll(filepath.Join(projectA, ".agents", "agent-a"), 0o755); err != nil {
		t.Fatalf("mkdir projectA: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectB, ".agents", "agent-a"), 0o755); err != nil {
		t.Fatalf("mkdir projectB: %v", err)
	}
	projectBFile := filepath.Join(projectB, ".agents", "agent-a", "MEMORY.md")
	if err := os.WriteFile(projectBFile, []byte("BETA"), 0o644); err != nil {
		t.Fatalf("write projectB memory: %v", err)
	}
	projectALink := filepath.Join(projectA, ".agents", "agent-a", "MEMORY.md")
	if err := os.Symlink(projectBFile, projectALink); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	guard, err := memoryapp.NewPathGuard(projectA)
	if err != nil {
		t.Fatalf("new path guard: %v", err)
	}
	scope := memorydomain.MemoryScopeKey{
		AgentID:   "agent-a",
		ProjectID: "project-a",
		RouteKey:  "telegram:default:direct:user-1",
		ChatMode:  memorydomain.ChatModeMain,
	}

	err = guard.ValidateCandidate(scope, memorydomain.LayerAgentPrivate, projectALink)
	if !errors.Is(err, memoryapp.ErrCrossProjectAccess) {
		t.Fatalf("expected ErrCrossProjectAccess, got %v", err)
	}
}
