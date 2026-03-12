package telegram

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestParseWebhookRequestRejectsMethod(t *testing.T) {
	adapter, err := NewAdapter(Options{
		Token:              "token-1",
		WebhookSecretToken: "secret-1",
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	_, _, err = adapter.ParseWebhookRequest(req)
	if !errors.Is(err, ErrWebhookMethod) {
		t.Fatalf("expected ErrWebhookMethod, got %v", err)
	}
}

func TestParseWebhookRequestRejectsInvalidJSON(t *testing.T) {
	adapter, err := NewAdapter(Options{
		Token:              "token-1",
		WebhookSecretToken: "secret-1",
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "secret-1")

	_, _, err = adapter.ParseWebhookRequest(req)
	if !errors.Is(err, ErrInvalidWebhook) {
		t.Fatalf("expected ErrInvalidWebhook, got %v", err)
	}
}

func TestSetWebhookRetriesTransientError(t *testing.T) {
	var calls int32
	adapter, err := NewAdapter(Options{
		Token: "token-1",
		HTTPClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				_ = req
				current := atomic.AddInt32(&calls, 1)
				if current == 1 {
					return nil, errors.New("connection reset by peer")
				}
				body := io.NopCloser(strings.NewReader(`{"ok":true}`))
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       body,
				}, nil
			}),
		},
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	if err := adapter.SetWebhook(context.Background(), "https://example.com/webhooks/telegram"); err != nil {
		t.Fatalf("set webhook: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 calls with one retry, got %d", got)
	}
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
