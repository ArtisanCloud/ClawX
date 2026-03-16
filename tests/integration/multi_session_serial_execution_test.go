package integration

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

func TestMultiSessionSerialExecutionRejectConcurrentSameSession(t *testing.T) {
	repo := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(repo, repo, nil)
	backend := newBlockingBackend("primary")
	router := service.NewRouter(config.Snapshot{
		AllowedRoots:    []string{"."},
		DefaultCWD:      ".",
		Timeout:         2 * time.Second,
		TelegramEnabled: true,
	}, manager, backend)

	ctx := context.Background()
	conversationID := "telegram:-:-:user-serial-us3"
	windowID := "window-serial-us3"

	created, err := manager.CreateSession(ctx, command.SessionCommand{
		Mode:           command.ModeNew,
		ConversationID: conversationID,
		WindowID:       windowID,
		Backend:        backend.Name(),
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("create base session: %v", err)
	}

	firstDone := make(chan error, 1)
	go func() {
		_, runErr := router.HandleSessionFlow(ctx, command.SessionCommand{
			Mode:           command.ModeContinue,
			ConversationID: conversationID,
			WindowID:       windowID,
			Input:          "first request",
			Backend:        backend.Name(),
			CWD:            ".",
		})
		firstDone <- runErr
	}()

	select {
	case <-backend.startedCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("first execution did not start in time")
	}

	_, err = router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: conversationID,
		WindowID:       windowID,
		Input:          "second request",
		Backend:        backend.Name(),
		CWD:            ".",
	})
	if err == nil {
		t.Fatalf("second concurrent request should be rejected")
	}
	if !errors.Is(err, session.ErrSessionBusy) && !errors.Is(err, persistence.ErrLockHeldByAnotherProcess) {
		t.Fatalf("unexpected concurrent rejection error: %v", err)
	}

	backend.release()
	if err := <-firstDone; err != nil {
		t.Fatalf("first execution should succeed after release: %v", err)
	}

	if got := backend.executeCount.Load(); got != 1 {
		t.Fatalf("backend execute should be called once for same session under contention, got=%d", got)
	}

	record, err := manager.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get session record: %v", err)
	}
	if record.Status != session.StatusIdle {
		t.Fatalf("expected session to return idle, got %s", record.Status)
	}
}

type blockingBackend struct {
	name         string
	executeCount atomic.Int64

	startedOnce sync.Once
	startedCh   chan struct{}
	releaseCh   chan struct{}
}

func newBlockingBackend(name string) *blockingBackend {
	return &blockingBackend{
		name:      name,
		startedCh: make(chan struct{}),
		releaseCh: make(chan struct{}),
	}
}

func (b *blockingBackend) Name() string {
	if b.name == "" {
		return "primary"
	}
	return b.name
}

func (b *blockingBackend) Execute(ctx context.Context, req execution.Request) (execution.Result, error) {
	b.executeCount.Add(1)
	b.startedOnce.Do(func() {
		close(b.startedCh)
	})

	select {
	case <-b.releaseCh:
		now := time.Now().UTC()
		return execution.Result{
			BackendSessionID: fmt.Sprintf("backend-%s", req.SessionID),
			Output:           req.Input,
			State:            execution.ResultSuccess,
			StartedAt:        now,
			CompletedAt:      now,
		}, nil
	case <-ctx.Done():
		return execution.Result{}, ctx.Err()
	}
}

func (b *blockingBackend) release() {
	select {
	case <-b.releaseCh:
		return
	default:
		close(b.releaseCh)
	}
}

func (b *blockingBackend) Cancel(_ context.Context, _ string) error {
	b.release()
	return nil
}

func (b *blockingBackend) HealthCheck(_ context.Context) error {
	return nil
}
