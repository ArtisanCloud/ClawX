package integration

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	feishuchat "clawx/internal/interfaces/chat/feishu"
	telegramchat "clawx/internal/interfaces/chat/telegram"
	wecomchat "clawx/internal/interfaces/chat/wecom"
)

func TestChannelReplayProtection(t *testing.T) {
	t.Run("telegram_webhook_replay_rejected", func(t *testing.T) {
		adapter, err := telegramchat.NewAdapter(telegramchat.Options{
			Token:              "token-1",
			WebhookSecretToken: "secret-1",
		})
		if err != nil {
			t.Fatalf("new telegram adapter: %v", err)
		}

		body := `{
  "update_id": 9001,
  "message": {
    "message_id": 2,
    "text": "hello webhook",
    "chat": {"id": 42, "type": "private"},
    "from": {"id": 7}
  }
}`

		first := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
		first.Header.Set("X-Telegram-Bot-Api-Secret-Token", "secret-1")
		_, _, err = adapter.ParseWebhookRequest(first)
		if err != nil {
			t.Fatalf("first telegram parse: %v", err)
		}

		replay := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
		replay.Header.Set("X-Telegram-Bot-Api-Secret-Token", "secret-1")
		_, _, err = adapter.ParseWebhookRequest(replay)
		if !errors.Is(err, telegramchat.ErrWebhookReplayRejected) {
			t.Fatalf("expected telegram replay rejection, got %v", err)
		}
	})

	t.Run("feishu_webhook_replay_rejected", func(t *testing.T) {
		adapter, err := feishuchat.NewAdapter(feishuchat.Options{
			AppID:             "cli_xxx",
			AppSecret:         "app-secret",
			VerificationToken: "verify-token",
		})
		if err != nil {
			t.Fatalf("new feishu adapter: %v", err)
		}

		body := buildFeishuTextEventBody("evt-replay-it", "om-replay-it", "oc_1", "p2p", "ou_user_1", "verify-token", "/new")
		first := buildFeishuSignedRequest("app-secret", body)
		_, err = adapter.ParseWebhookRequest(first)
		if err != nil {
			t.Fatalf("first feishu parse: %v", err)
		}

		replay := buildFeishuSignedRequest("app-secret", body)
		_, err = adapter.ParseWebhookRequest(replay)
		if !errors.Is(err, feishuchat.ErrWebhookReplayRejected) {
			t.Fatalf("expected feishu replay rejection, got %v", err)
		}
	})

	t.Run("wecom_webhook_replay_rejected", func(t *testing.T) {
		adapter, err := wecomchat.NewAdapter(wecomchat.Options{
			CorpID:         "ww_test_corp",
			AgentID:        "1000002",
			Secret:         "corp-secret",
			Token:          "verify-token",
			EncodingAESKey: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
		})
		if err != nil {
			t.Fatalf("new wecom adapter: %v", err)
		}

		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		nonce := "nonce-replay-it"
		body, signature := buildSignedWeComEvent(
			t,
			"verify-token",
			"ww_test_corp",
			"abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
			timestamp,
			nonce,
			"msg-replay-it",
			"/new",
		)
		url := fmt.Sprintf("/webhooks/wecom?msg_signature=%s&timestamp=%s&nonce=%s", signature, timestamp, nonce)
		first := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
		_, err = adapter.ParseWebhookRequest(first)
		if err != nil {
			t.Fatalf("first wecom parse: %v", err)
		}

		replay := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
		_, err = adapter.ParseWebhookRequest(replay)
		if !errors.Is(err, wecomchat.ErrWebhookReplayRejected) {
			t.Fatalf("expected wecom replay rejection, got %v", err)
		}
	})
}
