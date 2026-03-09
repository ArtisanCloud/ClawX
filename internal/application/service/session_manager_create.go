package service

import (
	"context"
	"fmt"
	"strings"

	"synapsex/internal/application/command"
	"synapsex/internal/domain/session"
)

func (m *SessionManager) CreateSession(ctx context.Context, cmd command.SessionCommand) (session.Record, error) {
	cmd.ConversationID = strings.TrimSpace(cmd.ConversationID)
	cmd.WindowID = strings.TrimSpace(cmd.WindowID)
	cmd.Backend = strings.TrimSpace(cmd.Backend)
	cmd.CWD = strings.TrimSpace(cmd.CWD)
	if cmd.ConversationID == "" {
		return session.Record{}, command.ErrInvalidSessionCommand
	}
	if cmd.WindowID == "" {
		cmd.WindowID = "compat:" + cmd.ConversationID
	}
	if cmd.Backend == "" {
		cmd.Backend = "primary"
	}
	if cmd.CWD == "" {
		cmd.CWD = "."
	}

	record := session.Record{
		ID:             m.nextSessionID(),
		WindowID:       cmd.WindowID,
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
	if _, err := m.BindWindowToSession(ctx, cmd.WindowID, cmd.ConversationID, record.ID); err != nil {
		return session.Record{}, err
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
