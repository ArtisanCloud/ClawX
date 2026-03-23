package execution

import (
	"context"
	"time"
)

type Request struct {
	SessionID            string
	BackendSessionID     string
	CWD                  string
	AllowedRoots         []string
	PromptCacheKey       string
	PromptCacheRetention string
	MemoryContext        string
	MemoryScope          string
	MemoryACLMode        string
	Input                string
	Timeout              time.Duration
}

type Result struct {
	BackendSessionID     string
	Output               string
	PromptCacheKey       string
	PromptCacheRetention string
	PromptCachedTokens   int
	PromptTokens         int
	CompletionTokens     int
	TotalTokens          int
	MemoryScope          string
	MemoryACLMode        string
	State                ResultState
	StartedAt            time.Time
	CompletedAt          time.Time
	FailureReason        string
}

type Backend interface {
	Name() string
	Execute(ctx context.Context, request Request) (Result, error)
	Cancel(ctx context.Context, sessionID string) error
	HealthCheck(ctx context.Context) error
}
