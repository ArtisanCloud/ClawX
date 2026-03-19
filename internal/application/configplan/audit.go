package configplan

import (
	"fmt"
	"strings"
	"time"
)

type AuditEventType string

const (
	EventPlanCreated       AuditEventType = "plan_created"
	EventPlanPatched                      = "plan_patched"
	EventPlanShown                        = "plan_shown"
	EventPlanCanceled                     = "plan_canceled"
	EventPlanApplied                      = "plan_applied"
	EventPlanApplyRejected                = "plan_apply_rejected"
)

type AuditEvent struct {
	EventType            AuditEventType
	ConversationScope    string
	PlanID               string
	PlanVersion          int64
	SummaryVersion       int64
	SummaryRebuildReason string
	Actor                string
	Source               Source
	DiffSummary          string
	Timestamp            time.Time
}

func (e AuditEvent) FormatForLog() string {
	ts := e.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	return fmt.Sprintf("config_audit event=%s conversation=%s plan_id=%s actor=%s source=%s version=%d summary_version=%d summary_rebuild_reason=%q diff=%q ts=%s",
		e.EventType,
		strings.TrimSpace(e.ConversationScope),
		strings.TrimSpace(e.PlanID),
		strings.TrimSpace(e.Actor),
		strings.TrimSpace(string(e.Source)),
		e.PlanVersion,
		e.SummaryVersion,
		strings.TrimSpace(e.SummaryRebuildReason),
		strings.TrimSpace(e.DiffSummary),
		ts.Format(time.RFC3339),
	)
}
