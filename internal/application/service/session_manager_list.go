package service

import (
	"context"
	"sort"

	"synapsex/internal/domain/session"
)

type SessionSummary struct {
	ID               string
	Status           session.Status
	Backend          string
	BackendSessionID string
}

func (m *SessionManager) ListSessionSummaries(ctx context.Context, conversationID string) ([]SessionSummary, error) {
	records, err := m.repository.ListByConversation(ctx, conversationID)
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

