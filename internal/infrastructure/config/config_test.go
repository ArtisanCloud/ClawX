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

	t.Setenv("SYNAPSEX_EXEC_COMMAND", "printf")
	t.Setenv("SYNAPSEX_HTTP_LISTEN_ADDR", ":28080")

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
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte("SYNAPSEX_DISCORD_ENABLED=true\n"), 0o644); err != nil {
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
	wantWorkspace := filepath.Join(homeDir, ".synapsex", "workspaces", "project-alpha")
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

func TestWriteBootstrapFileWithDatabaseConfig(t *testing.T) {
	tempDir := t.TempDir()
	chdirForTest(t, tempDir)

	_, err := WriteBootstrapFile(BootstrapOptions{
		DefaultProfileID:   "codex",
		DatabaseEnabled:    true,
		DatabaseDriver:     "postgres",
		DatabaseHost:       "127.0.0.1",
		DatabasePort:       5432,
		DatabaseName:       "synapse_x",
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
	if cfg.Database.Name != "synapse_x" {
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
	stateDir := filepath.Join(homeDir, ".synapsex")
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

	manifestPath := filepath.Join(os.Getenv("HOME"), ".synapsex", "skills", "echo", "SKILL.md")
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

	manifestPath := filepath.Join(os.Getenv("HOME"), ".synapsex", "skills", "echo", "SKILL.md")
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

	t.Setenv("SYNAPSEX_CONFIG", "config.json")
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
