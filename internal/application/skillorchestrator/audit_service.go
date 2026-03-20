package skillorchestrator

import (
	"sort"
	"strings"
	"sync"
	"time"

	skilldomain "clawx/internal/domain/skill"
)

type AuditRecord struct {
	TraceID        string
	EventType      skilldomain.AuditEventType
	ConversationID string
	Actor          string
	SkillID        string
	Source         string
	Intent         string
	Confidence     float64
	Arguments      map[string]any
	Result         string
	Error          string
	OccurredAt     time.Time
}

type AuditService struct {
	mu      sync.RWMutex
	records []AuditRecord
}

func NewAuditService() *AuditService {
	return &AuditService{}
}

func (s *AuditService) Record(record AuditRecord) {
	if s == nil {
		return
	}
	item := record
	item.TraceID = strings.TrimSpace(item.TraceID)
	item.ConversationID = strings.TrimSpace(item.ConversationID)
	item.Actor = strings.TrimSpace(item.Actor)
	item.SkillID = strings.TrimSpace(item.SkillID)
	item.Source = strings.TrimSpace(item.Source)
	item.Intent = strings.TrimSpace(item.Intent)
	item.Result = strings.TrimSpace(item.Result)
	item.Error = strings.TrimSpace(item.Error)
	if item.OccurredAt.IsZero() {
		item.OccurredAt = time.Now().UTC()
	}
	item.Arguments = cloneArguments(item.Arguments)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, item)
}

func (s *AuditService) ListByTraceID(traceID string) []AuditRecord {
	traceID = strings.TrimSpace(traceID)
	if s == nil || traceID == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AuditRecord, 0)
	for _, item := range s.records {
		if item.TraceID != traceID {
			continue
		}
		out = append(out, cloneAuditRecord(item))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].OccurredAt.Before(out[j].OccurredAt)
	})
	return out
}

func (s *AuditService) ListByConversation(conversationID string, limit int) []AuditRecord {
	conversationID = strings.TrimSpace(conversationID)
	if s == nil || conversationID == "" {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AuditRecord, 0, limit)
	for i := len(s.records) - 1; i >= 0; i-- {
		item := s.records[i]
		if strings.TrimSpace(item.ConversationID) != conversationID {
			continue
		}
		out = append(out, cloneAuditRecord(item))
		if len(out) >= limit {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].OccurredAt.Before(out[j].OccurredAt)
	})
	return out
}

func cloneAuditRecord(item AuditRecord) AuditRecord {
	out := item
	out.Arguments = cloneArguments(item.Arguments)
	return out
}

func cloneArguments(values map[string]any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
