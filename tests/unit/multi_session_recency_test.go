package unit

import (
	"context"
	"testing"
	"time"

	"clawx/internal/application/command"
	"clawx/internal/application/service"
	"clawx/internal/domain/execution"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
)

func TestMultiSessionRecencyRefreshForNewResumeSwitchAndExecuteSuccess(t *testing.T) {
	repo := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(repo, repo, nil)
	router := service.NewRouter(config.Snapshot{
		AllowedRoots:    []string{"."},
		DefaultCWD:      ".",
		Timeout:         time.Second,
		TelegramEnabled: true,
	}, manager, recencyBackend{name: "primary"})
	ctx := context.Background()

	conversationID := "telegram:-:-:user-recency"
	windowA := "window-a"

	created, err := manager.CreateSession(ctx, command.SessionCommand{
		Mode:           command.ModeNew,
		ConversationID: conversationID,
		WindowID:       windowA,
		Backend:        "primary",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	bindingAfterNew, err := manager.GetWindowBinding(ctx, windowA)
	if err != nil {
		t.Fatalf("get binding after new: %v", err)
	}

	time.Sleep(2 * time.Millisecond)
	resumed, err := manager.ResumeSession(ctx, command.SessionCommand{
		Mode:            command.ModeResume,
		ConversationID:  conversationID,
		WindowID:        windowA,
		ResumeSessionID: created.ID,
		Backend:         "primary",
		CWD:             ".",
	})
	if err != nil {
		t.Fatalf("resume session: %v", err)
	}
	if resumed.ID != created.ID {
		t.Fatalf("resume should target original session: got=%q want=%q", resumed.ID, created.ID)
	}
	recordAfterResume, err := manager.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get session after resume: %v", err)
	}
	bindingAfterResume, err := manager.GetWindowBinding(ctx, windowA)
	if err != nil {
		t.Fatalf("get binding after resume: %v", err)
	}
	assertAfter(t, recordAfterResume.LastUsedAt, created.LastUsedAt, "resume should refresh session last_used_at")
	assertAfter(t, bindingAfterResume.LastUsedAt, bindingAfterNew.LastUsedAt, "resume should refresh window last_used_at")

	other, err := manager.CreateSession(ctx, command.SessionCommand{
		Mode:           command.ModeNew,
		ConversationID: conversationID,
		WindowID:       "window-b",
		Backend:        "primary",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("create secondary session: %v", err)
	}

	time.Sleep(2 * time.Millisecond)
	bindingAfterSwitch, err := manager.BindWindowToSession(ctx, windowA, conversationID, other.ID)
	if err != nil {
		t.Fatalf("bind window to secondary session (switch): %v", err)
	}
	if bindingAfterSwitch.CurrentSessionID != other.ID {
		t.Fatalf("switch should move current session: got=%q want=%q", bindingAfterSwitch.CurrentSessionID, other.ID)
	}
	assertAfter(t, bindingAfterSwitch.LastUsedAt, bindingAfterResume.LastUsedAt, "switch should refresh window last_used_at")

	time.Sleep(2 * time.Millisecond)
	flowResult, err := router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: conversationID,
		WindowID:       windowA,
		Input:          "execute-success",
		Backend:        "primary",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("execute success flow: %v", err)
	}
	if flowResult.Session.ID != other.ID {
		t.Fatalf("execute should use switched session: got=%q want=%q", flowResult.Session.ID, other.ID)
	}

	recordAfterExecute, err := manager.GetByID(ctx, other.ID)
	if err != nil {
		t.Fatalf("get session after execute: %v", err)
	}
	bindingAfterExecute, err := manager.GetWindowBinding(ctx, windowA)
	if err != nil {
		t.Fatalf("get binding after execute: %v", err)
	}
	assertAfter(t, recordAfterExecute.LastUsedAt, other.LastUsedAt, "execute success should refresh session last_used_at")
	assertAfter(t, bindingAfterExecute.LastUsedAt, bindingAfterSwitch.LastUsedAt, "execute success should refresh window last_used_at")
}

func assertAfter(t *testing.T, got, previous time.Time, message string) {
	t.Helper()
	if !got.After(previous) {
		t.Fatalf("%s: got=%s previous=%s", message, got.Format(time.RFC3339Nano), previous.Format(time.RFC3339Nano))
	}
}

type recencyBackend struct {
	name string
}

func (b recencyBackend) Name() string {
	if b.name == "" {
		return "primary"
	}
	return b.name
}

func (b recencyBackend) Execute(_ context.Context, req execution.Request) (execution.Result, error) {
	now := time.Now().UTC()
	return execution.Result{
		BackendSessionID: "backend-" + req.SessionID,
		Output:           req.Input,
		State:            execution.ResultSuccess,
		StartedAt:        now,
		CompletedAt:      now,
	}, nil
}

func (b recencyBackend) Cancel(_ context.Context, _ string) error {
	return nil
}

func (b recencyBackend) HealthCheck(_ context.Context) error {
	return nil
}
