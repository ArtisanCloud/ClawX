package session

import (
	"context"
	"errors"
)

var ErrSessionNotFound = errors.New("session not found")

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

