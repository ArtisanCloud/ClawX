package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"synapsex/internal/infrastructure/config"
)

func runConfigChannelTelegram() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Telegram 增量配置（回车保持原值，仅按当前模式提示必要字段）")

	enabled, err := promptBool("启用 Telegram?", cfg.TelegramEnabled)
	if err != nil {
		return err
	}

	token, err := promptString("Telegram Bot Token (留空保持): ")
	if err != nil {
		return err
	}
	botUsername, err := promptString("Telegram Bot Username (留空保持): ")
	if err != nil {
		return err
	}
	defaultAgent, err := promptString(fmt.Sprintf("Telegram defaultAgent (留空保持，当前 %s): ", firstNonEmpty(cfg.TelegramDefaultAgentID, cfg.DefaultAgentID, "main")))
	if err != nil {
		return err
	}
	requireMention, err := promptBool("群聊要求命令或@提及?", cfg.TelegramRequireCommandMention)
	if err != nil {
		return err
	}

	currentMode := strings.ToLower(strings.TrimSpace(cfg.TelegramMode))
	if currentMode == "" {
		currentMode = "polling"
	}
	mode, err := promptMenu(
		fmt.Sprintf("Telegram mode (当前 %s):", currentMode),
		[]menuOption{
			{key: "polling", label: "Polling", aliases: []string{"1", "polling"}, selected: currentMode == "polling"},
			{key: "webhook", label: "Webhook", aliases: []string{"2", "webhook"}, selected: currentMode == "webhook"},
		},
	)
	if err != nil {
		return err
	}
	if mode == "" {
		mode = currentMode
	}

	pollingSeconds := int(defaultDurationSeconds(cfg.TelegramPollingTimeout, 30*time.Second))
	webhookURL := ""
	webhookPath := ""
	webhookSecret := ""
	if mode == "polling" {
		pollingSeconds, err = promptIntDefault("pollingSeconds", pollingSeconds)
		if err != nil {
			return err
		}
	} else {
		webhookURL, err = promptString(fmt.Sprintf("webhookUrl (当前 %s): ", firstNonEmpty(cfg.TelegramWebhookURL, "空")))
		if err != nil {
			return err
		}
		if strings.TrimSpace(webhookURL) == "" {
			webhookURL = strings.TrimSpace(cfg.TelegramWebhookURL)
		}
		if err := validateTelegramWebhookURL(webhookURL); err != nil {
			return err
		}

		webhookPath, err = promptString(fmt.Sprintf("webhookPath (当前 %s): ", firstNonEmpty(cfg.TelegramWebhookPath, "/webhooks/telegram")))
		if err != nil {
			return err
		}
		if strings.TrimSpace(webhookPath) == "" {
			webhookPath = firstNonEmpty(cfg.TelegramWebhookPath, "/webhooks/telegram")
		}
		webhookPath = normalizeTelegramWebhookPathInput(webhookPath)
		if strings.TrimSpace(webhookPath) == "" {
			return fmt.Errorf("webhook mode requires webhookPath")
		}

		webhookSecret, err = promptString("webhookSecret (留空保持): ")
		if err != nil {
			return err
		}
	}

	chatIDsRaw, err := promptString(fmt.Sprintf("allowedChatIds 逗号分隔 (留空保持，当前 %s): ", summarizeList(cfg.TelegramAllowedChatIDs)))
	if err != nil {
		return err
	}

	updates := map[string]string{
		"channels.telegram.enabled":                 strconv.FormatBool(enabled),
		"channels.telegram.mode":                    mode,
		"channels.telegram.requireCommandOrMention": strconv.FormatBool(requireMention),
	}
	if mode == "polling" {
		updates["channels.telegram.pollingSeconds"] = strconv.Itoa(pollingSeconds)
	} else {
		updates["channels.telegram.webhookUrl"] = strings.TrimSpace(webhookURL)
		updates["channels.telegram.webhookPath"] = strings.TrimSpace(webhookPath)
		if strings.TrimSpace(webhookSecret) != "" {
			updates["channels.telegram.webhookSecret"] = strings.TrimSpace(webhookSecret)
		}
	}
	if strings.TrimSpace(token) != "" {
		updates["channels.telegram.token"] = strings.TrimSpace(token)
	}
	if strings.TrimSpace(botUsername) != "" {
		updates["channels.telegram.botUsername"] = strings.TrimSpace(botUsername)
	}
	if strings.TrimSpace(defaultAgent) != "" {
		updates["channels.telegram.defaultAgent"] = strings.TrimSpace(defaultAgent)
	}
	if strings.TrimSpace(chatIDsRaw) != "" {
		values := splitCSV(strings.TrimSpace(chatIDsRaw))
		payload, err := json.Marshal(values)
		if err != nil {
			return err
		}
		updates["channels.telegram.allowedChatIds"] = string(payload)
	}

	if _, err := config.SetValuesByDotKey(updates); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "Telegram channel config updated.")
	return nil
}

func validateTelegramWebhookURL(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fmt.Errorf("webhook mode requires webhookUrl")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("invalid webhookUrl: %w", err)
	}
	if !parsed.IsAbs() || !strings.EqualFold(parsed.Scheme, "https") || strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("webhookUrl must be an absolute HTTPS url")
	}
	return nil
}

func normalizeTelegramWebhookPathInput(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	if value != "/" {
		value = strings.TrimRight(value, "/")
	}
	return value
}
