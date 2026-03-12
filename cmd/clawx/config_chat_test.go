package main

import (
	"path/filepath"
	"testing"

	"clawx/internal/infrastructure/config"
	chatiface "clawx/internal/interfaces/chat"
)

func TestParseConfigChatCommand(t *testing.T) {
	cmd, ok, err := parseConfigChatCommand("/config plan 创建 agent review 使用 claude")
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if !ok {
		t.Fatalf("expected config command")
	}
	if cmd.Action != "plan" {
		t.Fatalf("unexpected action: %s", cmd.Action)
	}
	if cmd.Instruction == "" {
		t.Fatalf("expected instruction")
	}
}

func TestHandleConfigChatCommandPlanAndApply(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	homeDir := filepath.Join(tempDir, "home")

	t.Setenv("CLAWX_CONFIG", configPath)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAWX_CONFIG_ADMIN_USERS", "")

	clearPendingConfigPlans()

	planMessage := chatiface.Message{
		ConversationID: "conv-1",
		UserID:         "user-1",
		Text:           "/config plan 创建 agent review 使用 claude",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(planMessage)
	if err != nil {
		t.Fatalf("plan command failed: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled")
	}
	if response == "" {
		t.Fatalf("expected response")
	}

	applyMessage := chatiface.Message{
		ConversationID: "conv-1",
		UserID:         "user-1",
		Text:           "/config apply",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, _, err = handleConfigChatCommand(applyMessage)
	if err != nil {
		t.Fatalf("apply command failed: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	agent, ok := cfg.Agents["review"]
	if !ok {
		t.Fatalf("expected review agent")
	}
	if agent.ProfileID != "claude" {
		t.Fatalf("unexpected profile: %q", agent.ProfileID)
	}
}

func clearPendingConfigPlans() {
	pendingConfigPlansMu.Lock()
	defer pendingConfigPlansMu.Unlock()
	pendingConfigPlans = make(map[string]pendingConfigPlan)
}
