package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"synapsex/internal/domain/session"
)

type Clock func() time.Time

type SessionManager struct {
	repository     session.Repository
	locker         session.Locker
	windowBindings session.WindowBindingRepository
	clock          Clock
}

func NewSessionManager(repository session.Repository, locker session.Locker, clock Clock) *SessionManager {
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}

	var bindings session.WindowBindingRepository
	if candidate, ok := repository.(session.WindowBindingRepository); ok {
		bindings = candidate
	}
	return &SessionManager{
		repository:     repository,
		locker:         locker,
		windowBindings: bindings,
		clock:          clock,
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

func (m *SessionManager) SetWindowBindingRepository(repository session.WindowBindingRepository) {
	m.windowBindings = repository
}

func (m *SessionManager) GetWindowBinding(ctx context.Context, windowID string) (session.WindowBinding, error) {
	if m.windowBindings == nil {
		return session.WindowBinding{}, session.ErrWindowBindingNotFound
	}
	return m.windowBindings.GetWindowBinding(ctx, strings.TrimSpace(windowID))
}

func (m *SessionManager) SetWindowBinding(ctx context.Context, binding session.WindowBinding) error {
	if m.windowBindings == nil {
		return nil
	}
	binding.WindowID = strings.TrimSpace(binding.WindowID)
	binding.CurrentSessionID = strings.TrimSpace(binding.CurrentSessionID)
	binding.ConversationID = strings.TrimSpace(binding.ConversationID)
	if binding.UpdatedAt.IsZero() {
		binding.UpdatedAt = m.clock()
	}
	if binding.LastUsedAt.IsZero() {
		binding.LastUsedAt = binding.UpdatedAt
	}
	return m.windowBindings.SetWindowBinding(ctx, binding)
}

func (m *SessionManager) BindWindowToSession(ctx context.Context, windowID, conversationID, sessionID string) (session.WindowBinding, error) {
	now := m.clock()
	binding := session.WindowBinding{
		WindowID:         strings.TrimSpace(windowID),
		CurrentSessionID: strings.TrimSpace(sessionID),
		ConversationID:   strings.TrimSpace(conversationID),
		LastUsedAt:       now,
		UpdatedAt:        now,
	}
	if err := m.SetWindowBinding(ctx, binding); err != nil {
		return session.WindowBinding{}, err
	}
	return binding, nil
}

func (m *SessionManager) ListWindowBindingsByConversation(ctx context.Context, conversationID string) ([]session.WindowBinding, error) {
	if m.windowBindings == nil {
		return nil, nil
	}
	return m.windowBindings.ListWindowBindingsByConversation(ctx, strings.TrimSpace(conversationID))
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
