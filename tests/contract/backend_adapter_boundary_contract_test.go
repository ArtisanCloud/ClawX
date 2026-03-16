package contract

import (
	"context"
	"fmt"
	"sync"
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

func TestBackendAdapterBoundaryContract(t *testing.T) {
	backend := &backendBoundaryProbe{requests: make([]execution.Request, 0, 8)}
	router := newBackendBoundaryRouter(backend)
	ctx := context.Background()

	t.Run("execute_path_uses_backend_interface_without_channel_extensions", func(t *testing.T) {
		channels := []string{"discord", "telegram", "feishu", "wecom", "slack"}
		for _, channel := range channels {
			message := mustNormalizeBoundaryMessage(t, channel, "backend-boundary-user", "execute-"+channel)
			decision, err := router.Route(ctx, message)
			if err != nil {
				t.Fatalf("route %s: %v", channel, err)
			}
			if decision.Kind != service.DecisionExecute {
				t.Fatalf("channel %s routed to %s, want execute", channel, decision.Kind)
			}

			_, err = router.HandleSessionFlow(ctx, command.SessionCommand{
				Mode:           command.ModeContinue,
				ConversationID: decision.ConversationID,
				WindowID:       decision.WindowID,
				Input:          decision.Message.Text,
				Backend:        backend.Name(),
				CWD:            ".",
			})
			if err != nil {
				t.Fatalf("execute flow %s: %v", channel, err)
			}
		}

		if got := backend.executeCount.Load(); got != int64(len(channels)) {
			t.Fatalf("unexpected execute count: got=%d want=%d", got, len(channels))
		}

		backend.mu.Lock()
		defer backend.mu.Unlock()
		if len(backend.requests) != len(channels) {
			t.Fatalf("request capture size mismatch: got=%d want=%d", len(backend.requests), len(channels))
		}
		for i, req := range backend.requests {
			if req.SessionID == "" || req.Input == "" {
				t.Fatalf("request[%d] missing core fields: %+v", i, req)
			}
			if req.Timeout <= 0 {
				t.Fatalf("request[%d] missing timeout: %+v", i, req)
			}
		}
	})

	t.Run("control_path_does_not_invoke_backend", func(t *testing.T) {
		before := backend.executeCount.Load()
		message := mustNormalizeBoundaryMessage(t, "telegram", "backend-boundary-control", "/new")
		decision, err := router.Route(ctx, message)
		if err != nil {
			t.Fatalf("route control: %v", err)
		}
		if decision.Kind != service.DecisionControl {
			t.Fatalf("control command routed to %s", decision.Kind)
		}
		if _, err := router.HandleControlCommand(ctx, decision.Command, decision.ConversationID, decision.WindowID); err != nil {
			t.Fatalf("handle control: %v", err)
		}
		after := backend.executeCount.Load()
		if after != before {
			t.Fatalf("control path should not execute backend: before=%d after=%d", before, after)
		}
	})
}

func mustNormalizeBoundaryMessage(t *testing.T, channel, userID, text string) chatiface.Message {
	t.Helper()
	message, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
		Channel:         channel,
		UserID:          userID,
		Text:            text,
		IsDirectMessage: true,
		IsAllowed:       true,
	})
	if err != nil {
		t.Fatalf("normalize %s message: %v", channel, err)
	}
	return message
}

func newBackendBoundaryRouter(backend execution.Backend) *service.Router {
	repo := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(repo, repo, nil)
	cfg := config.Snapshot{
		AllowedRoots:    []string{"."},
		DefaultCWD:      ".",
		Timeout:         time.Second,
		DiscordEnabled:  true,
		TelegramEnabled: true,
		FeishuEnabled:   true,
		WeComEnabled:    true,
		ExtendedChannels: map[string]config.ExtendedChannelConfig{
			"slack": {Enabled: true},
		},
	}
	return service.NewRouter(cfg, manager, backend)
}

type backendBoundaryProbe struct {
	executeCount atomic.Int64

	mu       sync.Mutex
	requests []execution.Request
}

func (b *backendBoundaryProbe) Name() string {
	return "primary"
}

func (b *backendBoundaryProbe) Execute(_ context.Context, request execution.Request) (execution.Result, error) {
	b.executeCount.Add(1)
	b.mu.Lock()
	b.requests = append(b.requests, request)
	b.mu.Unlock()

	now := time.Now().UTC()
	return execution.Result{
		BackendSessionID: fmt.Sprintf("backend-%s", request.SessionID),
		Output:           request.Input,
		State:            execution.ResultSuccess,
		StartedAt:        now,
		CompletedAt:      now,
	}, nil
}

func (b *backendBoundaryProbe) Cancel(_ context.Context, _ string) error { return nil }
func (b *backendBoundaryProbe) HealthCheck(_ context.Context) error      { return nil }
