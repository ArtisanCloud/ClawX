package feishu

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseWebhookRequestChallengeSuccess(t *testing.T) {
	adapter, err := NewAdapter(Options{
		AppID:             "cli_xxx",
		AppSecret:         "app-secret",
		VerificationToken: "verify-token",
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	body := `{"type":"url_verification","token":"verify-token","challenge":"challenge-ok"}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/feishu", strings.NewReader(body))
	setSignatureHeaders(req, "app-secret", body)

	result, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("parse webhook request: %v", err)
	}
	if !result.IsChallenge {
		t.Fatalf("expected challenge request")
	}
	if result.Challenge != "challenge-ok" {
		t.Fatalf("unexpected challenge: %q", result.Challenge)
	}
}

func TestParseWebhookRequestSignatureMismatch(t *testing.T) {
	adapter, err := NewAdapter(Options{
		AppID:             "cli_xxx",
		AppSecret:         "app-secret",
		VerificationToken: "verify-token",
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	body := `{"type":"url_verification","token":"verify-token","challenge":"challenge-ok"}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/feishu", strings.NewReader(body))
	req.Header.Set("X-Lark-Request-Timestamp", "1700000000")
	req.Header.Set("X-Lark-Request-Nonce", "nonce-1")
	req.Header.Set("X-Lark-Signature", "invalid")

	_, err = adapter.ParseWebhookRequest(req)
	if !errors.Is(err, ErrWebhookUnauthorized) {
		t.Fatalf("expected ErrWebhookUnauthorized, got %v", err)
	}
}

func TestParseWebhookRequestMessageSuccess(t *testing.T) {
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
    "event_id": "evt-1",
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
	req := httptest.NewRequest(http.MethodPost, "/webhooks/feishu", strings.NewReader(body))
	setSignatureHeaders(req, "app-secret", body)

	result, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("parse webhook request: %v", err)
	}
	if !result.HasMessage {
		t.Fatalf("expected message event")
	}
	if result.Envelope.Message.Channel != "feishu" {
		t.Fatalf("unexpected channel: %q", result.Envelope.Message.Channel)
	}
	if result.Envelope.Message.Text != "/new" {
		t.Fatalf("unexpected text: %q", result.Envelope.Message.Text)
	}
	if result.Envelope.Target.ChatID != "oc_1" {
		t.Fatalf("unexpected chat id: %q", result.Envelope.Target.ChatID)
	}
}

func setSignatureHeaders(req *http.Request, appSecret string, body string) {
	timestamp := "1700000000"
	nonce := "nonce-1"
	signature := signBody(appSecret, timestamp, nonce, body)

	req.Header.Set("X-Lark-Request-Timestamp", timestamp)
	req.Header.Set("X-Lark-Request-Nonce", nonce)
	req.Header.Set("X-Lark-Signature", signature)
}

func signBody(secret, timestamp, nonce, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + nonce + body))
	return hex.EncodeToString(mac.Sum(nil))
}
