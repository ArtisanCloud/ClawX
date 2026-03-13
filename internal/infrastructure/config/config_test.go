package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReadsConfigJSON(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	content := `{
  "providers": {
    "profiles": {
      "codex": {
        "kind": "codex-cli",
        "command": "codex"
      }
    }
  },
  "agents": {
    "default": "main",
    "list": [
      {
        "id": "main",
        "profile": "codex",
        "workspace": ".",
        "timeoutSeconds": 42,
        "default": true
      }
    ]
  },
  "runtime": {
    "allowedRoots": ["."],
    "defaultCwd": ".",
    "timeoutSeconds": 42
  },
  "execution": {
    "command": "cat"
  },
  "channels": {
    "telegram": {
      "enabled": true,
      "mode": "polling",
      "token": "token-1",
      "botUsername": "my_bot"
    }
  },
  "gateway": {
    "listenAddr": ":19090",
    "health": {
      "enabled": true,
      "path": "/healthz"
    }
  }
}`
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Timeout.Seconds() != 42 {
		t.Fatalf("unexpected timeout: %v", cfg.Timeout)
	}
	if !cfg.TelegramEnabled {
		t.Fatalf("expected telegram to be enabled")
	}
	if cfg.TelegramToken != "token-1" {
		t.Fatalf("unexpected telegram token: %q", cfg.TelegramToken)
	}
	if cfg.HTTPListenAddr != ":19090" {
		t.Fatalf("unexpected listen addr: %q", cfg.HTTPListenAddr)
	}
	if cfg.ActiveProfile == nil || cfg.ActiveProfile.Kind != "codex-cli" {
		t.Fatalf("expected active profile to resolve to codex-cli, got %#v", cfg.ActiveProfile)
	}
	if cfg.ActiveAgent == nil || cfg.ActiveAgent.ID != "main" {
		t.Fatalf("expected active agent to resolve, got %#v", cfg.ActiveAgent)
	}
}

func TestLoadAllowsEnvOverrideOnTopOfConfigJSON(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	content := `{
  "runtime": {
    "allowedRoots": ["."],
    "defaultCwd": "."
  },
  "execution": {
    "command": "cat"
  },
  "gateway": {
    "listenAddr": ":8080",
    "health": {
      "enabled": true,
      "path": "/healthz"
    }
  }
}`
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	t.Setenv("CLAWX_EXEC_COMMAND", "printf")
	t.Setenv("CLAWX_HTTP_LISTEN_ADDR", ":28080")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.ExecCommand != "printf" {
		t.Fatalf("expected env override for exec command, got %q", cfg.ExecCommand)
	}
	if cfg.HTTPListenAddr != ":28080" {
		t.Fatalf("expected env override for listen addr, got %q", cfg.HTTPListenAddr)
	}
}

func TestLoadIgnoresDotEnvWhenConfigExistsByDefault(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	content := `{
  "runtime": {
    "allowedRoots": ["."],
    "defaultCwd": "."
  },
  "execution": {
    "command": "cat"
  },
  "channels": {
    "discord": {
      "enabled": false
    }
  }
}`
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte("CLAWX_DISCORD_ENABLED=true\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DiscordEnabled {
		t.Fatalf("expected .env to be ignored when config.json exists")
	}
}

func TestEnsureDefaultFileCreatesConfigJSON(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	path, created, err := EnsureDefaultFile()
	if err != nil {
		t.Fatalf("ensure default file: %v", err)
	}
	if !created {
		t.Fatalf("expected config file to be created")
	}
	if path != "config.json" {
		t.Fatalf("unexpected config path: %q", path)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load created config: %v", err)
	}
	if cfg.ActiveAgent == nil || cfg.ActiveAgent.ID != "main" {
		t.Fatalf("expected default agent main, got %#v", cfg.ActiveAgent)
	}
	if cfg.ActiveProfile == nil || cfg.ActiveProfile.ID != "codex" {
		t.Fatalf("expected default profile codex, got %#v", cfg.ActiveProfile)
	}
}

func TestEnsureDefaultFileDoesNotOverwriteExistingConfig(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	content := `{"runtime":{"allowedRoots":["."],"defaultCwd":"."},"execution":{"command":"cat"}}`
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	path, created, err := EnsureDefaultFile()
	if err != nil {
		t.Fatalf("ensure default file: %v", err)
	}
	if created {
		t.Fatalf("expected existing config to be preserved")
	}
	if path != "config.json" {
		t.Fatalf("unexpected config path: %q", path)
	}

	loaded, err := os.ReadFile(filepath.Join(tempDir, "config.json"))
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}
	if string(loaded) != content {
		t.Fatalf("config.json was unexpectedly overwritten")
	}
}

func TestWriteBootstrapFileUsesSelectedProfileAndChannels(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	path, err := WriteBootstrapFile(BootstrapOptions{
		BaseProfileID:         "claude",
		DefaultProfileID:      "team-claude",
		MainWorkspace:         "../workspace-alpha",
		TelegramEnabled:       true,
		TelegramToken:         "tg-token",
		TelegramBotUsername:   "@tg_bot",
		DiscordEnabled:        true,
		DiscordBotToken:       "dc-token",
		DiscordRequireMention: false,
	})
	if err != nil {
		t.Fatalf("write bootstrap file: %v", err)
	}
	if path != "config.json" {
		t.Fatalf("unexpected config path: %q", path)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ActiveProfile == nil || cfg.ActiveProfile.ID != "team-claude" {
		t.Fatalf("expected active profile team-claude, got %#v", cfg.ActiveProfile)
	}
	if cfg.ActiveAgent == nil || cfg.ActiveAgent.Workspace != "../workspace-alpha" {
		t.Fatalf("expected workspace override, got %#v", cfg.ActiveAgent)
	}
	if !cfg.TelegramEnabled || cfg.TelegramToken != "tg-token" {
		t.Fatalf("unexpected telegram config: enabled=%v token=%q", cfg.TelegramEnabled, cfg.TelegramToken)
	}
	if cfg.TelegramBotUsername != "tg_bot" {
		t.Fatalf("unexpected telegram username: %q", cfg.TelegramBotUsername)
	}
	if !cfg.DiscordEnabled || cfg.DiscordBotToken != "dc-token" {
		t.Fatalf("unexpected discord config: enabled=%v token=%q", cfg.DiscordEnabled, cfg.DiscordBotToken)
	}
	if cfg.DiscordRequireMention {
		t.Fatalf("expected discord require mention to be false")
	}
}

func TestLoadParsesChannelInstancesAndAgentBindings(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	content := `{
  "providers": {
    "profiles": {
      "codex": {
        "kind": "codex-cli",
        "command": "codex"
      },
      "claude": {
        "kind": "claude-cli",
        "command": "claude"
      }
    }
  },
  "agents": {
    "default": "main",
    "list": [
      {
        "id": "main",
        "profile": "codex",
        "workspace": ".",
        "default": true
      },
      {
        "id": "docs-agent",
        "profile": "claude",
        "workspace": "../docs"
      }
    ]
  },
  "runtime": {
    "allowedRoots": [".", "../docs"],
    "defaultCwd": "."
  },
  "channels": {
    "discord": {
      "enabled": true,
      "defaultAgent": "main",
      "agentBindings": {
        "channel:1001": "docs-agent"
      },
      "instances": [
        {
          "id": "discord-main",
          "enabled": true,
          "botToken": "token-1",
          "apiBaseUrl": "https://discord.com/api/v10",
          "gatewayUrl": "wss://gateway.discord.gg/?v=10&encoding=json",
          "requireMention": true,
          "defaultAgent": "main",
          "agentBindings": {
            "channel:2002": "docs-agent"
          }
        }
      ]
    }
  }
}`
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if !cfg.DiscordEnabled {
		t.Fatalf("expected discord enabled")
	}
	if got := len(cfg.DiscordInstances); got != 1 {
		t.Fatalf("unexpected discord instances count: %d", got)
	}
	if cfg.DiscordInstances[0].ID != "discord-main" {
		t.Fatalf("unexpected discord instance id: %q", cfg.DiscordInstances[0].ID)
	}
	if cfg.DiscordInstances[0].DefaultAgentID != "main" {
		t.Fatalf("unexpected instance default agent: %q", cfg.DiscordInstances[0].DefaultAgentID)
	}
	if cfg.DiscordInstances[0].AgentBindings["channel:2002"] != "docs-agent" {
		t.Fatalf("unexpected instance binding: %#v", cfg.DiscordInstances[0].AgentBindings)
	}
	if cfg.DiscordAgentBindings["channel:1001"] != "docs-agent" {
		t.Fatalf("unexpected channel binding: %#v", cfg.DiscordAgentBindings)
	}
}

func TestLoadParsesFeishuAndWeComInstances(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	content := `{
  "providers": {
    "profiles": {
      "codex": {
        "kind": "codex-cli",
        "command": "codex"
      }
    }
  },
  "agents": {
    "default": "main",
    "list": [
      {
        "id": "main",
        "profile": "codex",
        "workspace": ".",
        "default": true
      }
    ]
  },
  "runtime": {
    "allowedRoots": ["."],
    "defaultCwd": "."
  },
  "channels": {
    "feishu": {
      "enabled": true,
      "defaultAgent": "main",
      "instances": [
        {
          "id": "feishu-default",
          "enabled": true,
          "mode": "webhook",
          "appId": "app-id",
          "appSecret": "app-secret",
          "verificationToken": "verify-token",
          "encryptKey": "encrypt-key",
          "defaultAgent": "main"
        }
      ]
    },
    "wecom": {
      "enabled": true,
      "defaultAgent": "main",
      "instances": [
        {
          "id": "wecom-default",
          "enabled": true,
          "mode": "webhook",
          "corpId": "corp-id",
          "agentId": "agent-id",
          "secret": "secret-value",
          "token": "token-value",
          "encodingAesKey": "encoding-key",
          "defaultAgent": "main"
        }
      ]
    }
  }
}`
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if !cfg.FeishuEnabled || len(cfg.FeishuInstances) != 1 {
		t.Fatalf("unexpected feishu config: enabled=%v instances=%d", cfg.FeishuEnabled, len(cfg.FeishuInstances))
	}
	if cfg.FeishuInstances[0].ID != "feishu-default" {
		t.Fatalf("unexpected feishu instance id: %q", cfg.FeishuInstances[0].ID)
	}
	if !cfg.WeComEnabled || len(cfg.WeComInstances) != 1 {
		t.Fatalf("unexpected wecom config: enabled=%v instances=%d", cfg.WeComEnabled, len(cfg.WeComInstances))
	}
	if cfg.WeComInstances[0].ID != "wecom-default" {
		t.Fatalf("unexpected wecom instance id: %q", cfg.WeComInstances[0].ID)
	}
}

func TestLoadParsesExtendedWaveChannelConfig(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	content := `{
  "providers": {
    "profiles": {
      "codex": {
        "kind": "codex-cli",
        "command": "codex"
      }
    }
  },
  "agents": {
    "default": "main",
    "list": [
      {
        "id": "main",
        "profile": "codex",
        "workspace": ".",
        "default": true
      }
    ]
  },
  "runtime": {
    "allowedRoots": ["."],
    "defaultCwd": "."
  },
  "channels": {
    "slack": {
      "enabled": true,
      "defaultAgent": "main",
      "instances": [
        {
          "id": "slack-prod",
          "enabled": true,
          "defaultAgent": "main"
        },
        {
          "enabled": false
        }
      ]
    },
    "nextcloud-talk": {
      "enabled": false,
      "instances": [
        {
          "enabled": true
        }
      ]
    }
  }
}`
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	slack, ok := cfg.ExtendedChannels["slack"]
	if !ok {
		t.Fatalf("expected slack extended channel to be present")
	}
	if !slack.Enabled {
		t.Fatalf("expected slack channel enabled")
	}
	if slack.DefaultAgentID != "main" {
		t.Fatalf("unexpected slack default agent: %q", slack.DefaultAgentID)
	}
	if len(slack.Instances) != 2 {
		t.Fatalf("unexpected slack instances: %d", len(slack.Instances))
	}
	if slack.Instances[0].ID != "slack-prod" {
		t.Fatalf("unexpected slack instance id: %q", slack.Instances[0].ID)
	}
	if slack.Instances[1].ID == "" {
		t.Fatalf("expected generated id for slack second instance")
	}
	if slack.Instances[1].DefaultAgentID != "main" {
		t.Fatalf("expected fallback default agent for slack second instance")
	}
	if !cfg.IsChannelEnabled("slack") {
		t.Fatalf("expected IsChannelEnabled(slack)=true")
	}

	nextcloud, ok := cfg.ExtendedChannels["nextcloud-talk"]
	if !ok {
		t.Fatalf("expected nextcloud-talk channel")
	}
	if len(nextcloud.Instances) != 1 {
		t.Fatalf("unexpected nextcloud-talk instances: %d", len(nextcloud.Instances))
	}
	if nextcloud.Instances[0].ID == "" {
		t.Fatalf("expected generated id for nextcloud-talk instance")
	}
	if nextcloud.Instances[0].DefaultAgentID != "main" {
		t.Fatalf("expected fallback default agent for nextcloud-talk instance")
	}
}

func TestLoadRejectsUnknownChannelAgentBinding(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	content := `{
  "providers": {
    "profiles": {
      "codex": {
        "kind": "codex-cli",
        "command": "codex"
      }
    }
  },
  "agents": {
    "default": "main",
    "list": [
      {
        "id": "main",
        "profile": "codex",
        "workspace": ".",
        "default": true
      }
    ]
  },
  "runtime": {
    "allowedRoots": ["."],
    "defaultCwd": "."
  },
  "channels": {
    "telegram": {
      "enabled": true,
      "instances": [
        {
          "id": "tg-1",
          "enabled": true,
          "mode": "polling",
          "token": "token-1",
          "defaultAgent": "missing-agent"
        }
      ]
    }
  }
}`
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	_, err := Load()
	if !errors.Is(err, ErrUnknownAgent) {
		t.Fatalf("expected ErrUnknownAgent, got %v", err)
	}
}

func TestValidateRejectsUnknownExtendedChannelAgentBinding(t *testing.T) {
	cfg := defaultSnapshot()
	cfg.ProviderProfiles = map[string]ProviderProfile{
		"codex": {
			ID:      "codex",
			Kind:    "codex-cli",
			Command: "codex",
		},
	}
	cfg.Agents = map[string]Agent{
		"main": {
			ID:        "main",
			ProfileID: "codex",
			Workspace: ".",
			Timeout:   30,
		},
	}
	cfg.DefaultAgentID = "main"
	cfg.ExtendedChannels = map[string]ExtendedChannelConfig{
		"slack": {
			Enabled:        true,
			DefaultAgentID: "missing-agent",
			Instances: []ExtendedChannelInstance{
				{ID: "slack-1", Enabled: true, DefaultAgentID: "main"},
			},
		},
	}
	cfg.normalizeChannelInstances()

	err := cfg.Validate()
	if !errors.Is(err, ErrUnknownAgent) {
		t.Fatalf("expected ErrUnknownAgent, got %v", err)
	}
}

func TestUpsertAgentUsesSuggestedWorkspace(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)
	homeDir := filepath.Join(tempDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", homeDir)

	if _, err := UpsertAgent(AgentUpsertOptions{
		ID:        "project-alpha",
		ProfileID: "codex",
	}); err != nil {
		t.Fatalf("upsert agent: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	agent, ok := cfg.Agents["project-alpha"]
	if !ok {
		t.Fatalf("expected project-alpha agent")
	}
	wantWorkspace := filepath.Join(homeDir, ".clawx", "workspaces", "project-alpha")
	if agent.Workspace != wantWorkspace {
		t.Fatalf("unexpected workspace: got %q want %q", agent.Workspace, wantWorkspace)
	}
}

func TestSetDefaultAgentUpdatesDefault(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)
	homeDir := filepath.Join(tempDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", homeDir)

	if _, err := UpsertAgent(AgentUpsertOptions{
		ID:        "review",
		ProfileID: "claude",
	}); err != nil {
		t.Fatalf("upsert agent: %v", err)
	}
	if _, err := SetDefaultAgent("review"); err != nil {
		t.Fatalf("set default agent: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DefaultAgentID != "review" {
		t.Fatalf("unexpected default agent: %q", cfg.DefaultAgentID)
	}
}

func TestSetValueByDotKeyUpdatesTelegramIncrementally(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	path, created, err := EnsureDefaultFile()
	if err != nil {
		t.Fatalf("ensure default file: %v", err)
	}
	if !created || path != "config.json" {
		t.Fatalf("expected default config.json to be created, created=%v path=%q", created, path)
	}

	if _, err := SetValueByDotKey("channels.discord.enabled", "true"); err != nil {
		t.Fatalf("set discord enabled: %v", err)
	}
	if _, err := SetValueByDotKey("channels.discord.botToken", "discord-token"); err != nil {
		t.Fatalf("set discord token: %v", err)
	}
	if _, err := SetValueByDotKey("channels.telegram.enabled", "true"); err != nil {
		t.Fatalf("set telegram enabled: %v", err)
	}
	if _, err := SetValueByDotKey("channels.telegram.token", "telegram-token"); err != nil {
		t.Fatalf("set telegram token: %v", err)
	}
	if _, err := SetValueByDotKey("channels.telegram.botUsername", "my_bot"); err != nil {
		t.Fatalf("set telegram username: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ActiveAgent == nil || cfg.ActiveAgent.ID != "main" {
		t.Fatalf("expected default main agent to remain, got %#v", cfg.ActiveAgent)
	}
	if !cfg.DiscordEnabled || cfg.DiscordBotToken != "discord-token" {
		t.Fatalf("expected discord config to remain set, enabled=%v token=%q", cfg.DiscordEnabled, cfg.DiscordBotToken)
	}
	if !cfg.TelegramEnabled || cfg.TelegramToken != "telegram-token" {
		t.Fatalf("expected telegram to be updated, enabled=%v token=%q", cfg.TelegramEnabled, cfg.TelegramToken)
	}
	if cfg.TelegramBotUsername != "my_bot" {
		t.Fatalf("unexpected telegram username: %q", cfg.TelegramBotUsername)
	}
}

func TestSetValuesByDotKeyAppliesSingleChannelPatchWithoutOverwritingOthers(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	if _, _, err := EnsureDefaultFile(); err != nil {
		t.Fatalf("ensure default file: %v", err)
	}
	if _, err := SetValuesByDotKey(map[string]string{
		"channels.telegram.enabled": "true",
		"channels.telegram.token":   "telegram-token-old",
		"channels.discord.enabled":  "true",
		"channels.discord.botToken": "discord-token-old",
	}); err != nil {
		t.Fatalf("seed channel values: %v", err)
	}

	if _, err := SetValuesByDotKey(map[string]string{
		"channels.wecom.enabled":        "true",
		"channels.wecom.mode":           "webhook",
		"channels.wecom.corpId":         "ww_new",
		"channels.wecom.agentId":        "1000002",
		"channels.wecom.secret":         "secret_new",
		"channels.wecom.token":          "token_new",
		"channels.wecom.encodingAesKey": "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
	}); err != nil {
		t.Fatalf("set wecom values: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.TelegramToken != "telegram-token-old" {
		t.Fatalf("telegram token should be preserved, got %q", cfg.TelegramToken)
	}
	if cfg.DiscordBotToken != "discord-token-old" {
		t.Fatalf("discord token should be preserved, got %q", cfg.DiscordBotToken)
	}
	if cfg.WeComCorpID != "ww_new" {
		t.Fatalf("wecom corp id should be updated, got %q", cfg.WeComCorpID)
	}
}

func TestGetValueByDotKeySupportsNestedAndRoot(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	if _, _, err := EnsureDefaultFile(); err != nil {
		t.Fatalf("ensure default file: %v", err)
	}
	if _, err := SetValueByDotKey("channels.telegram.enabled", "true"); err != nil {
		t.Fatalf("set telegram enabled: %v", err)
	}

	value, err := GetValueByDotKey("channels.telegram.enabled")
	if err != nil {
		t.Fatalf("get nested key: %v", err)
	}
	enabled, ok := value.(bool)
	if !ok || !enabled {
		t.Fatalf("expected bool true for telegram enabled, got %#v", value)
	}

	root, err := GetValueByDotKey("")
	if err != nil {
		t.Fatalf("get root config: %v", err)
	}
	if _, ok := root.(map[string]any); !ok {
		t.Fatalf("expected root get to return map, got %T", root)
	}
}

func TestGetValueByDotKeyMissingReturnsNotFound(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	if _, _, err := EnsureDefaultFile(); err != nil {
		t.Fatalf("ensure default file: %v", err)
	}

	_, err := GetValueByDotKey("channels.telegram.not_exists")
	if !errors.Is(err, ErrConfigKeyNotFound) {
		t.Fatalf("expected ErrConfigKeyNotFound, got %v", err)
	}
}

func TestValidateAllowsTelegramWebhookMode(t *testing.T) {
	cfg := defaultSnapshot()
	cfg.TelegramInstances = []TelegramInstance{
		{
			ID:                      "telegram-default",
			Enabled:                 true,
			Mode:                    "webhook",
			Token:                   "token-1",
			WebhookURL:              "https://example.com/webhook/tg",
			WebhookPath:             "/webhooks/telegram",
			RequireCommandOrMention: true,
			PollingTimeout:          30,
			DefaultAgentID:          "main",
		},
	}
	cfg.normalizeChannelInstances()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate webhook mode: %v", err)
	}
}

func TestValidateRejectsTelegramWebhookWithoutURL(t *testing.T) {
	cfg := defaultSnapshot()
	cfg.TelegramInstances = []TelegramInstance{
		{
			ID:                      "telegram-default",
			Enabled:                 true,
			Mode:                    "webhook",
			Token:                   "token-1",
			WebhookPath:             "/webhooks/telegram",
			RequireCommandOrMention: true,
			PollingTimeout:          30,
			DefaultAgentID:          "main",
		},
	}
	cfg.normalizeChannelInstances()

	err := cfg.Validate()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestValidateRejectsFeishuWebhookWithoutCredentials(t *testing.T) {
	cfg := defaultSnapshot()
	cfg.FeishuInstances = []FeishuInstance{
		{
			ID:             "feishu-default",
			Enabled:        true,
			Mode:           "webhook",
			DefaultAgentID: "main",
		},
	}
	cfg.normalizeChannelInstances()

	err := cfg.Validate()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestValidateRejectsWeComWebhookWithoutCredentials(t *testing.T) {
	cfg := defaultSnapshot()
	cfg.WeComInstances = []WeComInstance{
		{
			ID:             "wecom-default",
			Enabled:        true,
			Mode:           "webhook",
			DefaultAgentID: "main",
		},
	}
	cfg.normalizeChannelInstances()

	err := cfg.Validate()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestWriteBootstrapFileWithDatabaseConfig(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	_, err := WriteBootstrapFile(BootstrapOptions{
		DefaultProfileID:   "codex",
		DatabaseEnabled:    true,
		DatabaseDriver:     "postgres",
		DatabaseHost:       "127.0.0.1",
		DatabasePort:       5432,
		DatabaseName:       "claw_x",
		DatabaseUser:       "postgres",
		DatabasePassword:   "secret",
		DatabaseSSLMode:    "disable",
		DatabaseAutoCreate: true,
	})
	if err != nil {
		t.Fatalf("write bootstrap file: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if !cfg.Database.Enabled {
		t.Fatalf("expected database enabled")
	}
	if cfg.Database.Driver != "postgres" {
		t.Fatalf("unexpected database driver: %q", cfg.Database.Driver)
	}
	if cfg.Database.Name != "claw_x" {
		t.Fatalf("unexpected database name: %q", cfg.Database.Name)
	}
	if cfg.Database.User != "postgres" {
		t.Fatalf("unexpected database user: %q", cfg.Database.User)
	}
	if cfg.Database.Password != "secret" {
		t.Fatalf("unexpected database password")
	}
}

func TestEnsureStateLayoutMigratesLegacyWorkspaceRootAndConfigPaths(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	homeDir := filepath.Join(tempDir, "home")
	stateDir := filepath.Join(homeDir, ".clawx")
	legacyRoot := filepath.Join(stateDir, "workworkspace")
	legacyMain := filepath.Join(legacyRoot, "main")
	if err := os.MkdirAll(legacyMain, 0o755); err != nil {
		t.Fatalf("mkdir legacy workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyMain, "marker.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write legacy marker: %v", err)
	}

	currentRoot := filepath.Join(stateDir, "workspaces")
	legacyMainPath := filepath.Join(legacyRoot, "main")
	content := `{
  "runtime": {
    "allowedRoots": ["` + legacyRoot + `"],
    "defaultCwd": "` + legacyMainPath + `"
  },
  "providers": {
    "profiles": {
      "codex": {
        "kind": "codex-cli",
        "command": "codex"
      }
    }
  },
  "agents": {
    "default": "main",
    "list": [
      {
        "id": "main",
        "profile": "codex",
        "workspace": "` + legacyMainPath + `",
        "default": true
      }
    ]
  }
}`
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := EnsureStateLayout(); err != nil {
		t.Fatalf("ensure state layout: %v", err)
	}

	migratedMain := filepath.Join(currentRoot, "main")
	if _, err := os.Stat(filepath.Join(migratedMain, "marker.txt")); err != nil {
		t.Fatalf("expected migrated marker file under new root: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ActiveAgent == nil {
		t.Fatalf("expected active agent")
	}
	wantMain := filepath.Join(currentRoot, "main")
	if cfg.ActiveAgent.Workspace != wantMain {
		t.Fatalf("expected workspace to be rewritten: got %q want %q", cfg.ActiveAgent.Workspace, wantMain)
	}
	if cfg.DefaultCWD != wantMain {
		t.Fatalf("expected default cwd to be rewritten: got %q want %q", cfg.DefaultCWD, wantMain)
	}
}

func TestEnsureStateLayoutCreatesStarterSkill(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	if err := EnsureStateLayout(); err != nil {
		t.Fatalf("ensure state layout: %v", err)
	}

	manifestPath := filepath.Join(os.Getenv("HOME"), ".clawx", "skills", "echo", "SKILL.md")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read starter skill manifest: %v", err)
	}
	content := string(body)
	if !strings.Contains(content, "name: echo") {
		t.Fatalf("expected starter skill to contain name: echo")
	}
	if !strings.Contains(content, "description: ") {
		t.Fatalf("expected starter skill to contain description")
	}
}

func TestEnsureStateLayoutDoesNotOverwriteStarterSkill(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	manifestPath := filepath.Join(os.Getenv("HOME"), ".clawx", "skills", "echo", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("mkdir starter skill dir: %v", err)
	}
	custom := "---\nname: echo\ndescription: custom\n---\ncustom-body\n"
	if err := os.WriteFile(manifestPath, []byte(custom), 0o644); err != nil {
		t.Fatalf("write custom starter skill manifest: %v", err)
	}

	if err := EnsureStateLayout(); err != nil {
		t.Fatalf("ensure state layout: %v", err)
	}

	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read starter skill manifest: %v", err)
	}
	if string(body) != custom {
		t.Fatalf("starter skill manifest should not be overwritten")
	}
}

func chdirForTest(t *testing.T, dir string) {
	t.Helper()

	t.Setenv("CLAWX_CONFIG", "config.json")
	homeDir := filepath.Join(dir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", homeDir)

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previous)
	})
}
