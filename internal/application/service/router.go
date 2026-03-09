package service

import (
	"context"
	"errors"
	"strings"

	"synapsex/internal/application/intent"
	"synapsex/internal/domain/execution"
	skilldomain "synapsex/internal/domain/skill"
	"synapsex/internal/infrastructure/config"
	"synapsex/internal/interfaces/chat"
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
	Message        chat.Message
	SkillName      string
	SkillInput     string
	IntentReason   string
	Confidence     float64
	Skill          *skilldomain.Definition
}

type Router struct {
	cfg            config.Snapshot
	sessionManager *SessionManager
	backend        execution.Backend
	intentPipeline *intent.Pipeline
}

type RouterOption func(*Router)

func WithIntentPipeline(pipeline *intent.Pipeline) RouterOption {
	return func(r *Router) {
		r.intentPipeline = pipeline
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
		Message:        message,
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

func isBareControlCommand(text string) bool {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "new", "resume", "list", "cancel", "current":
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
