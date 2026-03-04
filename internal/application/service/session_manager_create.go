package service

import (
	"context"
	"fmt"
	"strings"

	"synapsex/internal/application/command"
	"synapsex/internal/domain/session"
)

func (m *SessionManager) CreateSession(ctx context.Context, cmd command.SessionCommand) (session.Record, error) {
	cmd, err := cmd.Normalize()
	if err != nil {
		return session.Record{}, err
	}

	record := session.Record{
		ID:             m.nextSessionID(),
		Backend:        cmd.Backend,
		ConversationID: cmd.ConversationID,
		CWD:            cmd.CWD,
		Status:         session.StatusIdle,
	}
	record.Touch(m.clock())

	if err := record.Validate(); err != nil {
		return session.Record{}, err
	}
	if err := m.repository.Create(ctx, record); err != nil {
		return session.Record{}, fmt.Errorf("create session: %w", err)
	}
	return record, nil
}

func (m *SessionManager) nextSessionID() string {
	now := m.clock()
	return fmt.Sprintf("sess-%d", now.UTC().UnixNano())
}

func trimSessionID(value string) string {
	return strings.TrimSpace(value)
}

