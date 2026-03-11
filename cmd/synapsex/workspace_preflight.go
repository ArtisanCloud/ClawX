package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

func autoBootstrapDefaultAgentWorkspace(cfg config.Snapshot) (config.Snapshot, bool, error) {
	defaultAgentID := strings.TrimSpace(cfg.DefaultAgentID)
	if defaultAgentID == "" && cfg.ActiveAgent != nil {
		defaultAgentID = strings.TrimSpace(cfg.ActiveAgent.ID)
	}
	if defaultAgentID == "" {
		return cfg, false, nil
	}

	agent, ok := cfg.Agents[defaultAgentID]
	if !ok {
		return cfg, false, nil
	}

	workspace := strings.TrimSpace(agent.Workspace)
	if workspace == "" {
		workspace = strings.TrimSpace(cfg.DefaultCWD)
	}
	if workspace == "" {
		return cfg, false, nil
	}

	workspace = filepath.Clean(workspace)
	suggested := filepath.Clean(config.SuggestedAgentWorkspace(defaultAgentID))
	if workspace != suggested {
		return cfg, false, nil
	}

	empty, err := directoryEmptyOrMissing(workspace)
	if err != nil || !empty {
		return cfg, false, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return cfg, false, nil
	}
	cwd = filepath.Clean(strings.TrimSpace(cwd))
	if cwd == "" || cwd == workspace {
		return cfg, false, nil
	}

	project, err := looksLikeProjectDirectory(cwd)
	if err != nil || !project {
		return cfg, false, nil
	}

	timeoutSeconds := int(agent.Timeout / time.Second)
	if timeoutSeconds <= 0 {
		timeoutSeconds = int(cfg.Timeout / time.Second)
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 600
	}

	if _, err := config.UpsertAgent(config.AgentUpsertOptions{
		ID:             defaultAgentID,
		ProfileID:      agent.ProfileID,
		Workspace:      cwd,
		TimeoutSeconds: timeoutSeconds,
		SetAsDefault:   true,
	}); err != nil {
		return cfg, false, fmt.Errorf("auto bootstrap workspace for agent %q: %w", defaultAgentID, err)
	}

	updated, err := config.Load()
	if err != nil {
		return cfg, false, err
	}
	return updated, true, nil
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

func directoryEmptyOrMissing(path string) (bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return true, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func looksLikeProjectDirectory(path string) (bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}

	markers := []string{
		".git",
		"go.mod",
		"package.json",
		"pyproject.toml",
		"Cargo.toml",
	}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			return true, nil
		} else if !os.IsNotExist(err) {
			return false, err
		}
	}
	return false, nil
}
