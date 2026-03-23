package skill

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidControlPlan = errors.New("invalid control plan")

type ControlPlan struct {
	Type      string
	Intent    string
	Target    map[string]any
	Mode      string
	Risk      RiskLevel
	Reason    string
	CreatedAt time.Time
}

func (p ControlPlan) Normalize() ControlPlan {
	out := p
	out.Type = strings.ToLower(strings.TrimSpace(out.Type))
	out.Intent = strings.ToLower(strings.TrimSpace(out.Intent))
	out.Mode = strings.ToLower(strings.TrimSpace(out.Mode))
	out.Reason = strings.TrimSpace(out.Reason)
	if out.Target == nil {
		out.Target = map[string]any{}
	}
	if out.CreatedAt.IsZero() {
		out.CreatedAt = time.Now().UTC()
	}
	if strings.TrimSpace(string(out.Risk)) == "" {
		out.Risk = RiskLow
	}
	return out
}

func (p ControlPlan) Validate() error {
	normalized := p.Normalize()
	if normalized.Type != "control_plan" {
		return ErrInvalidControlPlan
	}
	if normalized.Intent == "" {
		return ErrInvalidControlPlan
	}
	if normalized.Mode != "execute" && normalized.Mode != "suggest" {
		return ErrInvalidControlPlan
	}
	if err := normalized.Risk.Validate(); err != nil {
		return ErrInvalidControlPlan
	}
	if len(normalized.Target) == 0 {
		return ErrInvalidControlPlan
	}
	return nil
}

func (p ControlPlan) AgentID() string {
	normalized := p.Normalize()
	if normalized.Intent != "agent.use" {
		return ""
	}
	raw, ok := normalized.Target["agent_id"]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
