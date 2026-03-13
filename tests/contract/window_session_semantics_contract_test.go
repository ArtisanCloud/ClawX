package contract

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"clawx/internal/application/command"
	"clawx/internal/application/service"
	"clawx/internal/domain/execution"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
	chatiface "clawx/internal/interfaces/chat"
)

func TestWindowSessionSemanticsContract(t *testing.T) {
	router, backend := newWindowSemanticsContractRouter()
	ctx := context.Background()

	channels := []string{"telegram", "discord", "feishu", "wecom"}
	for _, channel := range channels {
		t.Run(channel, func(t *testing.T) {
			userID := "window-user-" + channel

			compatMsg := mustNormalizeWindowMessage(t, channel, userID, "compat-first", "")
			if !strings.HasPrefix(compatMsg.WindowID, "compat:") {
				t.Fatalf("expected compat window id for %s, got %q", channel, compatMsg.WindowID)
			}
			compatSessionA := mustRunContinueFlow(t, ctx, router, compatMsg)

			explicitWindowID := "window-explicit-" + channel
			explicitControl := mustNormalizeWindowMessage(t, channel, userID, "/new", explicitWindowID)
			created, err := router.HandleControlCommand(ctx, "/new", explicitControl.ConversationID, explicitControl.WindowID)
			if err != nil {
				t.Fatalf("%s /new in explicit window: %v", channel, err)
			}
			if created.CreatedSessionID == "" || created.CreatedSessionID == compatSessionA {
				t.Fatalf("%s expected distinct explicit session, got=%q compat=%q", channel, created.CreatedSessionID, compatSessionA)
			}

			compatSessionB := mustRunContinueFlow(t, ctx, router, mustNormalizeWindowMessage(t, channel, userID, "compat-second", ""))
			if compatSessionB != compatSessionA {
				t.Fatalf("%s compat path should stay on original session: got=%q want=%q", channel, compatSessionB, compatSessionA)
			}

			explicitSession := mustRunContinueFlow(t, ctx, router, mustNormalizeWindowMessage(t, channel, userID, "explicit-second", explicitWindowID))
			if explicitSession != created.CreatedSessionID {
				t.Fatalf("%s explicit window should stay on explicit session: got=%q want=%q", channel, explicitSession, created.CreatedSessionID)
			}
		})
	}

	if got := backend.executeCount.Load(); got != int64(len(channels)*3) {
		t.Fatalf("unexpected execute count: got=%d want=%d", got, len(channels)*3)
	}
}

func mustRunContinueFlow(t *testing.T, ctx context.Context, router *service.Router, message chatiface.Message) string {
	t.Helper()
	decision, err := router.Route(ctx, message)
	if err != nil {
		t.Fatalf("route message %q: %v", message.Text, err)
	}
	if decision.Kind != service.DecisionExecute {
		t.Fatalf("message %q routed to %s, want execute", message.Text, decision.Kind)
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
		t.Fatalf("handle session flow for %q: %v", message.Text, err)
	}
	return flow.Session.ID
}

func mustNormalizeWindowMessage(t *testing.T, channel, userID, text, windowID string) chatiface.Message {
	t.Helper()
	message, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
		Channel:         channel,
		UserID:          userID,
		Text:            text,
		WindowID:        windowID,
		IsDirectMessage: true,
		IsAllowed:       true,
	})
	if err != nil {
		t.Fatalf("normalize %s message: %v", channel, err)
	}
	return message
}

type windowSemanticsBackend struct {
	executeCount atomic.Int64
}

func (b *windowSemanticsBackend) Name() string {
	return "primary"
}

func (b *windowSemanticsBackend) Execute(_ context.Context, req execution.Request) (execution.Result, error) {
	b.executeCount.Add(1)
	now := time.Now().UTC()
	return execution.Result{
		BackendSessionID: fmt.Sprintf("backend-%s", req.SessionID),
		Output:           req.Input,
		State:            execution.ResultSuccess,
		StartedAt:        now,
		CompletedAt:      now,
	}, nil
}

func (b *windowSemanticsBackend) Cancel(_ context.Context, _ string) error { return nil }
func (b *windowSemanticsBackend) HealthCheck(_ context.Context) error      { return nil }

func newWindowSemanticsContractRouter() (*service.Router, *windowSemanticsBackend) {
	repo := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(repo, repo, nil)
	backend := &windowSemanticsBackend{}
	cfg := config.Snapshot{
		AllowedRoots:    []string{"."},
		DefaultCWD:      ".",
		Timeout:         time.Second,
		DiscordEnabled:  true,
		TelegramEnabled: true,
		FeishuEnabled:   true,
		WeComEnabled:    true,
	}
	return service.NewRouter(cfg, manager, backend), backend
}
