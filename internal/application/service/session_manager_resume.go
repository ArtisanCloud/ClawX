package service

import (
	"context"
	"errors"

	"clawx/internal/application/command"
	"clawx/internal/domain/session"
)

var ErrConversationMismatch = errors.New("session does not belong to the active conversation")

func (m *SessionManager) ResumeSession(ctx context.Context, cmd command.SessionCommand) (session.Record, error) {
	cmd, err := cmd.Normalize()
	if err != nil {
		return session.Record{}, err
	}

	record, err := m.repository.GetByID(ctx, trimSessionID(cmd.ResumeSessionID))
	if err != nil {
		return session.Record{}, err
	}
	if record.ConversationID != cmd.ConversationID {
		return session.Record{}, ErrConversationMismatch
	}

	record.WindowID = cmd.WindowID
	record.Touch(m.clock())
	if err := m.repository.Save(ctx, record); err != nil {
		return session.Record{}, err
	}
	if _, err := m.BindWindowToSession(ctx, cmd.WindowID, cmd.ConversationID, record.ID); err != nil {
		return session.Record{}, err
	}
	return record, nil
}
