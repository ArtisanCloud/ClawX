package skill

import (
	"fmt"
	"strings"
	"time"
)

type AuditEventType string

const (
	AuditEventRouting AuditEventType = "skill_routing"
	AuditEventPolicy  AuditEventType = "skill_policy_rejected"
	AuditEventExec    AuditEventType = "skill_exec"
	AuditEventConfirm AuditEventType = "skill_confirm"
	AuditEventReplay  AuditEventType = "skill_audit_replay"
)

type AuditEvent struct {
	EventType      AuditEventType
	ConversationID string
	Actor          string
	SkillID        string
	Source         string
	Intent         string
	Confidence     float64
	Result         string
	Error          string
	OccurredAt     time.Time
}

func (e AuditEvent) FormatForLog() string {
	when := e.OccurredAt
	if when.IsZero() {
		when = time.Now().UTC()
	}
	return fmt.Sprintf("skill_audit event=%s conversation=%s actor=%s skill_id=%s source=%s intent=%s confidence=%.2f result=%s error=%q ts=%s",
		strings.TrimSpace(string(e.EventType)),
		strings.TrimSpace(e.ConversationID),
		strings.TrimSpace(e.Actor),
		strings.TrimSpace(e.SkillID),
		strings.TrimSpace(e.Source),
		strings.TrimSpace(e.Intent),
		e.Confidence,
		strings.TrimSpace(e.Result),
		strings.TrimSpace(e.Error),
		when.Format(time.RFC3339),
	)
}
