package skill

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidSkillBinding = errors.New("invalid skill binding")

type BindingScope string

const (
	ScopeGlobal     BindingScope = "global"
	ScopeProject    BindingScope = "project"
	ScopeAgentLocal BindingScope = "agent-local"
)

type SkillBinding struct {
	SkillID   string
	Version   string
	Scope     BindingScope
	ProjectID string
	AgentID   string
	UpdatedAt time.Time
}

func (b SkillBinding) Normalize() SkillBinding {
	out := b
	out.SkillID = strings.TrimSpace(out.SkillID)
	out.Version = strings.TrimSpace(out.Version)
	out.ProjectID = strings.TrimSpace(out.ProjectID)
	out.AgentID = strings.TrimSpace(out.AgentID)
	out.Scope = BindingScope(strings.TrimSpace(string(out.Scope)))
	if out.UpdatedAt.IsZero() {
		out.UpdatedAt = time.Now().UTC()
	}
	return out
}

func (b SkillBinding) Validate() error {
	normalized := b.Normalize()
	if normalized.SkillID == "" || normalized.Version == "" {
		return ErrInvalidSkillBinding
	}
	switch normalized.Scope {
	case ScopeGlobal:
	case ScopeProject:
		if normalized.ProjectID == "" {
			return ErrInvalidSkillBinding
		}
	case ScopeAgentLocal:
		if normalized.AgentID == "" {
			return ErrInvalidSkillBinding
		}
	default:
		return ErrInvalidSkillBinding
	}
	return nil
}

func (b SkillBinding) Key() string {
	normalized := b.Normalize()
	return fmt.Sprintf("%s:%s:%s:%s:%s",
		normalized.Scope,
		normalized.ProjectID,
		normalized.AgentID,
		normalized.SkillID,
		normalized.Version,
	)
}
