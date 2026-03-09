package service

import (
	"context"
	"errors"
	"sort"
	"strings"

	"synapsex/internal/domain/session"
)

type SessionSummary struct {
	ID               string
	Status           session.Status
	Backend          string
	BackendSessionID string
}

func (m *SessionManager) ListSessionSummaries(ctx context.Context, conversationID string) ([]SessionSummary, error) {
	records, err := m.repository.ListByConversation(ctx, strings.TrimSpace(conversationID))
	if err != nil {
		return nil, err
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].LastUsedAt.After(records[j].LastUsedAt)
	})

	summaries := make([]SessionSummary, 0, len(records))
	for _, record := range records {
		summaries = append(summaries, SessionSummary{
			ID:               record.ID,
			Status:           record.Status,
			Backend:          record.Backend,
			BackendSessionID: record.BackendSessionID,
		})
	}
	return summaries, nil
}

func (m *SessionManager) ListSessionSummariesByWindow(ctx context.Context, conversationID, windowID string) ([]SessionSummary, *SessionSummary, error) {
	conversationID = strings.TrimSpace(conversationID)
	windowID = strings.TrimSpace(windowID)
	summaries, err := m.ListSessionSummaries(ctx, conversationID)
	if err != nil {
		return nil, nil, err
	}

	currentRecord, err := m.GetCurrentSessionByWindow(ctx, conversationID, windowID)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) || errors.Is(err, session.ErrWindowBindingNotFound) {
			return summaries, nil, nil
		}
		return nil, nil, err
	}

	for _, summary := range summaries {
		if summary.ID != currentRecord.ID {
			continue
		}
		current := summary
		return summaries, &current, nil
	}
	return summaries, nil, nil
}

func (m *SessionManager) GetCurrentSessionByWindow(ctx context.Context, conversationID, windowID string) (session.Record, error) {
	conversationID = strings.TrimSpace(conversationID)
	windowID = strings.TrimSpace(windowID)
	if conversationID == "" || windowID == "" {
		return session.Record{}, session.ErrSessionNotFound
	}

	binding, err := m.GetWindowBinding(ctx, windowID)
	if err != nil {
		return session.Record{}, err
	}
	if strings.TrimSpace(binding.ConversationID) != conversationID {
		return session.Record{}, session.ErrSessionNotFound
	}

	record, err := m.repository.GetByID(ctx, binding.CurrentSessionID)
	if err != nil {
		return session.Record{}, err
	}
	if strings.TrimSpace(record.ConversationID) != conversationID {
		return session.Record{}, session.ErrSessionNotFound
	}
	return record, nil
}

func (m *SessionManager) SwitchSession(ctx context.Context, conversationID, windowID, targetSessionID string) (session.Record, error) {
	conversationID = strings.TrimSpace(conversationID)
	windowID = strings.TrimSpace(windowID)
	targetSessionID = trimSessionID(targetSessionID)
	if conversationID == "" || windowID == "" || targetSessionID == "" {
		return session.Record{}, session.ErrSessionNotFound
	}

	record, err := m.repository.GetByID(ctx, targetSessionID)
	if err != nil {
		return session.Record{}, err
	}
	if strings.TrimSpace(record.ConversationID) != conversationID {
		return session.Record{}, ErrConversationMismatch
	}

	record.WindowID = windowID
	record.Touch(m.clock())
	if err := m.repository.Save(ctx, record); err != nil {
		return session.Record{}, err
	}
	if _, err := m.BindWindowToSession(ctx, windowID, conversationID, record.ID); err != nil {
		return session.Record{}, err
	}
	return record, nil
}
