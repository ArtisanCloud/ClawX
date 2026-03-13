package feishu

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseWebhookRequestRejectsReplayEvent(t *testing.T) {
	adapter, err := NewAdapter(Options{
		AppID:             "cli_xxx",
		AppSecret:         "app-secret",
		VerificationToken: "verify-token",
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	body := `{
  "header": {
    "event_id": "evt-replay-1",
    "event_type": "im.message.receive_v1",
    "token": "verify-token"
  },
  "event": {
    "sender": {
      "sender_id": {"open_id": "ou_test_user"}
    },
    "message": {
      "message_id": "om_1",
      "message_type": "text",
      "chat_id": "oc_1",
      "chat_type": "p2p",
      "content": "{\"text\":\"/new\"}"
    }
  }
}`

	firstReq := httptest.NewRequest(http.MethodPost, "/webhooks/feishu", strings.NewReader(body))
	setSignatureHeaders(firstReq, "app-secret", body)

	first, err := adapter.ParseWebhookRequest(firstReq)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	if !first.HasMessage {
		t.Fatalf("expected first message event")
	}
	if first.Envelope.EventID != "evt-replay-1" {
		t.Fatalf("unexpected event id: %q", first.Envelope.EventID)
	}

	replayReq := httptest.NewRequest(http.MethodPost, "/webhooks/feishu", strings.NewReader(body))
	setSignatureHeaders(replayReq, "app-secret", body)
	_, err = adapter.ParseWebhookRequest(replayReq)
	if !errors.Is(err, ErrWebhookReplayRejected) {
		t.Fatalf("expected ErrWebhookReplayRejected, got %v", err)
	}
}
