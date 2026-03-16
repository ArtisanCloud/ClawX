package contract

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"clawx/internal/application/command"
	"clawx/internal/application/service"
	"clawx/internal/domain/execution"
	"clawx/internal/domain/session"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
)

func TestMultiSessionControlContractSwitchSemantics(t *testing.T) {
	router, backend := newControlContractRouter()
	ctx := context.Background()
	conversationID := "discord:-:thread:user-contract-1"
	windowID := "window-contract-1"

	first, err := router.HandleControlCommand(ctx, "/new", conversationID, windowID)
	if err != nil {
		t.Fatalf("/new first: %v", err)
	}
	second, err := router.HandleControlCommand(ctx, "/new", conversationID, windowID)
	if err != nil {
		t.Fatalf("/new second: %v", err)
	}

	switched, err := router.HandleControlCommand(ctx, "/switch "+first.CreatedSessionID, conversationID, windowID)
	if err != nil {
		t.Fatalf("/switch: %v", err)
	}
	if switched.SwitchedSessionID != first.CreatedSessionID {
		t.Fatalf("unexpected switched session id: got=%q want=%q", switched.SwitchedSessionID, first.CreatedSessionID)
	}
	if got := backend.executeCount.Load(); got != 0 {
		t.Fatalf("switch should not trigger execution, execute_count=%d", got)
	}

	current, err := router.HandleControlCommand(ctx, "/current", conversationID, windowID)
	if err != nil {
		t.Fatalf("/current: %v", err)
	}
	if current.CurrentSession == nil || current.CurrentSession.ID != first.CreatedSessionID {
		t.Fatalf("expected current session after switch to be first session")
	}

	list, err := router.HandleControlCommand(ctx, "/list", conversationID, windowID)
	if err != nil {
		t.Fatalf("/list: %v", err)
	}
	if len(list.Sessions) < 2 {
		t.Fatalf("expected at least two sessions in list, got=%d", len(list.Sessions))
	}
	if list.CurrentSession == nil || list.CurrentSession.ID != first.CreatedSessionID {
		t.Fatalf("expected list current marker to match switched session")
	}

	cancel, err := router.HandleControlCommand(ctx, "/cancel", conversationID, windowID)
	if err != nil {
		t.Fatalf("/cancel: %v", err)
	}
	if !cancel.CancelNoop || cancel.CancelledSessionID != first.CreatedSessionID {
		t.Fatalf("cancel should target current switched session and return noop")
	}

	_, err = router.HandleControlCommand(ctx, "/switch missing-session", conversationID, windowID)
	if !errors.Is(err, session.ErrSessionNotFound) {
		t.Fatalf("switch to missing session should return session not found, got %v", err)
	}

	otherConversation := "discord:-:thread:user-contract-2"
	otherWindow := "window-contract-2"
	other, err := router.HandleControlCommand(ctx, "/new", otherConversation, otherWindow)
	if err != nil {
		t.Fatalf("create other conversation session: %v", err)
	}
	_, err = router.HandleControlCommand(ctx, "/switch "+other.CreatedSessionID, conversationID, windowID)
	if !errors.Is(err, service.ErrConversationMismatch) {
		t.Fatalf("switch to foreign conversation session should return ErrConversationMismatch, got %v", err)
	}

	if second.CreatedSessionID == "" {
		t.Fatalf("expected second created session id")
	}
}

func TestMultiSessionControlContractParseSwitchAndResume(t *testing.T) {
	conversationID := "discord:-:thread:user-contract-3"

	parsedSwitch, err := command.ParseControlCommand("/switch sess-1", conversationID, "window-x")
	if err != nil {
		t.Fatalf("parse /switch: %v", err)
	}
	if parsedSwitch.Kind != command.ControlSwitch || parsedSwitch.TargetSessionID != "sess-1" {
		t.Fatalf("unexpected /switch parse result: kind=%s target=%q", parsedSwitch.Kind, parsedSwitch.TargetSessionID)
	}

	_, err = command.ParseControlCommand("/switch", conversationID, "window-x")
	if !errors.Is(err, command.ErrInvalidControlCommand) {
		t.Fatalf("missing switch target should return invalid control command, got %v", err)
	}

	parsedResume, err := command.ParseControlCommand("/resume sess-2", conversationID, "window-x")
	if err != nil {
		t.Fatalf("parse /resume: %v", err)
	}
	if parsedResume.Kind != command.ControlResume || parsedResume.TargetSessionID != "sess-2" {
		t.Fatalf("unexpected /resume parse result: kind=%s target=%q", parsedResume.Kind, parsedResume.TargetSessionID)
	}
}

type countingBackend struct {
	executeCount atomic.Int64
}

func (b *countingBackend) Name() string {
	return "primary"
}

func (b *countingBackend) Execute(_ context.Context, request execution.Request) (execution.Result, error) {
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

func (b *countingBackend) Cancel(_ context.Context, _ string) error {
	return nil
}

func (b *countingBackend) HealthCheck(_ context.Context) error {
	return nil
}

func newControlContractRouter() (*service.Router, *countingBackend) {
	repo := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(repo, repo, nil)
	backend := &countingBackend{}

	cfg := config.Snapshot{
		AllowedRoots:   []string{"."},
		DefaultCWD:     ".",
		Timeout:        time.Second,
		DiscordEnabled: true,
	}
	return service.NewRouter(cfg, manager, backend), backend
}
