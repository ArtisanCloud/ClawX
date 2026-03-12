package main

import "testing"

func TestNormalizeWebhookRoutePath(t *testing.T) {
	t.Run("uses_explicit_path_first", func(t *testing.T) {
		got := normalizeWebhookRoutePath("webhooks/telegram", "https://example.com/ignored", "telegram-default")
		if got != "/webhooks/telegram" {
			t.Fatalf("unexpected route path: %q", got)
		}
	})

	t.Run("falls_back_to_webhook_url_path", func(t *testing.T) {
		got := normalizeWebhookRoutePath("", "https://example.com/webhooks/telegram-main?x=1", "telegram-default")
		if got != "/webhooks/telegram-main" {
			t.Fatalf("unexpected route path: %q", got)
		}
	})

	t.Run("falls_back_to_instance_default", func(t *testing.T) {
		got := normalizeWebhookRoutePath("", "", "telegram-default")
		if got != "/webhooks/telegram/telegram-default" {
			t.Fatalf("unexpected route path: %q", got)
		}
	})
}
