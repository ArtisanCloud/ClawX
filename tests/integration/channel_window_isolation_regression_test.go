package integration

import (
	"context"
	"testing"
	"time"

	"clawx/internal/application/command"
	"clawx/internal/application/service"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
	chatiface "clawx/internal/interfaces/chat"
)

func TestChannelWindowIsolationRegression(t *testing.T) {
	ctx := context.Background()
	repo := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(repo, repo, nil)
	router := service.NewRouter(config.Snapshot{
		AllowedRoots:    []string{"."},
		DefaultCWD:      ".",
		Timeout:         2 * time.Second,
		DiscordEnabled:  true,
		TelegramEnabled: true,
		FeishuEnabled:   true,
		WeComEnabled:    true,
	}, manager, fakeBackend{name: "primary"})

	windowID := "window-shared-regression"
	channels := []string{"telegram", "discord", "feishu", "wecom"}
	firstSessionByChannel := make(map[string]string, len(channels))

	for _, channel := range channels {
		msg := mustNormalizeMessage(t, chatiface.NormalizeInput{
			Channel:         channel,
			UserID:          "shared-window-user",
			WindowID:        windowID,
			Text:            "bootstrap-" + channel,
			IsDirectMessage: true,
			IsAllowed:       true,
		})
		flow := mustContinueFlowByMessage(t, ctx, router, msg)
		firstSessionByChannel[channel] = flow.Session.ID
	}

	seen := make(map[string]string, len(firstSessionByChannel))
	for channel, sessionID := range firstSessionByChannel {
		if existingChannel, exists := seen[sessionID]; exists {
			t.Fatalf("cross-channel session collision: %s and %s share session %q", channel, existingChannel, sessionID)
		}
		seen[sessionID] = channel
	}

	for _, channel := range channels {
		msg := mustNormalizeMessage(t, chatiface.NormalizeInput{
			Channel:         channel,
			UserID:          "shared-window-user",
			WindowID:        windowID,
			Text:            "continue-" + channel,
			IsDirectMessage: true,
			IsAllowed:       true,
		})
		flow := mustContinueFlowByMessage(t, ctx, router, msg)
		if got, want := flow.Session.ID, firstSessionByChannel[channel]; got != want {
			t.Fatalf("channel %s window isolation broke: got=%q want=%q", channel, got, want)
		}
	}
}

func mustContinueFlowByMessage(t *testing.T, ctx context.Context, router *service.Router, message chatiface.Message) service.SessionFlowResult {
	t.Helper()
	decision, err := router.Route(ctx, message)
	if err != nil {
		t.Fatalf("route %s message: %v", message.Channel, err)
	}

	flow, err := router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: decision.ConversationID,
		WindowID:       decision.WindowID,
		Input:          decision.Message.Text,
		Backend:        "primary",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("handle %s flow: %v", message.Channel, err)
	}
	return flow
}
