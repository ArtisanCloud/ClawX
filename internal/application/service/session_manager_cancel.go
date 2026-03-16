package service

import (
	"context"

	"clawx/internal/domain/execution"
	"clawx/internal/domain/session"
)

func (m *SessionManager) CancelExecution(ctx context.Context, backend execution.Backend, sessionID string) (session.Record, error) {
	record, err := m.repository.GetByID(ctx, sessionID)
	if err != nil {
		return session.Record{}, err
	}

	if !record.IsRunning() {
		return record, nil
	}

	if err := backend.Cancel(ctx, sessionID); err != nil {
		return session.Record{}, err
	}
	if err := m.ForceRelease(ctx, sessionID, record.LockToken); err != nil {
		return session.Record{}, err
	}
	return m.repository.GetByID(ctx, sessionID)
}

