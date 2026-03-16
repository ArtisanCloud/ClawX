package integration

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	telegramchat "clawx/internal/interfaces/chat/telegram"
)

func TestTelegramRuntimeRetryAndIsolation(t *testing.T) {
	t.Run("send_direct_retries_transient_error", func(t *testing.T) {
		var calls int32
		adapter, err := telegramchat.NewAdapter(telegramchat.Options{
			Token:   "token-1",
			BaseURL: "https://example.com",
			HTTPClient: &http.Client{
				Transport: integrationRoundTripper(func(req *http.Request) (*http.Response, error) {
					_ = req
					current := atomic.AddInt32(&calls, 1)
					if current == 1 {
						return nil, errors.New("connection reset by peer")
					}
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

		err = adapter.SendDirect(context.Background(), telegramchat.Target{ChatID: 1001}, "hello")
		if err != nil {
			t.Fatalf("send direct: %v", err)
		}
		if got := atomic.LoadInt32(&calls); got != 2 {
			t.Fatalf("expected one retry (2 calls), got %d", got)
		}
	})

	t.Run("one_adapter_failure_does_not_block_another", func(t *testing.T) {
		badAdapter, err := telegramchat.NewAdapter(telegramchat.Options{
			Token:   "token-bad",
			BaseURL: "https://example.com",
			HTTPClient: &http.Client{
				Transport: integrationRoundTripper(func(req *http.Request) (*http.Response, error) {
					_ = req
					return nil, errors.New("connection reset by peer")
				}),
			},
		})
		if err != nil {
			t.Fatalf("new bad adapter: %v", err)
		}

		var goodCalls int32
		goodAdapter, err := telegramchat.NewAdapter(telegramchat.Options{
			Token:   "token-good",
			BaseURL: "https://example.com",
			HTTPClient: &http.Client{
				Transport: integrationRoundTripper(func(req *http.Request) (*http.Response, error) {
					_ = req
					atomic.AddInt32(&goodCalls, 1)
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
					}, nil
				}),
			},
		})
		if err != nil {
			t.Fatalf("new good adapter: %v", err)
		}

		var wg sync.WaitGroup
		wg.Add(2)

		var badErr error
		var goodErr error
		go func() {
			defer wg.Done()
			badErr = badAdapter.SendDirect(context.Background(), telegramchat.Target{ChatID: 1002}, "hello-bad")
		}()
		go func() {
			defer wg.Done()
			goodErr = goodAdapter.SendDirect(context.Background(), telegramchat.Target{ChatID: 1003}, "hello-good")
		}()
		wg.Wait()

		if badErr == nil {
			t.Fatalf("expected bad adapter to fail")
		}
		if goodErr != nil {
			t.Fatalf("good adapter should still succeed, got %v", goodErr)
		}
		if got := atomic.LoadInt32(&goodCalls); got == 0 {
			t.Fatalf("expected good adapter to send at least one message")
		}
	})
}

type integrationRoundTripper func(req *http.Request) (*http.Response, error)

func (f integrationRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
