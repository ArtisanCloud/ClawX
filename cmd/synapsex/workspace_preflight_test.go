package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"synapsex/internal/infrastructure/config"
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
