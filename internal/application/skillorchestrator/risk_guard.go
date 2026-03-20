package skillorchestrator

import (
	"fmt"
	"strings"
	"sync"
	"time"

	skilldomain "clawx/internal/domain/skill"
)

type ConfirmationRequiredError struct {
	ConfirmationID string
}

func (e *ConfirmationRequiredError) Error() string {
	return fmt.Sprintf("confirmation required: %s", strings.TrimSpace(e.ConfirmationID))
}

type ConfirmationRejectedError struct {
	ConfirmationID string
}

func (e *ConfirmationRejectedError) Error() string {
	return fmt.Sprintf("confirmation rejected: %s", strings.TrimSpace(e.ConfirmationID))
}

type confirmationEntry struct {
	ConversationID string
	Actor          string
	SkillID        string
	Signature      string
	ExpiresAt      time.Time
}

type RiskAuthorizeRequest struct {
	ConversationID string
	Actor          string
	SkillID        string
	Signature      string
	RiskLevel      skilldomain.RiskLevel
	ConfirmationID string
	Decision       string
}

type RiskGuard struct {
	mu            sync.Mutex
	pending       map[string]confirmationEntry
	ttl           time.Duration
	confirmMedium bool
	idPrefix      string
}

func NewRiskGuard(ttl time.Duration) *RiskGuard {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &RiskGuard{
		pending:  make(map[string]confirmationEntry),
		ttl:      ttl,
		idPrefix: "cfm",
	}
}

func (g *RiskGuard) SetConfirmMedium(enabled bool) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.confirmMedium = enabled
}

func (g *RiskGuard) Authorize(request RiskAuthorizeRequest) error {
	if g == nil {
		return nil
	}
	risk := request.RiskLevel
	needsConfirm := risk == skilldomain.RiskHigh || (risk == skilldomain.RiskMedium && g.confirmMedium)
	if !needsConfirm {
		return nil
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	g.cleanupExpiredLocked()

	confirmationID := strings.TrimSpace(request.ConfirmationID)
	if confirmationID == "" {
		id := g.newConfirmationIDLocked()
		g.pending[id] = confirmationEntry{
			ConversationID: strings.TrimSpace(request.ConversationID),
			Actor:          strings.TrimSpace(request.Actor),
			SkillID:        strings.TrimSpace(request.SkillID),
			Signature:      strings.TrimSpace(request.Signature),
			ExpiresAt:      time.Now().UTC().Add(g.ttl),
		}
		return &ConfirmationRequiredError{ConfirmationID: id}
	}

	entry, ok := g.pending[confirmationID]
	if !ok {
		return fmt.Errorf("confirmation %q not found or expired", confirmationID)
	}
	if entry.Signature != strings.TrimSpace(request.Signature) {
		return fmt.Errorf("confirmation %q does not match current action", confirmationID)
	}
	if entry.ConversationID != strings.TrimSpace(request.ConversationID) {
		return fmt.Errorf("confirmation %q does not match current conversation", confirmationID)
	}
	if entry.Actor != strings.TrimSpace(request.Actor) {
		return fmt.Errorf("confirmation %q does not match current actor", confirmationID)
	}

	delete(g.pending, confirmationID)
	decision := strings.ToLower(strings.TrimSpace(request.Decision))
	switch decision {
	case "approve", "confirm", "yes":
		return nil
	case "reject", "no":
		return &ConfirmationRejectedError{ConfirmationID: confirmationID}
	default:
		return fmt.Errorf("confirmation %q decision is required", confirmationID)
	}
}

func (g *RiskGuard) newConfirmationIDLocked() string {
	return fmt.Sprintf("%s-%d", g.idPrefix, time.Now().UTC().UnixNano())
}

func (g *RiskGuard) cleanupExpiredLocked() {
	now := time.Now().UTC()
	for key, item := range g.pending {
		if !item.ExpiresAt.IsZero() && item.ExpiresAt.Before(now) {
			delete(g.pending, key)
		}
	}
}
