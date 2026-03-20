package skillorchestrator

import (
	"sort"
	"strings"
	"time"

	skilldomain "clawx/internal/domain/skill"
)

type ContextDigestProjector struct {
	maxEntries int
}

func NewContextDigestProjector(maxEntries int) *ContextDigestProjector {
	if maxEntries <= 0 {
		maxEntries = 50
	}
	return &ContextDigestProjector{maxEntries: maxEntries}
}

func (p *ContextDigestProjector) Project(conversationID string, records []AuditRecord) skilldomain.ContextDigest {
	conversationID = strings.TrimSpace(conversationID)
	values := make([]AuditRecord, 0, len(records))
	for _, item := range records {
		if conversationID != "" && strings.TrimSpace(item.ConversationID) != conversationID {
			continue
		}
		values = append(values, item)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].OccurredAt.Before(values[j].OccurredAt)
	})
	if len(values) > p.maxEntries {
		values = values[len(values)-p.maxEntries:]
	}
	entries := make([]skilldomain.ContextDigestEntry, 0, len(values))
	for _, item := range values {
		entries = append(entries, skilldomain.ContextDigestEntry{
			TraceID:    strings.TrimSpace(item.TraceID),
			SkillID:    strings.TrimSpace(item.SkillID),
			Intent:     strings.TrimSpace(item.Intent),
			Source:     strings.TrimSpace(item.Source),
			Result:     strings.TrimSpace(item.Result),
			RiskLevel:  inferRiskLevelFromArguments(item.Arguments),
			OccurredAt: item.OccurredAt,
		})
	}
	digest := skilldomain.ContextDigest{
		Version:        skilldomain.ContextDigestVersion,
		ConversationID: conversationID,
		Entries:        entries,
		GeneratedAt:    time.Now().UTC(),
	}
	return digest.Normalize()
}

func inferRiskLevelFromArguments(arguments map[string]any) skilldomain.RiskLevel {
	if len(arguments) == 0 {
		return skilldomain.RiskLow
	}
	raw, ok := arguments["risk"]
	if !ok {
		return skilldomain.RiskLow
	}
	text, _ := raw.(string)
	switch strings.ToLower(strings.TrimSpace(text)) {
	case string(skilldomain.RiskHigh):
		return skilldomain.RiskHigh
	case string(skilldomain.RiskMedium):
		return skilldomain.RiskMedium
	default:
		return skilldomain.RiskLow
	}
}
