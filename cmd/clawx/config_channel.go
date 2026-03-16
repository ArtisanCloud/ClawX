package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"clawx/internal/infrastructure/config"
)

func runConfigChannelCommand(args []string) error {
	if _, _, err := config.EnsureDefaultFile(); err != nil {
		return fmt.Errorf("prepare config: %w", err)
	}

	channel := ""
	if len(args) > 0 {
		channel = strings.ToLower(strings.TrimSpace(args[0]))
	}
	if channel == "" {
		if !interactiveInputAvailable() {
			return fmt.Errorf("channel is required in non-interactive mode; use `clawx config channel telegram|discord|feishu|wecom`")
		}
		selected, err := promptMenu(
			"选择要增量配置的 Channel:",
			[]menuOption{
				{key: "telegram", label: "Telegram", aliases: []string{"1", "telegram", "tg"}, selected: true},
				{key: "discord", label: "Discord", aliases: []string{"2", "discord", "dc"}},
				{key: "feishu", label: "Feishu", aliases: []string{"3", "feishu", "fs"}},
				{key: "wecom", label: "WeCom", aliases: []string{"4", "wecom", "wx", "qywx"}},
			},
		)
		if err != nil {
			return err
		}
		channel = selected
	}

	switch channel {
	case "telegram", "tg":
		return runConfigChannelTelegram()
	case "discord", "dc":
		return runConfigChannelDiscord()
	case "feishu", "fs":
		return runConfigChannelFeishu()
	case "wecom", "wx", "qywx":
		return runConfigChannelWeCom()
	case "help", "-h", "--help":
		fmt.Fprintln(os.Stdout, "Usage: clawx config channel [telegram|discord|feishu|wecom]")
		return nil
	default:
		return fmt.Errorf("unknown channel %q; expected telegram, discord, feishu, or wecom", channel)
	}
}

func runConfigChannelFeishu() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Feishu 增量配置（回车保持原值）")

	enabled, err := promptBool("启用 Feishu?", cfg.FeishuEnabled)
	if err != nil {
		return err
	}
	appID, err := promptString("Feishu appId (留空保持): ")
	if err != nil {
		return err
	}
	appSecret, err := promptString("Feishu appSecret (留空保持): ")
	if err != nil {
		return err
	}
	verifyToken, err := promptString("Feishu verificationToken (留空保持): ")
	if err != nil {
		return err
	}
	encryptKey, err := promptString("Feishu encryptKey (留空保持): ")
	if err != nil {
		return err
	}
	defaultAgent, err := promptString(fmt.Sprintf("Feishu defaultAgent (留空保持，当前 %s): ", firstNonEmpty(cfg.FeishuDefaultAgentID, cfg.DefaultAgentID, "main")))
	if err != nil {
		return err
	}

	updates := map[string]string{
		"channels.feishu.enabled": strconv.FormatBool(enabled),
		"channels.feishu.mode":    "webhook",
	}
	if strings.TrimSpace(appID) != "" {
		updates["channels.feishu.appId"] = strings.TrimSpace(appID)
	}
	if strings.TrimSpace(appSecret) != "" {
		updates["channels.feishu.appSecret"] = strings.TrimSpace(appSecret)
	}
	if strings.TrimSpace(verifyToken) != "" {
		updates["channels.feishu.verificationToken"] = strings.TrimSpace(verifyToken)
	}
	if strings.TrimSpace(encryptKey) != "" {
		updates["channels.feishu.encryptKey"] = strings.TrimSpace(encryptKey)
	}
	if strings.TrimSpace(defaultAgent) != "" {
		updates["channels.feishu.defaultAgent"] = strings.TrimSpace(defaultAgent)
	}
	if _, err := config.SetValuesByDotKey(updates); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "Feishu channel config updated.")
	return nil
}

func runConfigChannelWeCom() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "WeCom 增量配置（回车保持原值）")

	enabled, err := promptBool("启用 WeCom?", cfg.WeComEnabled)
	if err != nil {
		return err
	}
	corpID, err := promptString("WeCom corpId (留空保持): ")
	if err != nil {
		return err
	}
	agentID, err := promptString("WeCom agentId (留空保持): ")
	if err != nil {
		return err
	}
	secret, err := promptString("WeCom secret (留空保持): ")
	if err != nil {
		return err
	}
	token, err := promptString("WeCom token (留空保持): ")
	if err != nil {
		return err
	}
	encodingAESKey, err := promptString("WeCom encodingAesKey (留空保持): ")
	if err != nil {
		return err
	}
	defaultAgent, err := promptString(fmt.Sprintf("WeCom defaultAgent (留空保持，当前 %s): ", firstNonEmpty(cfg.WeComDefaultAgentID, cfg.DefaultAgentID, "main")))
	if err != nil {
		return err
	}
	agentBindingsRaw, err := promptString("WeCom agentBindings JSON (留空保持): ")
	if err != nil {
		return err
	}

	updates := map[string]string{
		"channels.wecom.enabled": strconv.FormatBool(enabled),
		"channels.wecom.mode":    "webhook",
	}
	if strings.TrimSpace(corpID) != "" {
		updates["channels.wecom.corpId"] = strings.TrimSpace(corpID)
	}
	if strings.TrimSpace(agentID) != "" {
		updates["channels.wecom.agentId"] = strings.TrimSpace(agentID)
	}
	if strings.TrimSpace(secret) != "" {
		updates["channels.wecom.secret"] = strings.TrimSpace(secret)
	}
	if strings.TrimSpace(token) != "" {
		updates["channels.wecom.token"] = strings.TrimSpace(token)
	}
	if strings.TrimSpace(encodingAESKey) != "" {
		updates["channels.wecom.encodingAesKey"] = strings.TrimSpace(encodingAESKey)
	}
	if strings.TrimSpace(defaultAgent) != "" {
		updates["channels.wecom.defaultAgent"] = strings.TrimSpace(defaultAgent)
	}
	if strings.TrimSpace(agentBindingsRaw) != "" {
		var raw map[string]string
		if err := json.Unmarshal([]byte(agentBindingsRaw), &raw); err != nil {
			return fmt.Errorf("invalid agentBindings json: %w", err)
		}
		payload, err := json.Marshal(raw)
		if err != nil {
			return err
		}
		updates["channels.wecom.agentBindings"] = string(payload)
	}
	if _, err := config.SetValuesByDotKey(updates); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "WeCom channel config updated.")
	return nil
}
