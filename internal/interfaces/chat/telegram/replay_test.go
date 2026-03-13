package telegram

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseWebhookRequestRejectsReplayEvent(t *testing.T) {
	adapter, err := NewAdapter(Options{
		Token:              "token-1",
		WebhookSecretToken: "secret-1",
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	body := `{
  "update_id": 1001,
  "message": {
    "message_id": 2,
    "text": "hello webhook",
    "chat": {"id": 42, "type": "private"},
    "from": {"id": 7}
  }
}`

	firstReq := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	firstReq.Header.Set("X-Telegram-Bot-Api-Secret-Token", "secret-1")
	first, ok, err := adapter.ParseWebhookRequest(firstReq)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	if !ok {
		t.Fatalf("first parse should produce message")
	}
	if first.EventID != "1001" {
		t.Fatalf("unexpected event id: %q", first.EventID)
	}

	replayReq := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	replayReq.Header.Set("X-Telegram-Bot-Api-Secret-Token", "secret-1")
	_, _, err = adapter.ParseWebhookRequest(replayReq)
	if !errors.Is(err, ErrWebhookReplayRejected) {
		t.Fatalf("expected ErrWebhookReplayRejected, got %v", err)
	}
}
