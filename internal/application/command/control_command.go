package command

import (
	"errors"
	"strings"
)

var ErrInvalidControlCommand = errors.New("invalid control command")

type ControlKind string

const (
	ControlNew     ControlKind = "new"
	ControlResume  ControlKind = "resume"
	ControlList    ControlKind = "list"
	ControlCancel  ControlKind = "cancel"
	ControlCurrent ControlKind = "current"
)

type ControlCommand struct {
	Kind            ControlKind
	ConversationID  string
	WindowID        string
	TargetSessionID string
}

func ParseControlCommand(raw, conversationID string, windowID ...string) (ControlCommand, error) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return ControlCommand{}, ErrInvalidControlCommand
	}

	normalizedConversationID := strings.TrimSpace(conversationID)
	normalizedWindowID := ""
	if len(windowID) > 0 {
		normalizedWindowID = strings.TrimSpace(windowID[0])
	}
	if normalizedWindowID == "" && normalizedConversationID != "" {
		normalizedWindowID = "compat:" + normalizedConversationID
	}

	command := ControlCommand{
		ConversationID: normalizedConversationID,
		WindowID:       normalizedWindowID,
	}

	switch normalizeControlName(fields[0]) {
	case "new":
		command.Kind = ControlNew
	case "resume":
		command.Kind = ControlResume
	case "list":
		command.Kind = ControlList
	case "cancel":
		command.Kind = ControlCancel
	case "current":
		command.Kind = ControlCurrent
	default:
		return ControlCommand{}, ErrInvalidControlCommand
	}

	if command.ConversationID == "" {
		return ControlCommand{}, ErrInvalidControlCommand
	}

	if command.Kind == ControlResume {
		if len(fields) < 2 {
			return ControlCommand{}, ErrInvalidControlCommand
		}
		command.TargetSessionID = strings.TrimSpace(fields[1])
		if command.TargetSessionID == "" {
			return ControlCommand{}, ErrInvalidControlCommand
		}
	}

	return command, nil
}

func normalizeControlName(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	name = strings.TrimPrefix(name, "/")
	return name
}
