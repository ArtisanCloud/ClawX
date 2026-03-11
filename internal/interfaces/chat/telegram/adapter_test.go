package telegram

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseWebhookRequestSuccess(t *testing.T) {
	adapter, err := NewAdapter(Options{
		Token:              "token-1",
		WebhookSecretToken: "secret-1",
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	req := httptest.NewRequest("POST", "/webhook", strings.NewReader(`{
  "update_id": 1,
  "message": {
    "message_id": 2,
    "text": "hello webhook",
    "chat": {"id": 42, "type": "private"},
    "from": {"id": 7}
  }
}`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "secret-1")

	envelope, ok, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("parse webhook request: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true for message update")
	}
	if envelope.Target.ChatID != 42 {
		t.Fatalf("unexpected chat id: %d", envelope.Target.ChatID)
	}
	if envelope.Message.Text != "hello webhook" {
		t.Fatalf("unexpected text: %q", envelope.Message.Text)
	}
}

func TestParseWebhookRequestSecretMismatch(t *testing.T) {
	adapter, err := NewAdapter(Options{
		Token:              "token-1",
		WebhookSecretToken: "secret-1",
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	req := httptest.NewRequest("POST", "/webhook", strings.NewReader(`{"update_id":1}`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong-secret")

	_, _, err = adapter.ParseWebhookRequest(req)
	if !errors.Is(err, ErrWebhookUnauthorized) {
		t.Fatalf("expected ErrWebhookUnauthorized, got %v", err)
	}
}
