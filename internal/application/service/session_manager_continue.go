package service

import (
	"context"
	"errors"

	"synapsex/internal/application/command"
	"synapsex/internal/domain/session"
)

func (m *SessionManager) ContinueSession(ctx context.Context, cmd command.SessionCommand) (session.Record, error) {
	cmd, err := cmd.Normalize()
	if err != nil {
		return session.Record{}, err
	}

	if record, err := m.continueByWindowBinding(ctx, cmd); err == nil {
		return record, nil
	} else if !errors.Is(err, session.ErrWindowBindingNotFound) && !errors.Is(err, session.ErrSessionNotFound) {
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
	if _, err := m.BindWindowToSession(ctx, cmd.WindowID, cmd.ConversationID, record.ID); err != nil {
		return session.Record{}, err
	}
	return record, nil
}

func (m *SessionManager) continueByWindowBinding(ctx context.Context, cmd command.SessionCommand) (session.Record, error) {
	binding, err := m.GetWindowBinding(ctx, cmd.WindowID)
	if err != nil {
		return session.Record{}, err
	}
	if binding.ConversationID != cmd.ConversationID {
		return session.Record{}, session.ErrSessionNotFound
	}

	record, err := m.repository.GetByID(ctx, binding.CurrentSessionID)
	if err != nil {
		return session.Record{}, err
	}
	if record.ConversationID != cmd.ConversationID {
		return session.Record{}, session.ErrSessionNotFound
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
