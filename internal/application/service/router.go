package service

import (
	"context"
	"errors"
	"strings"

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
)

type Decision struct {
	Kind           DecisionKind
	Command        string
	ConversationID string
	Message        chat.Message
}

type Router struct {
	cfg            config.Snapshot
	sessionManager *SessionManager
}

func NewRouter(cfg config.Snapshot, sessionManager *SessionManager) *Router {
	return &Router{
		cfg:            cfg,
		sessionManager: sessionManager,
	}
}

func (r *Router) Route(_ context.Context, message chat.Message) (Decision, error) {
	if err := r.ValidateContext(message); err != nil {
		return Decision{}, err
	}

	text := strings.TrimSpace(message.Text)
	if text == "" {
		return Decision{}, ErrEmptyMessage
	}

	decision := Decision{
		ConversationID: message.ConversationID,
		Message:        message,
	}

	if strings.HasPrefix(text, "/") {
		decision.Kind = DecisionControl
		decision.Command = text
		return decision, nil
	}

	decision.Kind = DecisionExecute
	return decision, nil
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

