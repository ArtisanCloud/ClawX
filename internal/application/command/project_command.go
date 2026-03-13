package command

import (
	"errors"
	"strings"
)

var ErrNotProjectControlCommand = errors.New("not project control command")

type ProjectControlKind string

const (
	ProjectControlCreate  ProjectControlKind = "create"
	ProjectControlList    ProjectControlKind = "list"
	ProjectControlUse     ProjectControlKind = "use"
	ProjectControlCurrent ProjectControlKind = "current"
)

type ProjectControlCommand struct {
	Kind        ProjectControlKind
	ProjectID   string
	ProjectName string
}

func ParseProjectControlCommand(raw string) (ProjectControlCommand, error) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return ProjectControlCommand{}, ErrInvalidControlCommand
	}
	if normalizeControlName(fields[0]) != "project" {
		return ProjectControlCommand{}, ErrNotProjectControlCommand
	}
	if len(fields) < 2 {
		return ProjectControlCommand{}, ErrInvalidControlCommand
	}

	cmd := ProjectControlCommand{}
	switch strings.ToLower(strings.TrimSpace(fields[1])) {
	case "create":
		if len(fields) < 3 {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Kind = ProjectControlCreate
		cmd.ProjectID = strings.TrimSpace(fields[2])
		if cmd.ProjectID == "" {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
		if len(fields) > 3 {
			cmd.ProjectName = strings.TrimSpace(strings.Join(fields[3:], " "))
		}
	case "list":
		cmd.Kind = ProjectControlList
	case "use":
		if len(fields) < 3 {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Kind = ProjectControlUse
		cmd.ProjectID = strings.TrimSpace(fields[2])
		if cmd.ProjectID == "" {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
	case "current":
		cmd.Kind = ProjectControlCurrent
	default:
		return ProjectControlCommand{}, ErrInvalidControlCommand
	}
	return cmd, nil
}
