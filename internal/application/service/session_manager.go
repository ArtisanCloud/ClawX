package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"synapsex/internal/domain/session"
)

type Clock func() time.Time

type SessionManager struct {
	repository session.Repository
	locker     session.Locker
	clock      Clock
}

func NewSessionManager(repository session.Repository, locker session.Locker, clock Clock) *SessionManager {
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return &SessionManager{
		repository: repository,
		locker:     locker,
		clock:      clock,
	}
}

func (m *SessionManager) GetByID(ctx context.Context, sessionID string) (session.Record, error) {
	return m.repository.GetByID(ctx, sessionID)
}

func (m *SessionManager) GetLatestByConversation(ctx context.Context, conversationID string) (session.Record, error) {
	return m.repository.GetLatestByConversation(ctx, conversationID)
}

func (m *SessionManager) ListByConversation(ctx context.Context, conversationID string) ([]session.Record, error) {
	return m.repository.ListByConversation(ctx, conversationID)
}

func (m *SessionManager) Save(ctx context.Context, record session.Record) error {
	record.Touch(m.clock())
	return m.repository.Save(ctx, record)
}

func (m *SessionManager) AcquireExecution(ctx context.Context, sessionID string) (session.Record, string, error) {
	record, err := m.repository.GetByID(ctx, sessionID)
	if err != nil {
		return session.Record{}, "", err
	}
	if record.IsRunning() {
		return session.Record{}, "", session.ErrSessionBusy
	}

	lockToken, err := m.locker.Acquire(ctx, sessionID)
	if err != nil {
		return session.Record{}, "", err
	}

	record, err = m.repository.GetByID(ctx, sessionID)
	if err != nil {
		_ = m.locker.Release(ctx, sessionID, lockToken)
		return session.Record{}, "", err
	}

	if err := record.StartExecution(lockToken, m.clock()); err != nil {
		_ = m.locker.Release(ctx, sessionID, lockToken)
		return session.Record{}, "", err
	}
	if err := m.repository.Save(ctx, record); err != nil {
		_ = m.locker.Release(ctx, sessionID, lockToken)
		return session.Record{}, "", err
	}
	return record, lockToken, nil
}

func (m *SessionManager) FinishExecution(ctx context.Context, sessionID, lockToken string, nextStatus session.Status) (session.Record, error) {
	record, err := m.repository.GetByID(ctx, sessionID)
	if err != nil {
		return session.Record{}, err
	}
	if record.LockToken != lockToken {
		return session.Record{}, session.ErrInvalidLock
	}
	if err := record.FinishExecution(nextStatus, m.clock()); err != nil {
		return session.Record{}, err
	}
	if err := m.repository.Save(ctx, record); err != nil {
		return session.Record{}, err
	}
	if err := m.locker.Release(ctx, sessionID, lockToken); err != nil {
		return session.Record{}, err
	}
	return record, nil
}

func (m *SessionManager) ForceRelease(ctx context.Context, sessionID, lockToken string) error {
	record, err := m.repository.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if lockToken == "" {
		lockToken = record.LockToken
	}
	if record.LockToken == "" {
		return nil
	}
	if record.LockToken != lockToken {
		return session.ErrInvalidLock
	}
	record.Status = session.StatusIdle
	record.LockToken = ""
	record.Touch(m.clock())
	if err := m.repository.Save(ctx, record); err != nil {
		return err
	}
	return m.locker.Release(ctx, sessionID, lockToken)
}

func (m *SessionManager) MarkError(ctx context.Context, sessionID, lockToken, failureReason string) error {
	record, err := m.repository.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if record.LockToken != lockToken {
		return session.ErrInvalidLock
	}
	if err := record.FinishExecution(session.StatusError, m.clock()); err != nil {
		if !errors.Is(err, session.ErrSessionNotBusy) {
			return fmt.Errorf("finish session with error (%s): %w", failureReason, err)
		}
	}
	if err := m.repository.Save(ctx, record); err != nil {
		return err
	}
	return m.locker.Release(ctx, sessionID, lockToken)
}

