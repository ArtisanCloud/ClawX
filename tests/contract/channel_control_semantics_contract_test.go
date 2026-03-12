package contract

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"synapsex/internal/application/service"
	"synapsex/internal/domain/execution"
	"synapsex/internal/infrastructure/config"
	"synapsex/internal/infrastructure/persistence"
	chatiface "synapsex/internal/interfaces/chat"
)

func TestChannelControlSemanticsContract(t *testing.T) {
	router, backend := newCrossChannelControlRouter()
	ctx := context.Background()

	cases := []struct {
		name    string
		message func(text string) chatiface.Message
	}{
		{
			name: "discord",
			message: func(text string) chatiface.Message {
				msg, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
					Channel:         "discord",
					UserID:          "user-discord",
					Text:            text,
					IsDirectMessage: true,
					IsAllowed:       true,
				})
				if err != nil {
					t.Fatalf("normalize discord message: %v", err)
				}
				return msg
			},
		},
		{
			name: "telegram",
			message: func(text string) chatiface.Message {
				msg, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
					Channel:         "telegram",
					UserID:          "user-telegram",
					Text:            text,
					IsDirectMessage: true,
					IsAllowed:       true,
				})
				if err != nil {
					t.Fatalf("normalize telegram message: %v", err)
				}
				return msg
			},
		},
		{
			name: "feishu",
			message: func(text string) chatiface.Message {
				msg, err := chatiface.NormalizeFeishuTextEvent(chatiface.FeishuNormalizeInput{
					ChatID:   "oc-test-feishu",
					ChatType: "p2p",
					UserID:   "ou_feishu_user",
					Text:     text,
				})
				if err != nil {
					t.Fatalf("normalize feishu message: %v", err)
				}
				return msg
			},
		},
		{
			name: "wecom",
			message: func(text string) chatiface.Message {
				msg, err := chatiface.NormalizeWeComTextEvent(chatiface.WeComNormalizeInput{
					FromUserID: "zhangsan",
					Text:       text,
				})
				if err != nil {
					t.Fatalf("normalize wecom message: %v", err)
				}
				return msg
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			current0 := mustControl(t, router, ctx, tc.message("/current"))
			if !current0.CurrentChecked || current0.CurrentSession != nil {
				t.Fatalf("%s: expected empty current before /new", tc.name)
			}

			createdA := mustControl(t, router, ctx, tc.message("/new"))
			if createdA.CreatedSessionID == "" {
				t.Fatalf("%s: expected created session A", tc.name)
			}
			createdB := mustControl(t, router, ctx, tc.message("/new"))
			if createdB.CreatedSessionID == "" || createdB.CreatedSessionID == createdA.CreatedSessionID {
				t.Fatalf("%s: expected distinct created session B", tc.name)
			}

			listed := mustControl(t, router, ctx, tc.message("/list"))
			if len(listed.Sessions) < 2 {
				t.Fatalf("%s: expected at least two sessions in /list", tc.name)
			}
			if listed.CurrentSession == nil || listed.CurrentSession.ID != createdB.CreatedSessionID {
				t.Fatalf("%s: expected current session B after second /new", tc.name)
			}

			switched := mustControl(t, router, ctx, tc.message("/switch "+createdA.CreatedSessionID))
			if switched.SwitchedSessionID != createdA.CreatedSessionID {
				t.Fatalf("%s: unexpected switched session id", tc.name)
			}

			current1 := mustControl(t, router, ctx, tc.message("/current"))
			if current1.CurrentSession == nil || current1.CurrentSession.ID != createdA.CreatedSessionID {
				t.Fatalf("%s: expected switched session visible in /current", tc.name)
			}

			resumed := mustControl(t, router, ctx, tc.message("/resume "+createdB.CreatedSessionID))
			if resumed.ResumedSessionID != createdB.CreatedSessionID {
				t.Fatalf("%s: unexpected resumed session id", tc.name)
			}

			current2 := mustControl(t, router, ctx, tc.message("/current"))
			if current2.CurrentSession == nil || current2.CurrentSession.ID != createdB.CreatedSessionID {
				t.Fatalf("%s: expected resumed session visible in /current", tc.name)
			}

			cancelled := mustControl(t, router, ctx, tc.message("/cancel"))
			if !cancelled.CancelNoop || cancelled.CancelledSessionID != createdB.CreatedSessionID {
				t.Fatalf("%s: expected /cancel noop on idle current session", tc.name)
			}
		})
	}

	if got := backend.executeCount.Load(); got != 0 {
		t.Fatalf("control contract should not execute backend, got execute_count=%d", got)
	}
}

func mustControl(t *testing.T, router *service.Router, ctx context.Context, message chatiface.Message) service.ControlFlowResult {
	t.Helper()
	decision, err := router.Route(ctx, message)
	if err != nil {
		t.Fatalf("route control %q: %v", message.Text, err)
	}
	if decision.Kind != service.DecisionControl {
		t.Fatalf("message %q routed to %s, want control", message.Text, decision.Kind)
	}
	result, err := router.HandleControlCommand(ctx, decision.Command, decision.ConversationID, decision.WindowID)
	if err != nil {
		t.Fatalf("handle control %q: %v", message.Text, err)
	}
	return result
}

type crossChannelCountingBackend struct {
	executeCount atomic.Int64
}

func (b *crossChannelCountingBackend) Name() string { return "primary" }

func (b *crossChannelCountingBackend) Execute(_ context.Context, request execution.Request) (execution.Result, error) {
	b.executeCount.Add(1)
	now := time.Now().UTC()
	return execution.Result{
		BackendSessionID: fmt.Sprintf("backend-%s", request.SessionID),
		Output:           request.Input,
		State:            execution.ResultSuccess,
		StartedAt:        now,
		CompletedAt:      now,
	}, nil
}

func (b *crossChannelCountingBackend) Cancel(_ context.Context, _ string) error { return nil }
func (b *crossChannelCountingBackend) HealthCheck(_ context.Context) error      { return nil }

func newCrossChannelControlRouter() (*service.Router, *crossChannelCountingBackend) {
	repo := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(repo, repo, nil)
	backend := &crossChannelCountingBackend{}
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
