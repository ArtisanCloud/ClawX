package main

import (
	"os"
	"testing"

	"clawx/internal/application/skillregistry"
	"clawx/internal/infrastructure/config"
)

func TestBuildSkillRuntimeComponentsBuiltinDiscovery(t *testing.T) {
	cfg := config.Snapshot{
		DefaultCWD: t.TempDir(),
		Skills: config.SkillConfig{
			Enabled: true,
			Sources: config.SkillSources{
				UserDir:        t.TempDir(),
				WorkspaceDir:   ".clawx/skills",
				BuiltinEnabled: true,
				BuiltinDir:     "internal/skills/builtin",
			},
			DefaultMode: "allow",
		},
		IntentRouter: config.IntentRouterConfig{
			LLMFallback: config.LLMFallbackConfig{Enabled: false, ConfidenceThreshold: 0.70},
		},
	}
	registry, _, err := buildSkillRuntimeComponents(cfg, "agent-a", cfg.DefaultCWD)
	if err != nil {
		t.Fatalf("build skill components: %v", err)
	}
	assertRegistryContainsSkill(t, registry, "web-search")
	assertRegistryContainsSkill(t, registry, "web-fetch")
}

func TestBuildSkillRuntimeComponentsRelativeBuiltinDirFromDifferentCWD(t *testing.T) {
	cfg := config.Snapshot{
		DefaultCWD: t.TempDir(),
		Skills: config.SkillConfig{
			Enabled: true,
			Sources: config.SkillSources{
				UserDir:        t.TempDir(),
				WorkspaceDir:   ".clawx/skills",
				BuiltinEnabled: true,
				BuiltinDir:     "internal/skills/builtin",
			},
			DefaultMode: "allow",
		},
		IntentRouter: config.IntentRouterConfig{
			LLMFallback: config.LLMFallbackConfig{Enabled: false, ConfidenceThreshold: 0.70},
		},
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	outside := t.TempDir()
	if err := os.Chdir(outside); err != nil {
		t.Fatalf("chdir outside repo: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	registry, _, err := buildSkillRuntimeComponents(cfg, "agent-b", cfg.DefaultCWD)
	if err != nil {
		t.Fatalf("build skill components from different cwd: %v", err)
	}
	assertRegistryContainsSkill(t, registry, "web-search")
}

func assertRegistryContainsSkill(t *testing.T, registry *skillregistry.Service, name string) {
	t.Helper()
	for _, entry := range registry.List() {
		if entry.SkillName == name {
			return
		}
	}
	t.Fatalf("skill %q not found in registry", name)
}
