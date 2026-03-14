package memory

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidMemoryScope   = errors.New("invalid memory scope")
	ErrInvalidMemoryProfile = errors.New("invalid memory profile")
	ErrInvalidMemoryItem    = errors.New("invalid memory load item")
	ErrMemoryNotFound       = errors.New("memory record not found")
)

type ChatMode string

const (
	ChatModeMain   ChatMode = "main"
	ChatModeShared ChatMode = "shared"
)

type ACLMode string

const (
	ACLModeStrict   ACLMode = "strict"
	ACLModeDegraded ACLMode = "degraded"
)

type Layer string

const (
	LayerAgentPrivate Layer = "agent_private"
	LayerProjectShare Layer = "project_shared"
	LayerMainPrivate  Layer = "main_private"
)

type LoadDecision string

const (
	DecisionLoaded        LoadDecision = "loaded"
	DecisionSkippedACL    LoadDecision = "skipped_acl"
	DecisionSkippedBudget LoadDecision = "skipped_budget"
	DecisionError         LoadDecision = "error"
)

type MemoryScopeKey struct {
	AgentID   string
	ProjectID string
	RouteKey  string
	SessionID string
	ChatMode  ChatMode
}

func (s MemoryScopeKey) Validate() error {
	if strings.TrimSpace(s.AgentID) == "" ||
		strings.TrimSpace(s.ProjectID) == "" ||
		strings.TrimSpace(s.RouteKey) == "" {
		return ErrInvalidMemoryScope
	}
	switch s.ChatMode {
	case ChatModeMain, ChatModeShared:
	default:
		return ErrInvalidMemoryScope
	}
	return nil
}

type MemoryProfile struct {
	ScopeKey         MemoryScopeKey
	LoadOrder        []Layer
	TokenBudget      int
	ACLMode          ACLMode
	AllowMainPrivate bool
}

func (p MemoryProfile) Validate() error {
	if err := p.ScopeKey.Validate(); err != nil {
		return err
	}
	if p.TokenBudget <= 0 {
		return ErrInvalidMemoryProfile
	}
	switch p.ACLMode {
	case "", ACLModeStrict, ACLModeDegraded:
	default:
		return ErrInvalidMemoryProfile
	}
	return nil
}

type MemoryLoadItem struct {
	Layer     Layer
	Path      string
	SizeBytes int
	Priority  int
	Decision  LoadDecision
	Reason    string
}

func (i MemoryLoadItem) Validate() error {
	if strings.TrimSpace(i.Path) == "" {
		return ErrInvalidMemoryItem
	}
	switch i.Layer {
	case LayerAgentPrivate, LayerProjectShare, LayerMainPrivate:
	default:
		return ErrInvalidMemoryItem
	}
	switch i.Decision {
	case "", DecisionLoaded, DecisionSkippedACL, DecisionSkippedBudget, DecisionError:
	default:
		return ErrInvalidMemoryItem
	}
	return nil
}

type TemplateManifest struct {
	TemplateVersion string    `json:"templateVersion"`
	RequiredFiles   []string  `json:"requiredFiles"`
	OptionalFiles   []string  `json:"optionalFiles"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type JournalEntry struct {
	EntryID        string    `json:"entryId"`
	CreatedAt      time.Time `json:"createdAt"`
	Author         string    `json:"author"`
	Scope          string    `json:"scope"`
	ContentSummary string    `json:"contentSummary"`
	RawRef         string    `json:"rawRef"`
}

type AuditRecord struct {
	Timestamp   time.Time      `json:"timestamp"`
	ScopeKey    MemoryScopeKey `json:"scopeKey"`
	LoadedFiles []string       `json:"loadedFiles"`
	DeniedFiles []string       `json:"deniedFiles"`
	ErrorFiles  []string       `json:"errorFiles"`
	ACLMode     ACLMode        `json:"aclMode"`
	Degraded    bool           `json:"degraded"`
}

type DigestStatus string

const (
	DigestPending     DigestStatus = "pending"
	DigestRunning     DigestStatus = "running"
	DigestCompleted   DigestStatus = "completed"
	DigestFailed      DigestStatus = "failed"
	DigestRejectedACL DigestStatus = "rejected_acl"
)

type DigestTriggerMode string

const (
	DigestTriggerManual DigestTriggerMode = "manual"
	DigestTriggerAuto   DigestTriggerMode = "auto"
)

type DigestJob struct {
	JobID       string            `json:"jobId"`
	ScopeKey    MemoryScopeKey    `json:"scopeKey"`
	TriggerMode DigestTriggerMode `json:"triggerMode"`
	Status      DigestStatus      `json:"status"`
	StartedAt   time.Time         `json:"startedAt"`
	EndedAt     time.Time         `json:"endedAt"`
	OutputFile  string            `json:"outputFile"`
}
