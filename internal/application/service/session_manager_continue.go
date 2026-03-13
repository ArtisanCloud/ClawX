package service

import (
	"context"
	"errors"
	"strings"

	"clawx/internal/application/command"
	"clawx/internal/domain/session"
)

func (m *SessionManager) ContinueSession(ctx context.Context, cmd command.SessionCommand) (session.Record, error) {
	cmd, err := cmd.Normalize()
	if err != nil {
		return session.Record{}, err
	}
	cmd.ProjectID = normalizeSessionProjectID(cmd.ProjectID)
	cmd.WindowID = buildSessionScopeWindowID(cmd.WindowID, cmd.ProjectID)

	if record, err := m.continueByWindowBinding(ctx, cmd); err == nil {
		return record, nil
	} else if !errors.Is(err, session.ErrWindowBindingNotFound) && !errors.Is(err, session.ErrSessionNotFound) {
		return session.Record{}, err
	}

	record, err := m.findLatestSessionForProject(ctx, cmd.ConversationID, cmd.ProjectID)
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
	if !sessionBelongsToProject(record.ID, cmd.ProjectID) {
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

func (m *SessionManager) findLatestSessionForProject(ctx context.Context, conversationID, projectID string) (session.Record, error) {
	projectID = normalizeSessionProjectID(projectID)
	records, err := m.repository.ListByConversation(ctx, strings.TrimSpace(conversationID))
	if err != nil {
		return session.Record{}, err
	}
	if len(records) == 0 {
		return session.Record{}, session.ErrSessionNotFound
	}

	var latest session.Record
	found := false
	for _, record := range records {
		if !sessionBelongsToProject(record.ID, projectID) {
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
	return latest, nil
}
