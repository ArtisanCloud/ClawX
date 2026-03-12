package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"clawx/internal/application/command"
	"clawx/internal/application/service"
	"clawx/internal/infrastructure/backend"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
	chatiface "clawx/internal/interfaces/chat"
	telegramchat "clawx/internal/interfaces/chat/telegram"
)

func TestPhase1FoundationScenarios(t *testing.T) {
	t.Run("telegram_execute_round_trip_uses_real_backend_and_delivery", func(t *testing.T) {
		stack := newTestRuntime(t)

		message, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
			Channel:         "telegram",
			UserID:          "user-1",
			Text:            "hello from telegram",
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

		flowResult, err := stack.router.HandleSessionFlow(context.Background(), command.SessionCommand{
			Mode:           command.ModeContinue,
			ConversationID: decision.ConversationID,
			Input:          decision.Message.Text,
			Backend:        "primary",
			CWD:            stack.cfg.DefaultCWD,
		})
		if err != nil {
			t.Fatalf("handle session flow: %v", err)
		}

		report := stack.delivery.Deliver(
			context.Background(),
			stack.sender,
			flowResult.Session.ID,
			flowResult.Execution.Output,
			telegramchat.MaxMessageLength,
			1,
		)

		if got, want := flowResult.Execution.Output, "hello from telegram"; got != want {
			t.Fatalf("unexpected backend output: got %q want %q", got, want)
		}
		if report.Delivered != 1 {
			t.Fatalf("unexpected delivered segments: %d", report.Delivered)
		}
		if len(stack.sender.messages) != 1 {
			t.Fatalf("unexpected outbound message count: %d", len(stack.sender.messages))
		}
		if got, want := stack.sender.messages[0], "hello from telegram"; got != want {
			t.Fatalf("unexpected outbound content: got %q want %q", got, want)
		}
	})

	t.Run("telegram_control_command_routes_to_control_flow", func(t *testing.T) {
		stack := newTestRuntime(t)

		message, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
			Channel:         "telegram",
			UserID:          "user-2",
			Text:            "/new",
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
		if decision.Kind != service.DecisionControl {
			t.Fatalf("unexpected decision kind: %s", decision.Kind)
		}

		result, err := stack.router.HandleControlCommand(context.Background(), decision.Command, decision.ConversationID)
		if err != nil {
			t.Fatalf("handle control command: %v", err)
		}
		if strings.TrimSpace(result.CreatedSessionID) == "" {
			t.Fatalf("expected created session id")
		}

		formatted := chatiface.FormatControlResponse(toControlResponse(result))
		if !strings.Contains(formatted, "已创建新会话") {
			t.Fatalf("unexpected control response: %q", formatted)
		}
	})

	t.Run("bare_control_command_routes_without_slash", func(t *testing.T) {
		stack := newTestRuntime(t)

		message, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
			Channel:         "telegram",
			UserID:          "user-3",
			Text:            "new",
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
		if decision.Kind != service.DecisionControl {
			t.Fatalf("unexpected decision kind: %s", decision.Kind)
		}

		result, err := stack.router.HandleControlCommand(context.Background(), decision.Command, decision.ConversationID)
		if err != nil {
			t.Fatalf("handle control command: %v", err)
		}
		if strings.TrimSpace(result.CreatedSessionID) == "" {
			t.Fatalf("expected created session id")
		}
	})

	t.Run("compat_window_fallback_without_explicit_window_id", func(t *testing.T) {
		stack := newTestRuntime(t)

		message, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
			Channel:         "telegram",
			UserID:          "user-compat-1",
			Text:            "hello compat path",
			IsDirectMessage: true,
			IsAllowed:       true,
		})
		if err != nil {
			t.Fatalf("normalize inbound message: %v", err)
		}
		if !strings.HasPrefix(message.WindowID, "compat:") {
			t.Fatalf("expected compat window id, got %q", message.WindowID)
		}

		decision, err := stack.router.Route(context.Background(), message)
		if err != nil {
			t.Fatalf("route message: %v", err)
		}

		flowResult, err := stack.router.HandleSessionFlow(context.Background(), command.SessionCommand{
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

		current, err := stack.router.HandleControlCommand(context.Background(), "/current", decision.ConversationID)
		if err != nil {
			t.Fatalf("handle current command: %v", err)
		}
		if current.CurrentSession == nil || current.CurrentSession.ID != flowResult.Session.ID {
			t.Fatalf("compat path should expose current session: got=%v want=%s", current.CurrentSession, flowResult.Session.ID)
		}
	})
}

type testRuntime struct {
	cfg            config.Snapshot
	router         *service.Router
	delivery       *service.OutputDelivery
	sender         *recordingSender
	repository     *persistence.SessionMemoryRepository
	sessionManager *service.SessionManager
}

func newTestRuntime(t *testing.T) testRuntime {
	t.Helper()

	cfg := config.Snapshot{
		AllowedRoots:    []string{"."},
		DefaultCWD:      ".",
		Timeout:         2 * time.Second,
		ExecCommand:     "cat",
		TelegramEnabled: true,
		FeishuEnabled:   true,
		WeComEnabled:    true,
	}

	repository := persistence.NewSessionMemoryRepository()
	sessionManager := service.NewSessionManager(repository, repository, nil)
	runner := backend.NewDirectRunner("primary", cfg.Timeout, backend.Options{
		Command:     cfg.ExecCommand,
		ValidateCWD: cfg.ValidateWorkingDirectory,
	})

	return testRuntime{
		cfg:            cfg,
		router:         service.NewRouter(cfg, sessionManager, runner),
		delivery:       service.NewOutputDelivery(service.NewOutputFormatter(), service.NewOutputStreamer()),
		sender:         &recordingSender{},
		repository:     repository,
		sessionManager: sessionManager,
	}
}

type recordingSender struct {
	messages []string
	errors   []string
}

func (s *recordingSender) SendText(_ context.Context, _ string, chunk string, _ bool) error {
	s.messages = append(s.messages, chunk)
	return nil
}

func (s *recordingSender) SendError(_ context.Context, _ string, message string) error {
	s.errors = append(s.errors, message)
	return nil
}

func toControlResponse(result service.ControlFlowResult) chatiface.ControlResponse {
	response := chatiface.ControlResponse{
		CreatedSessionID:   result.CreatedSessionID,
		ResumedSessionID:   result.ResumedSessionID,
		SwitchedSessionID:  result.SwitchedSessionID,
		CancelledSessionID: result.CancelledSessionID,
		Sessions:           make([]chatiface.ControlSessionSummary, 0, len(result.Sessions)),
	}

	for _, summary := range result.Sessions {
		response.Sessions = append(response.Sessions, chatiface.ControlSessionSummary{
			ID:     summary.ID,
			Status: string(summary.Status),
		})
	}
	return response
}
