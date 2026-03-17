package telegram

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestSendLocalFileUsesSendDocument(t *testing.T) {
	var gotPath string
	var gotCT string
	var gotChatID string
	var gotCaption string
	var gotFileName string
	var gotFileBody string
	adapter, err := NewAdapter(Options{
		Token: "token-1",
		HTTPClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				gotPath = req.URL.Path
				gotCT = req.Header.Get("Content-Type")
				if err := req.ParseMultipartForm(2 * 1024 * 1024); err != nil {
					t.Fatalf("parse multipart: %v", err)
				}
				gotChatID = req.FormValue("chat_id")
				gotCaption = req.FormValue("caption")
				file, header, err := req.FormFile("document")
				if err != nil {
					t.Fatalf("read form file: %v", err)
				}
				defer file.Close()
				body, err := io.ReadAll(file)
				if err != nil {
					t.Fatalf("read form file body: %v", err)
				}
				gotFileName = header.Filename
				gotFileBody = string(body)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				}, nil
			}),
		},
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test-long.png")
	if err := os.WriteFile(path, []byte("png-bytes"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := adapter.SendLocalFile(context.Background(), Target{ChatID: 42}, path, "产物回传"); err != nil {
		t.Fatalf("send local file: %v", err)
	}

	if gotPath != "/bottoken-1/sendDocument" {
		t.Fatalf("unexpected path: %q", gotPath)
	}
	if !strings.HasPrefix(strings.ToLower(gotCT), "multipart/form-data;") {
		t.Fatalf("unexpected content-type: %q", gotCT)
	}
	if gotChatID != "42" {
		t.Fatalf("unexpected chat id: %q", gotChatID)
	}
	if gotCaption != "产物回传" {
		t.Fatalf("unexpected caption: %q", gotCaption)
	}
	if gotFileName != "test-long.png" {
		t.Fatalf("unexpected filename: %q", gotFileName)
	}
	if gotFileBody != "png-bytes" {
		t.Fatalf("unexpected file body: %q", gotFileBody)
	}
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
