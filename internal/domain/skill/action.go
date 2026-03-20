package skill

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidSkillAction = errors.New("invalid skill action")

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

func (r RiskLevel) Validate() error {
	switch RiskLevel(strings.TrimSpace(string(r))) {
	case RiskLow, RiskMedium, RiskHigh:
		return nil
	default:
		return ErrInvalidSkillAction
	}
}

type SkillAction struct {
	Intent               string
	SkillID              string
	Arguments            map[string]any
	Confidence           float64
	RiskLevel            RiskLevel
	RequiresConfirmation bool
	Source               string
	CreatedAt            time.Time
}

func (a SkillAction) Normalize() SkillAction {
	out := a
	out.Intent = strings.TrimSpace(out.Intent)
	out.SkillID = strings.TrimSpace(out.SkillID)
	out.Source = strings.TrimSpace(out.Source)
	if out.Arguments == nil {
		out.Arguments = map[string]any{}
	}
	if out.CreatedAt.IsZero() {
		out.CreatedAt = time.Now().UTC()
	}
	if out.Confidence < 0 {
		out.Confidence = 0
	}
	if out.Confidence > 1 {
		out.Confidence = 1
	}
	if strings.TrimSpace(string(out.RiskLevel)) == "" {
		out.RiskLevel = RiskLow
	}
	return out
}

func (a SkillAction) Validate() error {
	normalized := a.Normalize()
	if normalized.Intent == "" || normalized.SkillID == "" {
		return ErrInvalidSkillAction
	}
	if err := normalized.RiskLevel.Validate(); err != nil {
		return err
	}
	if normalized.RiskLevel == RiskHigh && !normalized.RequiresConfirmation {
		return ErrInvalidSkillAction
	}
	return nil
}
