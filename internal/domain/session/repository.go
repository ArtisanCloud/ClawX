package session

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrSessionNotFound = errors.New("session not found")
var ErrWindowBindingNotFound = errors.New("window binding not found")

type Repository interface {
	Create(ctx context.Context, record Record) error
	Save(ctx context.Context, record Record) error
	GetByID(ctx context.Context, sessionID string) (Record, error)
	GetLatestByConversation(ctx context.Context, conversationID string) (Record, error)
	ListByConversation(ctx context.Context, conversationID string) ([]Record, error)
}

type Locker interface {
	Acquire(ctx context.Context, sessionID string) (string, error)
	Release(ctx context.Context, sessionID, lockToken string) error
}

type WindowBinding struct {
	WindowID         string
	CurrentSessionID string
	ConversationID   string
	UpdatedAt        time.Time
	LastUsedAt       time.Time
}

func (b WindowBinding) Validate() error {
	if strings.TrimSpace(b.WindowID) == "" {
		return ErrInvalidSession
	}
	if strings.TrimSpace(b.CurrentSessionID) == "" {
		return ErrInvalidSession
	}
	if strings.TrimSpace(b.ConversationID) == "" {
		return ErrInvalidSession
	}
	return nil
}

type WindowBindingRepository interface {
	GetWindowBinding(ctx context.Context, windowID string) (WindowBinding, error)
	SetWindowBinding(ctx context.Context, binding WindowBinding) error
	ListWindowBindingsByConversation(ctx context.Context, conversationID string) ([]WindowBinding, error)
}
