package service

import (
	"context"

	"synapsex/internal/application/command"
	"synapsex/internal/domain/session"
)

func (m *SessionManager) ContinueSession(ctx context.Context, cmd command.SessionCommand) (session.Record, error) {
	cmd, err := cmd.Normalize()
	if err != nil {
		return session.Record{}, err
	}

	record, err := m.repository.GetLatestByConversation(ctx, cmd.ConversationID)
	if err != nil {
		return session.Record{}, err
	}

	record.WindowID = cmd.WindowID
	record.Touch(m.clock())
	if err := m.repository.Save(ctx, record); err != nil {
		return session.Record{}, err
	}
	return record, nil
}
