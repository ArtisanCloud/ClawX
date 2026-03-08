package persistence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"synapsex/internal/domain/session"
)

func TestSessionFileRepositoryCreateAndReload(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	repo, err := NewSessionFileRepository(stateDir)
	if err != nil {
		t.Fatalf("new file repository: %v", err)
	}

	record := session.Record{
		ID:             "sess-1",
		AgentID:        "main",
		Backend:        "codex",
		ConversationID: "discord:-:chan:user",
		CWD:            "/tmp/work",
		Status:         session.StatusIdle,
		LastUsedAt:     time.Now().UTC(),
	}
	if err := repo.Create(context.Background(), record); err != nil {
		t.Fatalf("create record: %v", err)
	}

	record.BackendSessionID = "thread-1"
	record.LastUsedAt = record.LastUsedAt.Add(2 * time.Second)
	if err := repo.Save(context.Background(), record); err != nil {
		t.Fatalf("save record: %v", err)
	}

	storePath := filepath.Join(stateDir, "agents", "main", "sessions", "sessions.json")
	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("expect sessions.json at %s: %v", storePath, err)
	}

	reloaded, err := NewSessionFileRepository(stateDir)
	if err != nil {
		t.Fatalf("reload repository: %v", err)
	}
	got, err := reloaded.GetByID(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("get reloaded record: %v", err)
	}
	if got.BackendSessionID != "thread-1" {
		t.Fatalf("unexpected backend session id: %q", got.BackendSessionID)
	}
	if got.Status != session.StatusIdle {
		t.Fatalf("unexpected status: %s", got.Status)
	}
}

func TestSessionFileRepositoryClearsRunningLockOnReload(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	repo, err := NewSessionFileRepository(stateDir)
	if err != nil {
		t.Fatalf("new file repository: %v", err)
	}

	record := session.Record{
		ID:             "sess-lock",
		AgentID:        "main",
		Backend:        "codex",
		ConversationID: "discord:-:chan:user",
		CWD:            "/tmp/work",
		Status:         session.StatusIdle,
		LastUsedAt:     time.Now().UTC(),
	}
	if err := repo.Create(context.Background(), record); err != nil {
		t.Fatalf("create record: %v", err)
	}

	running := record
	running.Status = session.StatusRunning
	running.LockToken = "lock-1"
	running.LastUsedAt = running.LastUsedAt.Add(1 * time.Second)
	if err := repo.Save(context.Background(), running); err != nil {
		t.Fatalf("save running record: %v", err)
	}

	current, err := repo.GetByID(context.Background(), running.ID)
	if err != nil {
		t.Fatalf("get current record: %v", err)
	}
	if current.LockToken == "" || current.Status != session.StatusRunning {
		t.Fatalf("expected in-memory running lock state, got status=%s lock=%q", current.Status, current.LockToken)
	}

	reloaded, err := NewSessionFileRepository(stateDir)
	if err != nil {
		t.Fatalf("reload repository: %v", err)
	}
	got, err := reloaded.GetByID(context.Background(), running.ID)
	if err != nil {
		t.Fatalf("get reloaded record: %v", err)
	}
	if got.LockToken != "" {
		t.Fatalf("lock token should be cleared on reload, got %q", got.LockToken)
	}
	if got.Status != session.StatusIdle {
		t.Fatalf("running status should be normalized to idle on reload, got %s", got.Status)
	}
}

func TestSessionFileRepositoryRebuildsFromTranscriptWhenIndexMissing(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	repo, err := NewSessionFileRepository(stateDir)
	if err != nil {
		t.Fatalf("new file repository: %v", err)
	}

	record := session.Record{
		ID:               "sess-rebuild",
		AgentID:          "main",
		Backend:          "codex",
		BackendSessionID: "thread-9",
		ConversationID:   "discord:-:chan:user",
		CWD:              "/tmp/work",
		Status:           session.StatusIdle,
		LastUsedAt:       time.Now().UTC(),
	}
	if err := repo.Create(context.Background(), record); err != nil {
		t.Fatalf("create record: %v", err)
	}
	if err := repo.Save(context.Background(), record); err != nil {
		t.Fatalf("save record: %v", err)
	}

	storePath := filepath.Join(stateDir, "agents", "main", "sessions", "sessions.json")
	if err := os.Remove(storePath); err != nil {
		t.Fatalf("remove sessions index: %v", err)
	}

	reloaded, err := NewSessionFileRepository(stateDir)
	if err != nil {
		t.Fatalf("reload repository: %v", err)
	}
	got, err := reloaded.GetByID(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("get rebuilt record: %v", err)
	}
	if got.BackendSessionID != record.BackendSessionID {
		t.Fatalf("unexpected backend session id after rebuild: %q", got.BackendSessionID)
	}
}
