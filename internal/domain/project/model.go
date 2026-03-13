package project

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidProject   = errors.New("invalid project")
	ErrProjectNotFound  = errors.New("project not found")
	ErrBindingNotFound  = errors.New("project binding not found")
	ErrProposalNotFound = errors.New("project proposal not found")
	ErrProposalExpired  = errors.New("project proposal expired")
	ErrProposalInvalid  = errors.New("project proposal is invalid")
	ErrProjectInUse     = errors.New("project has active bindings")
	ErrProjectBusy      = errors.New("project has active sessions")
)

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusBroken   Status = "broken"
)

type Source string

const (
	SourceManual        Source = "manual"
	SourceAutoConfirmed Source = "auto_confirmed"
	SourceFallbackSeed  Source = "fallback_seed"
)

type ProposalStatus string

const (
	ProposalPending  ProposalStatus = "pending"
	ProposalAccepted ProposalStatus = "accepted"
	ProposalRejected ProposalStatus = "rejected"
	ProposalExpired  ProposalStatus = "expired"
)

type Record struct {
	ID             string
	Name           string
	WorkspacePath  string
	Status         Status
	DefaultAgentID string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (r Record) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return ErrInvalidProject
	}
	if strings.TrimSpace(r.WorkspacePath) == "" {
		return ErrInvalidProject
	}
	switch r.Status {
	case "", StatusActive, StatusInactive, StatusBroken:
	default:
		return ErrInvalidProject
	}
	return nil
}

type Registry struct {
	Version          int
	DefaultProjectID string
	Projects         map[string]Record
	UpdatedAt        time.Time
}

func (r Registry) Validate() error {
	if strings.TrimSpace(r.DefaultProjectID) == "" {
		return ErrInvalidProject
	}
	if len(r.Projects) == 0 {
		return ErrInvalidProject
	}
	for id, item := range r.Projects {
		if strings.TrimSpace(id) == "" {
			return ErrInvalidProject
		}
		if item.ID != id {
			return ErrInvalidProject
		}
		if err := item.Validate(); err != nil {
			return err
		}
	}
	if _, ok := r.Projects[r.DefaultProjectID]; !ok {
		return ErrInvalidProject
	}
	return nil
}

type RouteBinding struct {
	RouteKey  string
	ProjectID string
	UpdatedAt time.Time
	UpdatedBy string
	Source    Source
}

func (b RouteBinding) Validate() error {
	if strings.TrimSpace(b.RouteKey) == "" {
		return ErrInvalidProject
	}
	if strings.TrimSpace(b.ProjectID) == "" {
		return ErrInvalidProject
	}
	switch b.Source {
	case "", SourceManual, SourceAutoConfirmed, SourceFallbackSeed:
	default:
		return ErrInvalidProject
	}
	return nil
}

type Proposal struct {
	ID            string
	RouteKey      string
	FromProjectID string
	ToProjectID   string
	Confidence    float64
	Reason        string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	Status        ProposalStatus
}

type BindingIssue struct {
	RouteKey  string
	ProjectID string
	Reason    string
}

type AuditReport struct {
	TotalProjects    int
	ActiveProjects   int
	BrokenProjects   int
	TotalBindings    int
	BrokenBindings   int
	BindingIssues    []BindingIssue
	CheckedAt        time.Time
	ChangedProjectID []string
}

func (p Proposal) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return ErrInvalidProject
	}
	if strings.TrimSpace(p.RouteKey) == "" {
		return ErrInvalidProject
	}
	if strings.TrimSpace(p.ToProjectID) == "" {
		return ErrInvalidProject
	}
	switch p.Status {
	case "", ProposalPending, ProposalAccepted, ProposalRejected, ProposalExpired:
	default:
		return ErrInvalidProject
	}
	return nil
}
