package command

import (
	"errors"
	"strings"
)

var ErrInvalidSessionCommand = errors.New("invalid session command")

type SessionMode string

const (
	ModeNew      SessionMode = "new"
	ModeResume   SessionMode = "resume"
	ModeContinue SessionMode = "continue"
)

type SessionCommand struct {
	Mode            SessionMode
	ConversationID  string
	WindowID        string
	ProjectID       string
	RouteKey        string
	UserID          string
	IsDirectMessage bool
	ResumeSessionID string
	Input           string
	Backend         string
	CWD             string
}

func (c SessionCommand) Normalize() (SessionCommand, error) {
	normalized := c
	normalized.ConversationID = strings.TrimSpace(normalized.ConversationID)
	normalized.WindowID = strings.TrimSpace(normalized.WindowID)
	normalized.ProjectID = strings.TrimSpace(normalized.ProjectID)
	normalized.RouteKey = strings.TrimSpace(normalized.RouteKey)
	normalized.UserID = strings.TrimSpace(normalized.UserID)
	normalized.ResumeSessionID = strings.TrimSpace(normalized.ResumeSessionID)
	normalized.Input = strings.TrimSpace(normalized.Input)
	normalized.Backend = strings.TrimSpace(normalized.Backend)
	normalized.CWD = strings.TrimSpace(normalized.CWD)

	if normalized.Mode == "" {
		normalized.Mode = ModeContinue
	}
	if normalized.Backend == "" {
		normalized.Backend = "primary"
	}
	if normalized.CWD == "" {
		normalized.CWD = "."
	}
	if normalized.ConversationID == "" {
		return SessionCommand{}, ErrInvalidSessionCommand
	}
	if normalized.WindowID == "" {
		normalized.WindowID = "compat:" + normalized.ConversationID
	}

	switch normalized.Mode {
	case ModeNew, ModeResume, ModeContinue:
	default:
		return SessionCommand{}, ErrInvalidSessionCommand
	}

	if normalized.Mode == ModeResume && normalized.ResumeSessionID == "" {
		return SessionCommand{}, ErrInvalidSessionCommand
	}
	if normalized.Mode != ModeResume && normalized.Input == "" {
		return SessionCommand{}, ErrInvalidSessionCommand
	}

	return normalized, nil
}
