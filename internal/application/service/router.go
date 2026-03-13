package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"clawx/internal/application/intent"
	"clawx/internal/domain/execution"
	projectdomain "clawx/internal/domain/project"
	skilldomain "clawx/internal/domain/skill"
	"clawx/internal/infrastructure/config"
	"clawx/internal/interfaces/chat"
)

var (
	ErrRejectedContext = errors.New("channel context is not allowed")
	ErrEmptyMessage    = errors.New("message text is empty")
)

type DecisionKind string

const (
	DecisionExecute DecisionKind = "execute"
	DecisionControl DecisionKind = "control"
	DecisionSkill   DecisionKind = "skill"
)

type Decision struct {
	Kind           DecisionKind
	Command        string
	ConversationID string
	WindowID       string
	RouteKey       string
	ProjectID      string
	ProjectMode    string
	Message        chat.Message
	SkillName      string
	SkillInput     string
	IntentReason   string
	Confidence     float64
	Skill          *skilldomain.Definition
}

type ProjectResolver interface {
	ResolveProject(ctx context.Context, routeKey string) (projectID string, routingMode string, err error)
}

type ProjectCommandService interface {
	ProjectResolver
	CreateProject(ctx context.Context, projectID, name, workspacePath string) (projectdomain.Record, error)
	ListProjects(ctx context.Context) ([]projectdomain.Record, error)
	UseProject(ctx context.Context, routeKey, projectID, updatedBy string) (projectdomain.RouteBinding, error)
	BindRoute(ctx context.Context, routeKey, projectID, updatedBy string) (projectdomain.RouteBinding, error)
	UnbindRoute(ctx context.Context, routeKey string) error
	GetProject(ctx context.Context, projectID string) (projectdomain.Record, error)
	AuditProjects(ctx context.Context) (projectdomain.AuditReport, error)
	DeleteProject(ctx context.Context, projectID string, force bool) (projectdomain.Record, error)
	RepairProject(ctx context.Context, projectID string) (projectdomain.Record, error)
	SuggestProjectSwitch(ctx context.Context, routeKey, fromProjectID, toProjectID, reason string, confidence float64, createdBy string) (projectdomain.Proposal, error)
	ConfirmProjectSwitch(ctx context.Context, proposalID, updatedBy string) (projectdomain.RouteBinding, error)
}

type Router struct {
	cfg            config.Snapshot
	sessionManager *SessionManager
	backend        execution.Backend
	intentPipeline *intent.Pipeline
	project        ProjectResolver
	projectControl ProjectCommandService
}

type RouterOption func(*Router)

func WithIntentPipeline(pipeline *intent.Pipeline) RouterOption {
	return func(r *Router) {
		r.intentPipeline = pipeline
	}
}

func WithProjectResolver(resolver ProjectResolver) RouterOption {
	return func(r *Router) {
		r.project = resolver
		if control, ok := resolver.(ProjectCommandService); ok {
			r.projectControl = control
		}
	}
}

func NewRouter(cfg config.Snapshot, sessionManager *SessionManager, backend execution.Backend, options ...RouterOption) *Router {
	router := &Router{
		cfg:            cfg,
		sessionManager: sessionManager,
		backend:        backend,
	}
	for _, option := range options {
		if option != nil {
			option(router)
		}
	}
	return router
}

func (r *Router) Route(ctx context.Context, message chat.Message) (Decision, error) {
	if err := r.ValidateContext(message); err != nil {
		return Decision{}, err
	}

	text := strings.TrimSpace(message.Text)
	if text == "" {
		return Decision{}, ErrEmptyMessage
	}

	decision := Decision{
		ConversationID: message.ConversationID,
		WindowID:       message.WindowID,
		RouteKey:       strings.TrimSpace(message.RouteKey),
		Message:        message,
	}
	if err := r.resolveProject(ctx, &decision); err != nil {
		return Decision{}, err
	}

	if isBuiltInControlCommand(text) {
		decision.Kind = DecisionControl
		decision.Command = text
		return decision, nil
	}

	if r.intentPipeline != nil {
		intentResult, err := r.intentPipeline.Decide(ctx, message)
		if err != nil {
			return Decision{}, err
		}
		decision.IntentReason = intentResult.Decision.Reason
		decision.Confidence = intentResult.Decision.Confidence
		decision.Skill = intentResult.Skill
		switch intentResult.Decision.Kind {
		case skilldomain.IntentControl:
			decision.Kind = DecisionControl
			decision.Command = text
		case skilldomain.IntentSkill:
			decision.Kind = DecisionSkill
			decision.SkillName = skilldomain.NormalizeName(intentResult.Decision.SkillName)
			decision.SkillInput = strings.TrimSpace(intentResult.SkillInput)
		default:
			decision.Kind = DecisionExecute
		}
		if strings.TrimSpace(intentResult.ProposalProjectID) != "" && r.projectControl != nil {
			targetProjectID := normalizeProjectSwitchID(intentResult.ProposalProjectID)
			currentProjectID := normalizeProjectSwitchID(decision.ProjectID)
			if targetProjectID != "" && targetProjectID != currentProjectID {
				decision.Kind = DecisionControl
				decision.Command = buildProjectSuggestCommand(targetProjectID, intentResult.ProposalConfidence, intentResult.ProposalReason)
			}
		}
		return decision, nil
	}

	if strings.HasPrefix(text, "/") {
		decision.Kind = DecisionControl
		decision.Command = text
		return decision, nil
	}
	if isBareControlCommand(text) {
		decision.Kind = DecisionControl
		decision.Command = text
		return decision, nil
	}

	decision.Kind = DecisionExecute
	return decision, nil
}

func (r *Router) resolveProject(ctx context.Context, decision *Decision) error {
	if decision == nil {
		return nil
	}

	projectID := strings.TrimSpace(r.cfg.Projects.DefaultProjectID)
	if projectID == "" {
		projectID = "main"
	}
	mode := "fallback"

	routeKey := strings.TrimSpace(decision.RouteKey)
	if routeKey == "" {
		routeKey = "compat:" + strings.TrimSpace(decision.ConversationID)
	}
	decision.RouteKey = routeKey

	if r.project != nil {
		resolvedID, resolvedMode, err := r.project.ResolveProject(ctx, routeKey)
		if err != nil {
			return err
		}
		if strings.TrimSpace(resolvedID) != "" {
			projectID = strings.TrimSpace(resolvedID)
		}
		if strings.TrimSpace(resolvedMode) != "" {
			mode = strings.TrimSpace(resolvedMode)
		}
	}

	decision.ProjectID = projectID
	decision.ProjectMode = mode
	return nil
}

func isBareControlCommand(text string) bool {
	if strings.HasPrefix(strings.TrimSpace(text), "/") {
		return false
	}
	return isBuiltInControlCommand(text)
}

func isBuiltInControlCommand(text string) bool {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return false
	}
	name := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fields[0])), "/")
	switch name {
	case "new", "resume", "switch", "list", "cancel", "current", "project":
		return true
	default:
		return false
	}
}

func normalizeProjectSwitchID(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(trimmed))
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-_")
}

func buildProjectSuggestCommand(projectID string, confidence float64, reason string) string {
	projectID = normalizeProjectSwitchID(projectID)
	reason = strings.TrimSpace(strings.Join(strings.Fields(reason), " "))
	if reason == "" {
		reason = "intent_project_switch"
	}
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}
	return fmt.Sprintf("/project suggest %s %.2f %s", projectID, confidence, reason)
}

func (r *Router) ValidateContext(message chat.Message) error {
	if strings.TrimSpace(message.ConversationID) == "" || strings.TrimSpace(message.UserID) == "" {
		return ErrRejectedContext
	}
	if !message.ContextFlags.IsAllowed {
		return ErrRejectedContext
	}
	if !r.cfg.IsChannelEnabled(message.Channel) {
		return ErrRejectedContext
	}
	return nil
}
