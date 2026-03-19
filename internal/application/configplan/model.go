package configplan

import (
	"errors"
	"strings"
	"time"
)

type Kind string

const (
	KindUpsertAgent     Kind = "upsert-agent"
	KindSetDefaultAgent      = "set-default-agent"
	KindDeleteAgent          = "delete-agent"
	KindRenameAgent          = "rename-agent"
)

type Source string

const (
	SourceSlash Source = "slash"
	SourceNL           = "nl"
)

var (
	ErrInvalidPlanKind = errors.New("invalid config plan kind")
	ErrInvalidPlan     = errors.New("invalid config plan")
)

type UpsertAgentOptions struct {
	ID             string
	ProfileID      string
	Workspace      string
	TimeoutSeconds int
	SetAsDefault   bool
}

type DeleteAgentOptions struct {
	ID              string
	DeleteWorkspace bool
}

type RenameAgentOptions struct {
	FromID           string
	ToID             string
	TargetWorkspace  string
	MigrateWorkspace bool
}

type SummaryField string

const (
	SummaryFieldAgentID   SummaryField = "agent_id"
	SummaryFieldProfile   SummaryField = "profile"
	SummaryFieldWorkspace SummaryField = "workspace"
	SummaryFieldTimeout   SummaryField = "timeout"
	SummaryFieldDefault   SummaryField = "default"
)

type SummaryFieldState struct {
	Value         string
	LastPatchedBy string
	LastPatchedAt time.Time
	LastSource    Source
}

type ControlPlaneSummary struct {
	Version         int64
	PatchCount      int
	LastPatchedBy   string
	LastPatchedAt   time.Time
	LastPatchSource Source
	Fields          map[SummaryField]SummaryFieldState
	RebuiltAt       time.Time
	RebuildReason   string
}

type Plan struct {
	Kind           Kind
	ConversationID string
	CreatedBy      string
	CreatedAt      time.Time
	Summary        string
	Source         Source
	Version        int64

	AgentUpsertOpts   *UpsertAgentOptions
	AgentDeleteOpts   *DeleteAgentOptions
	AgentRenameOpts   *RenameAgentOptions
	DefaultAgentID    string
	PatchHistory      []Patch
	CompressedSummary *ControlPlaneSummary
	SummaryVersion    int64
}

func (p Plan) Validate() error {
	switch p.Kind {
	case KindUpsertAgent:
		if p.AgentUpsertOpts == nil {
			return ErrInvalidPlan
		}
		if strings.TrimSpace(p.AgentUpsertOpts.ID) == "" {
			return ErrInvalidPlan
		}
		if strings.TrimSpace(p.AgentUpsertOpts.ProfileID) == "" {
			return ErrInvalidPlan
		}
	case KindSetDefaultAgent:
		if strings.TrimSpace(p.DefaultAgentID) == "" {
			return ErrInvalidPlan
		}
	case KindDeleteAgent:
		if p.AgentDeleteOpts == nil {
			return ErrInvalidPlan
		}
		if strings.TrimSpace(p.AgentDeleteOpts.ID) == "" {
			return ErrInvalidPlan
		}
	case KindRenameAgent:
		if p.AgentRenameOpts == nil {
			return ErrInvalidPlan
		}
		if strings.TrimSpace(p.AgentRenameOpts.FromID) == "" || strings.TrimSpace(p.AgentRenameOpts.ToID) == "" {
			return ErrInvalidPlan
		}
		if strings.TrimSpace(p.AgentRenameOpts.FromID) == strings.TrimSpace(p.AgentRenameOpts.ToID) {
			return ErrInvalidPlan
		}
	default:
		return ErrInvalidPlanKind
	}
	if strings.TrimSpace(p.ConversationID) == "" {
		return ErrInvalidPlan
	}
	return nil
}

func (p Plan) Normalize() Plan {
	p.ConversationID = strings.TrimSpace(p.ConversationID)
	p.CreatedBy = strings.TrimSpace(p.CreatedBy)
	p.Summary = strings.TrimSpace(p.Summary)
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	if strings.TrimSpace(string(p.Source)) == "" {
		p.Source = SourceSlash
	}
	if p.Version <= 0 {
		p.Version = 1
	}
	if p.AgentUpsertOpts != nil {
		p.AgentUpsertOpts.ID = strings.TrimSpace(p.AgentUpsertOpts.ID)
		p.AgentUpsertOpts.ProfileID = strings.TrimSpace(p.AgentUpsertOpts.ProfileID)
		p.AgentUpsertOpts.Workspace = strings.TrimSpace(p.AgentUpsertOpts.Workspace)
	}
	if p.AgentDeleteOpts != nil {
		p.AgentDeleteOpts.ID = strings.TrimSpace(p.AgentDeleteOpts.ID)
	}
	if p.AgentRenameOpts != nil {
		p.AgentRenameOpts.FromID = strings.TrimSpace(p.AgentRenameOpts.FromID)
		p.AgentRenameOpts.ToID = strings.TrimSpace(p.AgentRenameOpts.ToID)
		p.AgentRenameOpts.TargetWorkspace = strings.TrimSpace(p.AgentRenameOpts.TargetWorkspace)
	}
	p.DefaultAgentID = strings.TrimSpace(p.DefaultAgentID)
	for i := range p.PatchHistory {
		p.PatchHistory[i].Value = strings.TrimSpace(p.PatchHistory[i].Value)
		p.PatchHistory[i].By = strings.TrimSpace(p.PatchHistory[i].By)
		if p.PatchHistory[i].At.IsZero() {
			p.PatchHistory[i].At = p.CreatedAt
		}
		if strings.TrimSpace(string(p.PatchHistory[i].Source)) == "" {
			p.PatchHistory[i].Source = p.Source
		}
	}
	if p.CompressedSummary != nil {
		if p.CompressedSummary.Version <= 0 {
			p.CompressedSummary.Version = 1
		}
		if p.CompressedSummary.PatchCount < 0 {
			p.CompressedSummary.PatchCount = 0
		}
		if p.CompressedSummary.Fields == nil {
			p.CompressedSummary.Fields = make(map[SummaryField]SummaryFieldState)
		}
		for key, field := range p.CompressedSummary.Fields {
			field.Value = strings.TrimSpace(field.Value)
			field.LastPatchedBy = strings.TrimSpace(field.LastPatchedBy)
			if strings.TrimSpace(string(field.LastSource)) == "" {
				field.LastSource = p.Source
			}
			p.CompressedSummary.Fields[key] = field
		}
		p.CompressedSummary.LastPatchedBy = strings.TrimSpace(p.CompressedSummary.LastPatchedBy)
		p.CompressedSummary.RebuildReason = strings.TrimSpace(p.CompressedSummary.RebuildReason)
		if strings.TrimSpace(string(p.CompressedSummary.LastPatchSource)) == "" {
			p.CompressedSummary.LastPatchSource = p.Source
		}
		if p.CompressedSummary.RebuiltAt.IsZero() {
			p.CompressedSummary.RebuiltAt = p.CreatedAt
		}
	}
	if p.SummaryVersion < 0 {
		p.SummaryVersion = 0
	}
	if p.CompressedSummary != nil && p.SummaryVersion < p.CompressedSummary.Version {
		p.SummaryVersion = p.CompressedSummary.Version
	}
	return p
}
