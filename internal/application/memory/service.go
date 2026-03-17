package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	memorydomain "clawx/internal/domain/memory"
)

type ProjectResolver interface {
	ResolveProject(ctx context.Context, routeKey string) (projectID string, routingMode string, err error)
}

type CommandService struct {
	templateRepo      memorydomain.TemplateRepository
	journalRepo       memorydomain.JournalRepository
	auditRepo         memorydomain.AuditRepository
	digestRepo        memorydomain.DigestRepository
	projectResolver   ProjectResolver
	scopeResolver     *ScopeResolver
	workspaceRoot     string
	ownerAllowlist    []string
	autoDigestEnabled bool
	now               func() time.Time
}

type CommandServiceOption func(*CommandService)

func WithCommandProjectResolver(resolver ProjectResolver) CommandServiceOption {
	return func(s *CommandService) {
		s.projectResolver = resolver
	}
}

func WithCommandScopeResolver(resolver *ScopeResolver) CommandServiceOption {
	return func(s *CommandService) {
		if resolver != nil {
			s.scopeResolver = resolver
		}
	}
}

func WithCommandWorkspaceRoot(root string) CommandServiceOption {
	return func(s *CommandService) {
		s.workspaceRoot = strings.TrimSpace(root)
	}
}

func WithCommandOwnerAllowlist(owners []string) CommandServiceOption {
	return func(s *CommandService) {
		s.ownerAllowlist = append([]string(nil), owners...)
	}
}

func WithCommandAutoDigestEnabled(enabled bool) CommandServiceOption {
	return func(s *CommandService) {
		s.autoDigestEnabled = enabled
	}
}

func WithCommandClock(now func() time.Time) CommandServiceOption {
	return func(s *CommandService) {
		if now != nil {
			s.now = now
		}
	}
}

func NewCommandService(
	templateRepo memorydomain.TemplateRepository,
	journalRepo memorydomain.JournalRepository,
	auditRepo memorydomain.AuditRepository,
	digestRepo memorydomain.DigestRepository,
	options ...CommandServiceOption,
) *CommandService {
	service := &CommandService{
		templateRepo:  templateRepo,
		journalRepo:   journalRepo,
		auditRepo:     auditRepo,
		digestRepo:    digestRepo,
		scopeResolver: NewScopeResolver(),
		workspaceRoot: ".",
		now:           func() time.Time { return time.Now().UTC() },
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	if service.scopeResolver == nil {
		service.scopeResolver = NewScopeResolver()
	}
	if strings.TrimSpace(service.workspaceRoot) == "" {
		service.workspaceRoot = "."
	}
	return service
}

type NoteInput struct {
	RouteKey        string
	ProjectID       string
	AgentID         string
	UserID          string
	IsDirectMessage bool
	Text            string
	Shared          bool
	RequestedBy     string
}

type NoteResult struct {
	Scope     string
	Path      string
	Timestamp time.Time
}

type DigestInput struct {
	RouteKey        string
	ProjectID       string
	AgentID         string
	UserID          string
	IsDirectMessage bool
	RequestedBy     string
}

type DigestResult struct {
	JobID             string
	Status            memorydomain.DigestStatus
	OutputFile        string
	AutoDigestEnabled bool
}

type AuditInput struct {
	RouteKey        string
	ProjectID       string
	AgentID         string
	UserID          string
	IsDirectMessage bool
	Limit           int
}

type AuditResult struct {
	TemplateVersion    string
	RequiredFiles      int
	MissingRequired    int
	ACLDeniedCount     int
	BudgetSkippedCount int
	RecentErrors       []string
}

type commandScopeContext struct {
	projectID      string
	routeKey       string
	scope          memorydomain.MemoryScopeKey
	profile        memorydomain.MemoryProfile
	classification SessionClassResult
	guard          *PathGuard
}

type scopeInput struct {
	RouteKey        string
	ProjectID       string
	AgentID         string
	UserID          string
	IsDirectMessage bool
}

func (s *CommandService) resolveScopeContext(ctx context.Context, input scopeInput) (commandScopeContext, error) {
	projectID := sanitizePathSegment(strings.TrimSpace(input.ProjectID))
	routeKey := strings.TrimSpace(input.RouteKey)
	if routeKey == "" {
		routeKey = "memory:control:direct:unknown"
	}
	if projectID == "" && s.projectResolver != nil {
		resolvedProjectID, _, err := s.projectResolver.ResolveProject(ctx, routeKey)
		if err != nil {
			return commandScopeContext{}, err
		}
		projectID = sanitizePathSegment(resolvedProjectID)
	}
	if projectID == "" {
		projectID = "main"
	}

	agentID := sanitizePathSegment(strings.TrimSpace(input.AgentID))
	if agentID == "" {
		agentID = "main"
	}

	scope, err := s.scopeResolver.Resolve(ScopeInput{
		AgentID:   agentID,
		ProjectID: projectID,
		RouteKey:  routeKey,
	})
	if err != nil {
		return commandScopeContext{}, err
	}

	profile := memorydomain.MemoryProfile{
		ScopeKey:         scope,
		TokenBudget:      1,
		ACLMode:          memorydomain.ACLModeStrict,
		AllowMainPrivate: true,
	}
	acl := ApplyLoaderACL(LoaderACLInput{
		Profile:         profile,
		RouteKey:        routeKey,
		UserID:          strings.TrimSpace(input.UserID),
		IsDirectMessage: input.IsDirectMessage,
		OwnerAllowlist:  s.ownerAllowlist,
	})
	profile = acl.Profile

	projectRoot := filepath.Join(strings.TrimSpace(s.workspaceRoot), projectID)
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		return commandScopeContext{}, err
	}
	guard, err := NewPathGuard(projectRoot)
	if err != nil {
		return commandScopeContext{}, err
	}
	if err := guard.EnsureAgentPrivateLayout(agentID); err != nil {
		return commandScopeContext{}, err
	}

	return commandScopeContext{
		projectID:      projectID,
		routeKey:       routeKey,
		scope:          profile.ScopeKey,
		profile:        profile,
		classification: acl.Classification,
		guard:          guard,
	}, nil
}

func (s *CommandService) appendAuditRecord(ctx context.Context, record memorydomain.AuditRecord) {
	if s.auditRepo == nil {
		return
	}
	_ = s.auditRepo.Append(ctx, record)
}

func truncateText(value string, max int) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	if max <= 0 || len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}
