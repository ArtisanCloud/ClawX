package main

import (
	"testing"

	"clawx/internal/infrastructure/config"
)

func TestNormalizeRunTarget(t *testing.T) {
	if got := normalizeRunTarget(" Discord "); got != "discord" {
		t.Fatalf("unexpected normalized target: %q", got)
	}
	if got := normalizeRunTarget("unknown"); got != "" {
		t.Fatalf("expected unknown target to be empty, got=%q", got)
	}
}

func TestApplyRunTargetOverrides(t *testing.T) {
	cfg := config.Snapshot{
		TelegramInstances: []config.TelegramInstance{{ID: "t1", Enabled: true}},
		DiscordInstances:  []config.DiscordInstance{{ID: "d1", Enabled: true}},
		FeishuInstances:   []config.FeishuInstance{{ID: "f1", Enabled: true}},
		WeComInstances:    []config.WeComInstance{{ID: "w1", Enabled: true}},
	}

	applyRunTargetOverrides(&cfg, "discord")

	if !cfg.DiscordInstances[0].Enabled {
		t.Fatalf("expected discord enabled")
	}
	if cfg.TelegramInstances[0].Enabled || cfg.FeishuInstances[0].Enabled || cfg.WeComInstances[0].Enabled {
		t.Fatalf("expected non-target channels disabled")
	}
	if cfg.DiscordEnabled != true || cfg.TelegramEnabled || cfg.FeishuEnabled || cfg.WeComEnabled {
		t.Fatalf("unexpected channel enabled flags")
	}
}
