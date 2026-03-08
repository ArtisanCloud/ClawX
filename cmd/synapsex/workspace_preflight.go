package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"synapsex/internal/infrastructure/config"
)

func ensureWorkspacesReady(cfg config.Snapshot) error {
	paths := collectWorkspacePaths(cfg)
	for _, path := range paths {
		if err := ensureWorkspacePath(path); err != nil {
			return err
		}
	}
	return nil
}

func collectWorkspacePaths(cfg config.Snapshot) []string {
	seen := make(map[string]struct{})
	add := func(raw string) {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return
		}
		cleaned := filepath.Clean(trimmed)
		seen[cleaned] = struct{}{}
	}

	add(cfg.DefaultCWD)
	if cfg.ActiveAgent != nil {
		add(cfg.ActiveAgent.Workspace)
	}
	for _, agent := range cfg.Agents {
		add(agent.Workspace)
	}

	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func ensureWorkspacePath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("workspace path is empty")
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create workspace %q: %w", path, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat workspace %q: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace %q is not a directory", path)
	}

	probe, err := os.CreateTemp(path, ".synapsex-writecheck-*")
	if err != nil {
		return fmt.Errorf("workspace %q is not writable: %w", path, err)
	}
	probePath := probe.Name()
	_ = probe.Close()
	_ = os.Remove(probePath)

	return nil
}
