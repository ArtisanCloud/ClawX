package command

import (
	"errors"
	"strings"
)

var ErrInvalidControlCommand = errors.New("invalid control command")

type ControlKind string

const (
	ControlNew    ControlKind = "new"
	ControlResume ControlKind = "resume"
	ControlList   ControlKind = "list"
	ControlCancel ControlKind = "cancel"
)

type ControlCommand struct {
	Kind            ControlKind
	ConversationID  string
	TargetSessionID string
}

func ParseControlCommand(raw, conversationID string) (ControlCommand, error) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return ControlCommand{}, ErrInvalidControlCommand
	}

	command := ControlCommand{
		ConversationID: strings.TrimSpace(conversationID),
	}

	switch fields[0] {
	case "/new":
		command.Kind = ControlNew
	case "/resume":
		command.Kind = ControlResume
	case "/list":
		command.Kind = ControlList
	case "/cancel":
		command.Kind = ControlCancel
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
