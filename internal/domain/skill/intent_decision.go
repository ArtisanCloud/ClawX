package skill

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidIntentDecision = errors.New("invalid intent decision")

type IntentKind string

const (
	IntentControl IntentKind = "control"
	IntentSkill   IntentKind = "skill"
	IntentTask    IntentKind = "task"
)

type IntentDecision struct {
	Kind       IntentKind
	SkillName  string
	Reason     string
	Confidence float64
	DecidedAt  time.Time
}

func (d IntentDecision) Normalize() (IntentDecision, error) {
	out := d
	out.Kind = IntentKind(strings.TrimSpace(string(out.Kind)))
	out.SkillName = NormalizeName(out.SkillName)
	out.Reason = strings.TrimSpace(out.Reason)
	if out.DecidedAt.IsZero() {
		out.DecidedAt = time.Now().UTC()
	}
	if out.Kind == "" || out.Reason == "" {
		return IntentDecision{}, ErrInvalidIntentDecision
	}
	if out.Kind == IntentSkill && out.SkillName == "" {
		return IntentDecision{}, ErrInvalidIntentDecision
	}
	if out.Kind != IntentSkill {
		out.SkillName = ""
	}
	if out.Confidence < 0 {
		out.Confidence = 0
	}
	if out.Confidence > 1 {
		out.Confidence = 1
	}
	return out, nil
}
