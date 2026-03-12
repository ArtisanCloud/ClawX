package main

import (
	"fmt"
	"os"
	"strings"

	"synapsex/internal/infrastructure/config"
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
			return fmt.Errorf("channel is required in non-interactive mode; use `synapsex config channel telegram|discord|feishu|wecom`")
		}
		selected, err := promptMenu(
			"选择要增量配置的 Channel:",
			[]menuOption{
				{key: "telegram", label: "Telegram", aliases: []string{"1", "telegram", "tg"}, selected: true},
				{key: "discord", label: "Discord", aliases: []string{"2", "discord", "dc"}},
				{key: "feishu", label: "Feishu（骨架）", aliases: []string{"3", "feishu", "fs"}},
				{key: "wecom", label: "WeCom（骨架）", aliases: []string{"4", "wecom", "wx", "qywx"}},
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
		fmt.Fprintln(os.Stdout, "Usage: synapsex config channel [telegram|discord|feishu|wecom]")
		return nil
	default:
		return fmt.Errorf("unknown channel %q; expected telegram, discord, feishu, or wecom", channel)
	}
}

func runConfigChannelFeishu() error {
	fmt.Fprintln(os.Stdout, "Feishu 增量配置骨架已就绪（Phase 2）。")
	fmt.Fprintln(os.Stdout, "当前请先使用 `synapsex config set channels.feishu.<key> <value>` 进行字段写入。")
	return nil
}

func runConfigChannelWeCom() error {
	fmt.Fprintln(os.Stdout, "WeCom 增量配置骨架已就绪（Phase 2）。")
	fmt.Fprintln(os.Stdout, "当前请先使用 `synapsex config set channels.wecom.<key> <value>` 进行字段写入。")
	return nil
}
