package integration

import (
	"path/filepath"
	"testing"

	"clawx/internal/infrastructure/config"
)

func TestChannelConfigIncrementalPreservesNonTargetChannels(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	homeDir := filepath.Join(tempDir, "home")

	t.Setenv("CLAWX_CONFIG", configPath)
	t.Setenv("HOME", homeDir)

	if _, _, err := config.EnsureDefaultFile(); err != nil {
		t.Fatalf("ensure default config: %v", err)
	}

	seed := map[string]string{
		"channels.telegram.enabled":         "true",
		"channels.telegram.token":           "tg-token-old",
		"channels.discord.enabled":          "true",
		"channels.discord.botToken":         "dc-token-old",
		"channels.feishu.enabled":           "true",
		"channels.feishu.appId":             "cli_old",
		"channels.feishu.appSecret":         "secret_old",
		"channels.feishu.verificationToken": "verify_old",
	}
	if _, err := config.SetValuesByDotKey(seed); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	patch := map[string]string{
		"channels.wecom.enabled":        "true",
		"channels.wecom.mode":           "webhook",
		"channels.wecom.corpId":         "ww_new",
		"channels.wecom.agentId":        "1000002",
		"channels.wecom.secret":         "secret_new",
		"channels.wecom.token":          "token_new",
		"channels.wecom.encodingAesKey": "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
	}
	if _, err := config.SetValuesByDotKey(patch); err != nil {
		t.Fatalf("apply wecom patch: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.TelegramToken != "tg-token-old" {
		t.Fatalf("telegram token should be preserved, got %q", cfg.TelegramToken)
	}
	if len(cfg.TelegramInstances) == 0 || cfg.TelegramInstances[0].Token != "tg-token-old" {
		t.Fatalf("telegram instance token should be preserved")
	}
	if len(cfg.DiscordInstances) == 0 || cfg.DiscordInstances[0].BotToken != "dc-token-old" {
		t.Fatalf("discord token should be preserved")
	}
	if len(cfg.FeishuInstances) == 0 || cfg.FeishuInstances[0].AppID != "cli_old" {
		t.Fatalf("feishu app id should be preserved")
	}
	if len(cfg.WeComInstances) == 0 {
		t.Fatalf("expected wecom instance after patch")
	}
	if cfg.WeComInstances[0].CorpID != "ww_new" {
		t.Fatalf("unexpected wecom corp id: %q", cfg.WeComInstances[0].CorpID)
	}
}
