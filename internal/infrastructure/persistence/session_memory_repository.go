package persistence

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"synapsex/internal/domain/session"
)

var ErrLockHeldByAnotherProcess = errors.New("lock held by another process")

type SessionMemoryRepository struct {
	mu             sync.RWMutex
	byID           map[string]session.Record
	byConversation map[string][]string
	lockCounter    uint64
}

func NewSessionMemoryRepository() *SessionMemoryRepository {
	return &SessionMemoryRepository{
		byID:           make(map[string]session.Record),
		byConversation: make(map[string][]string),
	}
}

func (r *SessionMemoryRepository) Create(_ context.Context, record session.Record) error {
	if err := record.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[record.ID]; exists {
		return fmt.Errorf("create session %s: %w", record.ID, session.ErrInvalidSession)
	}

	record = cloneRecord(record)
	r.byID[record.ID] = record
	r.byConversation[record.ConversationID] = append(r.byConversation[record.ConversationID], record.ID)
	return nil
}

func (r *SessionMemoryRepository) Save(_ context.Context, record session.Record) error {
	if err := record.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[record.ID]; !exists {
		return session.ErrSessionNotFound
	}
	r.byID[record.ID] = cloneRecord(record)
	return nil
}

func (r *SessionMemoryRepository) GetByID(_ context.Context, sessionID string) (session.Record, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	record, exists := r.byID[sessionID]
	if !exists {
		return session.Record{}, session.ErrSessionNotFound
	}
	return cloneRecord(record), nil
}

func (r *SessionMemoryRepository) GetLatestByConversation(_ context.Context, conversationID string) (session.Record, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byConversation[conversationID]
	if len(ids) == 0 {
		return session.Record{}, session.ErrSessionNotFound
	}

	var latest session.Record
	var found bool
	for _, id := range ids {
		record, exists := r.byID[id]
		if !exists {
			continue
		}
		if !found || record.LastUsedAt.After(latest.LastUsedAt) {
			latest = record
			found = true
		}
	}
	if !found {
		return session.Record{}, session.ErrSessionNotFound
	}
	return cloneRecord(latest), nil
}

func (r *SessionMemoryRepository) ListByConversation(_ context.Context, conversationID string) ([]session.Record, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byConversation[conversationID]
	if len(ids) == 0 {
		return nil, nil
	}

	records := make([]session.Record, 0, len(ids))
	for _, id := range ids {
		record, exists := r.byID[id]
		if !exists {
			continue
		}
		records = append(records, cloneRecord(record))
	}
	return records, nil
}

func (r *SessionMemoryRepository) Acquire(_ context.Context, sessionID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.byID[sessionID]
	if !exists {
		return "", session.ErrSessionNotFound
	}
	if record.LockToken != "" {
		return "", ErrLockHeldByAnotherProcess
	}

	r.lockCounter++
	lockToken := fmt.Sprintf("%s-%d-%d", sessionID, time.Now().UTC().UnixNano(), r.lockCounter)
	record.LockToken = lockToken
	r.byID[sessionID] = record
	return lockToken, nil
}

func (r *SessionMemoryRepository) Release(_ context.Context, sessionID, lockToken string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.byID[sessionID]
	if !exists {
		return session.ErrSessionNotFound
	}
	if record.LockToken == "" {
		return nil
	}
	if record.LockToken != lockToken {
		return session.ErrInvalidLock
	}
	record.LockToken = ""
	r.byID[sessionID] = record
	return nil
}

func cloneRecord(record session.Record) session.Record {
	return record
}

