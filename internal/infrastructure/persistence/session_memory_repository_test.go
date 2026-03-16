package persistence

import (
	"context"
	"errors"
	"testing"
	"time"

	"clawx/internal/domain/session"
)

func TestSessionMemoryRepositoryWindowBindingSetAndGet(t *testing.T) {
	t.Parallel()

	repo := NewSessionMemoryRepository()
	now := time.Now().UTC().Truncate(time.Second)
	binding := session.WindowBinding{
		WindowID:         "window-a",
		CurrentSessionID: "sess-1",
		ConversationID:   "discord:-:thread:user-1",
		UpdatedAt:        now,
		LastUsedAt:       now,
	}

	if err := repo.SetWindowBinding(context.Background(), binding); err != nil {
		t.Fatalf("set window binding: %v", err)
	}

	got, err := repo.GetWindowBinding(context.Background(), "window-a")
	if err != nil {
		t.Fatalf("get window binding: %v", err)
	}
	if got.WindowID != binding.WindowID {
		t.Fatalf("unexpected window id: got=%q want=%q", got.WindowID, binding.WindowID)
	}
	if got.CurrentSessionID != binding.CurrentSessionID {
		t.Fatalf("unexpected current session: got=%q want=%q", got.CurrentSessionID, binding.CurrentSessionID)
	}
	if got.ConversationID != binding.ConversationID {
		t.Fatalf("unexpected conversation id: got=%q want=%q", got.ConversationID, binding.ConversationID)
	}
}

func TestSessionMemoryRepositoryListWindowBindingsByConversation(t *testing.T) {
	t.Parallel()

	repo := NewSessionMemoryRepository()
	base := time.Now().UTC().Truncate(time.Second)

	bindings := []session.WindowBinding{
		{
			WindowID:         "window-a",
			CurrentSessionID: "sess-1",
			ConversationID:   "discord:-:thread:user-1",
			UpdatedAt:        base,
			LastUsedAt:       base,
		},
		{
			WindowID:         "window-b",
			CurrentSessionID: "sess-2",
			ConversationID:   "discord:-:thread:user-1",
			UpdatedAt:        base.Add(time.Second),
			LastUsedAt:       base.Add(time.Second),
		},
		{
			WindowID:         "window-c",
			CurrentSessionID: "sess-3",
			ConversationID:   "telegram:-:chat:user-1",
			UpdatedAt:        base.Add(2 * time.Second),
			LastUsedAt:       base.Add(2 * time.Second),
		},
	}

	for _, binding := range bindings {
		if err := repo.SetWindowBinding(context.Background(), binding); err != nil {
			t.Fatalf("set window binding %s: %v", binding.WindowID, err)
		}
	}

	got, err := repo.ListWindowBindingsByConversation(context.Background(), "discord:-:thread:user-1")
	if err != nil {
		t.Fatalf("list window bindings: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("unexpected bindings length: got=%d want=2", len(got))
	}

	none, err := repo.ListWindowBindingsByConversation(context.Background(), "discord:-:missing:user-1")
	if err != nil {
		t.Fatalf("list missing conversation: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected empty result for missing conversation, got=%d", len(none))
	}
}

func TestSessionMemoryRepositoryGetWindowBindingNotFound(t *testing.T) {
	t.Parallel()

	repo := NewSessionMemoryRepository()
	_, err := repo.GetWindowBinding(context.Background(), "window-missing")
	if !errors.Is(err, session.ErrWindowBindingNotFound) {
		t.Fatalf("expected ErrWindowBindingNotFound, got %v", err)
	}
}
