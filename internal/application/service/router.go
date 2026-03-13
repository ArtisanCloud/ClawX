package service

import (
	"context"
	"errors"
	"strings"

	"clawx/internal/application/intent"
	"clawx/internal/domain/execution"
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

type Router struct {
	cfg            config.Snapshot
	sessionManager *SessionManager
	backend        execution.Backend
	intentPipeline *intent.Pipeline
	project        ProjectResolver
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
	case "new", "resume", "switch", "list", "cancel", "current":
		return true
	default:
		return false
	}
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
