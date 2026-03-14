package memory

import (
	"strings"

	memorydomain "clawx/internal/domain/memory"
)

type ScopeInput struct {
	AgentID   string
	ProjectID string
	RouteKey  string
	SessionID string
	ChatMode  string
}

type ScopeResolver struct{}

func NewScopeResolver() *ScopeResolver {
	return &ScopeResolver{}
}

func (r *ScopeResolver) Resolve(input ScopeInput) (memorydomain.MemoryScopeKey, error) {
	scope := memorydomain.MemoryScopeKey{
		AgentID:   normalizeScopeIdentifier(input.AgentID),
		ProjectID: normalizeScopeIdentifier(input.ProjectID),
		RouteKey:  strings.TrimSpace(input.RouteKey),
		SessionID: strings.TrimSpace(input.SessionID),
		ChatMode:  parseChatMode(input.ChatMode, input.RouteKey),
	}
	if scope.AgentID == "" {
		scope.AgentID = "main"
	}
	if scope.ProjectID == "" {
		scope.ProjectID = "main"
	}
	if scope.RouteKey == "" {
		scope.RouteKey = "compat:unknown"
	}
	if err := scope.Validate(); err != nil {
		return memorydomain.MemoryScopeKey{}, err
	}
	return scope, nil
}

func parseChatMode(rawMode, routeKey string) memorydomain.ChatMode {
	switch strings.ToLower(strings.TrimSpace(rawMode)) {
	case string(memorydomain.ChatModeMain):
		return memorydomain.ChatModeMain
	case string(memorydomain.ChatModeShared):
		return memorydomain.ChatModeShared
	}
	if strings.Contains(strings.ToLower(strings.TrimSpace(routeKey)), ":direct:") {
		return memorydomain.ChatModeMain
	}
	return memorydomain.ChatModeShared
}

func normalizeScopeIdentifier(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
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
