package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"synapsex/internal/application/command"
	"synapsex/internal/application/service"
	chatiface "synapsex/internal/interfaces/chat"
	telegramchat "synapsex/internal/interfaces/chat/telegram"
)

func TestTelegramDualModeRouting(t *testing.T) {
	stack := newTestRuntime(t)

	t.Run("polling_mode_routes_execute", func(t *testing.T) {
		message, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
			Channel:         "telegram",
			UserID:          "polling-user",
			Text:            "hello polling",
			IsDirectMessage: true,
			IsAllowed:       true,
		})
		if err != nil {
			t.Fatalf("normalize inbound message: %v", err)
		}

		decision, err := stack.router.Route(context.Background(), message)
		if err != nil {
			t.Fatalf("route message: %v", err)
		}
		if decision.Kind != service.DecisionExecute {
			t.Fatalf("unexpected decision kind: %s", decision.Kind)
		}

		result, err := stack.router.HandleSessionFlow(context.Background(), command.SessionCommand{
			Mode:           command.ModeContinue,
			ConversationID: decision.ConversationID,
			WindowID:       decision.WindowID,
			Input:          decision.Message.Text,
			Backend:        "primary",
			CWD:            stack.cfg.DefaultCWD,
		})
		if err != nil {
			t.Fatalf("handle session flow: %v", err)
		}
		if result.Execution.Output != "hello polling" {
			t.Fatalf("unexpected output: %q", result.Execution.Output)
		}
	})

	t.Run("webhook_mode_routes_control", func(t *testing.T) {
		adapter, err := telegramchat.NewAdapter(telegramchat.Options{
			Token:              "token-1",
			WebhookSecretToken: "secret-1",
		})
		if err != nil {
			t.Fatalf("new adapter: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/webhooks/telegram", strings.NewReader(`{
  "update_id": 101,
  "message": {
    "message_id": 7,
    "text": "/new",
    "chat": {"id": 42, "type": "private"},
    "from": {"id": 99}
  }
}`))
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "secret-1")

		envelope, ok, err := adapter.ParseWebhookRequest(req)
		if err != nil {
			t.Fatalf("parse webhook request: %v", err)
		}
		if !ok {
			t.Fatalf("expected webhook message to be handled")
		}

		decision, err := stack.router.Route(context.Background(), envelope.Message)
		if err != nil {
			t.Fatalf("route webhook message: %v", err)
		}
		if decision.Kind != service.DecisionControl {
			t.Fatalf("unexpected decision kind: %s", decision.Kind)
		}

		result, err := stack.router.HandleControlCommand(context.Background(), decision.Command, decision.ConversationID, decision.WindowID)
		if err != nil {
			t.Fatalf("handle control command: %v", err)
		}
		if strings.TrimSpace(result.CreatedSessionID) == "" {
			t.Fatalf("expected created session id")
		}
	})
}
