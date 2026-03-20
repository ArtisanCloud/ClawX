package skillorchestrator

import (
	"fmt"
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

func FormatRouteAuditEvent(conversationID, actor string, decision RouteDecision) skilldomain.AuditEvent {
	action := decision.Action.Normalize()
	return skilldomain.AuditEvent{
		EventType:      skilldomain.AuditEventRouting,
		ConversationID: strings.TrimSpace(conversationID),
		Actor:          strings.TrimSpace(actor),
		SkillID:        strings.TrimSpace(action.SkillID),
		Source:         strings.TrimSpace(action.Source),
		Intent:         strings.TrimSpace(action.Intent),
		Confidence:     action.Confidence,
		Result:         strings.TrimSpace(decision.Reason),
	}
}

func FormatActionSummary(action skilldomain.SkillAction) string {
	normalized := action.Normalize()
	return fmt.Sprintf("intent=%s skill_id=%s confidence=%.2f source=%s",
		strings.TrimSpace(normalized.Intent),
		strings.TrimSpace(normalized.SkillID),
		normalized.Confidence,
		strings.TrimSpace(normalized.Source),
	)
}
