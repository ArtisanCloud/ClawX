package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"synapsex/internal/application/intent"
	"synapsex/internal/application/skillregistry"
	skilldomain "synapsex/internal/domain/skill"
	"synapsex/internal/infrastructure/config"
	skillsinfra "synapsex/internal/infrastructure/skills"
)

func runSkillCommand(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	agentID := strings.TrimSpace(cfg.DefaultAgentID)
	if agentID == "" {
		agentID = "main"
	}
	workspace := strings.TrimSpace(cfg.DefaultCWD)
	if cfg.ActiveAgent != nil && strings.TrimSpace(cfg.ActiveAgent.Workspace) != "" {
		workspace = strings.TrimSpace(cfg.ActiveAgent.Workspace)
	}

	registry, _, err := buildSkillRuntimeComponents(cfg, agentID, workspace)
	if err != nil {
		return err
	}

	command := "list"
	if len(args) > 0 {
		command = strings.ToLower(strings.TrimSpace(args[0]))
	}

	switch command {
	case "", "list":
		entries := registry.List()
		fmt.Fprintln(os.Stdout, skillregistry.FormatList(entries))
		return nil
	case "reload":
		result, err := registry.Reload(context.Background())
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "skill reload done: version=%d entries=%d active=%d\n", result.Version, result.Entries, result.Active)
		return nil
	case "enable":
		if len(args) < 2 {
			return fmt.Errorf("usage: synapsex skill enable <name>")
		}
		name := strings.TrimSpace(args[1])
		if err := registry.Enable(name); err != nil {
			return err
		}
		if _, err := config.SetSkillDisabledNames(registry.DisabledNames()); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "skill enabled: %s\n", skilldomain.NormalizeName(name))
		return nil
	case "disable":
		if len(args) < 2 {
			return fmt.Errorf("usage: synapsex skill disable <name>")
		}
		name := strings.TrimSpace(args[1])
		if err := registry.Disable(name); err != nil {
			return err
		}
		if _, err := config.SetSkillDisabledNames(registry.DisabledNames()); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "skill disabled: %s\n", skilldomain.NormalizeName(name))
		return nil
	default:
		return fmt.Errorf("unknown skill command %q", command)
	}
}

func buildSkillRuntimeComponents(cfg config.Snapshot, agentID, workspace string) (*skillregistry.Service, *intent.Pipeline, error) {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		workspace = cfg.DefaultCWD
	}
	sources := []skillsinfra.SourceSpec{
		{Source: skilldomain.SourceUser, Root: cfg.SkillUserDir()},
		{Source: skilldomain.SourceWorkspace, Root: cfg.WorkspaceSkillDir(workspace)},
	}
	if cfg.Skills.Sources.BuiltinEnabled {
		builtin := strings.TrimSpace(cfg.Skills.Sources.BuiltinDir)
		if builtin != "" && !filepath.IsAbs(builtin) {
			builtin = filepath.Clean(builtin)
		}
		sources = append(sources, skillsinfra.SourceSpec{
			Source: skilldomain.SourceBuiltin,
			Root:   builtin,
		})
	}

	registry, err := skillregistry.New(skillregistry.Config{
		Sources:    sources,
		IndexPath:  config.SkillIndexPath(agentID),
		Disabled:   cfg.Skills.DisabledNames,
		RefreshNow: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("init skill registry: %w", err)
	}

	var pairingChecker intent.PairingChecker
	pairingStore, err := skillsinfra.NewPairingStore(config.PairingStorePath(agentID))
	if err == nil {
		pairingChecker = pairingStore
	}

	policy := intent.PermissionPolicy{
		Enabled:       cfg.Skills.Enabled,
		AllowUsers:    buildSet(cfg.Skills.Allowlist.Users),
		AllowChannels: buildSet(cfg.Skills.Allowlist.Channels),
		DefaultMode:   cfg.Skills.DefaultMode,
		PairingTTL:    cfg.Skills.PairingTTL,
	}
	checker := intent.NewPermissionChecker(policy, pairingChecker)

	var fallback intent.LLMFallback
	if cfg.IntentRouter.LLMFallback.Enabled {
		fallback = intent.DefaultLLMFallback{}
	} else {
		fallback = disabledFallback{}
	}

	pipeline := intent.NewPipeline(
		registry,
		checker,
		fallback,
		cfg.IntentRouter.LLMFallback.ConfidenceThreshold,
	)
	return registry, pipeline, nil
}

type disabledFallback struct{}

func (disabledFallback) Match(_ context.Context, _ string, _ skilldomain.RegistrySnapshot) (intent.Candidate, bool, error) {
	return intent.Candidate{}, false, nil
}

func buildSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result[value] = struct{}{}
		}
	}
	return result
}
