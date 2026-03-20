package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clawx/internal/infrastructure/config"
)

func TestEnsureWorkspacesReadyCreatesMissingDirectories(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "workspaces", "main")
	reviewPath := filepath.Join(root, "workspaces", "review")

	cfg := config.Snapshot{
		DefaultCWD: mainPath,
		Agents: map[string]config.Agent{
			"main": {
				ID:        "main",
				ProfileID: "codex",
				Workspace: mainPath,
				Timeout:   10 * time.Minute,
			},
			"review": {
				ID:        "review",
				ProfileID: "claude",
				Workspace: reviewPath,
				Timeout:   10 * time.Minute,
			},
		},
	}

	if err := ensureWorkspacesReady(cfg); err != nil {
		t.Fatalf("ensure workspaces ready: %v", err)
	}

	if info, err := os.Stat(mainPath); err != nil || !info.IsDir() {
		t.Fatalf("main workspace not ready: err=%v", err)
	}
	if info, err := os.Stat(reviewPath); err != nil || !info.IsDir() {
		t.Fatalf("review workspace not ready: err=%v", err)
	}
}

func TestEnsureWorkspacesReadyCreatesSkillSourceDirectories(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "workspaces", "main")
	reviewPath := filepath.Join(root, "workspaces", "review")
	userSkills := filepath.Join(root, "user-skills")
	builtinSkills := filepath.Join(root, "builtin-skills")

	cfg := config.Snapshot{
		DefaultCWD: mainPath,
		Agents: map[string]config.Agent{
			"main": {
				ID:        "main",
				ProfileID: "codex",
				Workspace: mainPath,
				Timeout:   10 * time.Minute,
			},
			"review": {
				ID:        "review",
				ProfileID: "claude",
				Workspace: reviewPath,
				Timeout:   10 * time.Minute,
			},
		},
		Skills: config.SkillConfig{
			Sources: config.SkillSources{
				UserDir:        userSkills,
				WorkspaceDir:   ".clawx/skills",
				BuiltinEnabled: true,
				BuiltinDir:     builtinSkills,
			},
		},
	}

	if err := ensureWorkspacesReady(cfg); err != nil {
		t.Fatalf("ensure workspaces with skill dirs ready: %v", err)
	}

	assertDirExists(t, userSkills)
	assertDirExists(t, filepath.Join(mainPath, ".clawx", "skills"))
	assertDirExists(t, filepath.Join(reviewPath, ".clawx", "skills"))
	assertDirExists(t, builtinSkills)
}

func TestEnsureWorkspacePathRejectsFile(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	err := ensureWorkspacePath(filePath)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDirectoryEmptyOrMissing(t *testing.T) {
	root := t.TempDir()

	emptyDir := filepath.Join(root, "empty")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatalf("mkdir empty: %v", err)
	}
	isEmpty, err := directoryEmptyOrMissing(emptyDir)
	if err != nil {
		t.Fatalf("check empty dir: %v", err)
	}
	if !isEmpty {
		t.Fatalf("expected empty dir to be empty")
	}

	nonEmptyDir := filepath.Join(root, "non-empty")
	if err := os.MkdirAll(nonEmptyDir, 0o755); err != nil {
		t.Fatalf("mkdir non-empty: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nonEmptyDir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	isEmpty, err = directoryEmptyOrMissing(nonEmptyDir)
	if err != nil {
		t.Fatalf("check non-empty dir: %v", err)
	}
	if isEmpty {
		t.Fatalf("expected non-empty dir to be non-empty")
	}

	missing := filepath.Join(root, "missing")
	isEmpty, err = directoryEmptyOrMissing(missing)
	if err != nil {
		t.Fatalf("check missing path: %v", err)
	}
	if !isEmpty {
		t.Fatalf("expected missing path to be treated as empty")
	}
}

func TestLooksLikeProjectDirectory(t *testing.T) {
	root := t.TempDir()

	project := filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	ok, err := looksLikeProjectDirectory(project)
	if err != nil {
		t.Fatalf("looksLikeProjectDirectory(project): %v", err)
	}
	if !ok {
		t.Fatalf("expected project dir to be recognized")
	}

	plain := filepath.Join(root, "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatalf("mkdir plain: %v", err)
	}
	ok, err = looksLikeProjectDirectory(plain)
	if err != nil {
		t.Fatalf("looksLikeProjectDirectory(plain): %v", err)
	}
	if ok {
		t.Fatalf("plain dir should not be recognized as project")
	}
}

func assertDirExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected dir %q to exist: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %q to be dir", path)
	}
}
