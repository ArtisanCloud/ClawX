package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	memorydomain "clawx/internal/domain/memory"
)

type MemoryJournalFileStore struct {
	mu            sync.Mutex
	workspaceRoot string
}

func NewMemoryJournalFileStore(workspaceRoot string) (*MemoryJournalFileStore, error) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return nil, fmt.Errorf("workspace root is required")
	}
	return &MemoryJournalFileStore{workspaceRoot: filepath.Clean(workspaceRoot)}, nil
}

func (s *MemoryJournalFileStore) Append(_ context.Context, scopeKey memorydomain.MemoryScopeKey, entry memorydomain.JournalEntry) error {
	if err := scopeKey.Validate(); err != nil {
		return err
	}
	entry.Scope = normalizeJournalScope(entry.Scope)
	if strings.TrimSpace(entry.EntryID) == "" {
		entry.EntryID = fmt.Sprintf("entry-%d", time.Now().UTC().UnixNano())
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.journalPath(scopeKey, entry.Scope, entry.CreatedAt)
	if err != nil {
		return err
	}
	entries, err := readJournalEntries(path)
	if err != nil {
		return err
	}
	entries = append(entries, entry)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.Before(entries[j].CreatedAt)
	})
	payload, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal memory journal entries: %w", err)
	}
	payload = append(payload, '\n')
	return writeMemoryAtomic(path, payload, ".memory-journal-*.tmp")
}

func (s *MemoryJournalFileStore) ListByScope(_ context.Context, scopeKey memorydomain.MemoryScopeKey, scope string, day time.Time) ([]memorydomain.JournalEntry, error) {
	if err := scopeKey.Validate(); err != nil {
		return nil, err
	}
	scope = normalizeJournalScope(scope)
	if day.IsZero() {
		day = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.journalPath(scopeKey, scope, day)
	if err != nil {
		return nil, err
	}
	entries, err := readJournalEntries(path)
	if err != nil {
		if err == memorydomain.ErrMemoryNotFound {
			return nil, nil
		}
		return nil, err
	}
	result := make([]memorydomain.JournalEntry, len(entries))
	copy(result, entries)
	return result, nil
}

func (s *MemoryJournalFileStore) journalPath(scopeKey memorydomain.MemoryScopeKey, scope string, day time.Time) (string, error) {
	projectID := normalizeMemoryPathSegment(scopeKey.ProjectID)
	agentID := normalizeMemoryPathSegment(scopeKey.AgentID)
	if projectID == "" || agentID == "" {
		return "", memorydomain.ErrInvalidMemoryScope
	}

	base := filepath.Join(s.workspaceRoot, projectID, ".memory", "journal")
	switch scope {
	case "project-shared":
		base = filepath.Join(base, "shared")
	default:
		base = filepath.Join(base, "agents", agentID)
	}

	filename := day.UTC().Format("2006-01-02") + ".json"
	path := filepath.Join(base, filename)
	if err := ensurePathWithinRoot(filepath.Join(s.workspaceRoot, projectID), path); err != nil {
		return "", err
	}
	return path, nil
}

func readJournalEntries(path string) ([]memorydomain.JournalEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, memorydomain.ErrMemoryNotFound
		}
		return nil, fmt.Errorf("open memory journal file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var entries []memorydomain.JournalEntry
	if err := decoder.Decode(&entries); err != nil {
		return nil, fmt.Errorf("parse memory journal file: %w", err)
	}
	if decoder.More() {
		return nil, fmt.Errorf("parse memory journal file: trailing data")
	}
	if entries == nil {
		entries = make([]memorydomain.JournalEntry, 0)
	}
	return entries, nil
}

func normalizeJournalScope(raw string) string {
	scope := strings.ToLower(strings.TrimSpace(raw))
	if scope == "project_shared" || scope == "project-shared" {
		return "project-shared"
	}
	return "agent-private"
}
