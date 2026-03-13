package command

import (
	"errors"
	"strconv"
	"strings"
)

var ErrNotProjectControlCommand = errors.New("not project control command")

type ProjectControlKind string

const (
	ProjectControlCreate  ProjectControlKind = "create"
	ProjectControlList    ProjectControlKind = "list"
	ProjectControlUse     ProjectControlKind = "use"
	ProjectControlCurrent ProjectControlKind = "current"
	ProjectControlSuggest ProjectControlKind = "suggest"
	ProjectControlConfirm ProjectControlKind = "confirm"
)

type ProjectControlCommand struct {
	Kind        ProjectControlKind
	ProjectID   string
	ProjectName string
	ProposalID  string
	Confidence  float64
	Reason      string
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
	case "suggest":
		if len(fields) < 3 {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Kind = ProjectControlSuggest
		cmd.ProjectID = strings.TrimSpace(fields[2])
		if cmd.ProjectID == "" {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
		startReasonIndex := 3
		if len(fields) > 3 {
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(fields[3]), 64); err == nil {
				cmd.Confidence = parsed
				startReasonIndex = 4
			}
		}
		if len(fields) > startReasonIndex {
			cmd.Reason = strings.TrimSpace(strings.Join(fields[startReasonIndex:], " "))
		}
	case "confirm":
		if len(fields) < 3 {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Kind = ProjectControlConfirm
		cmd.ProposalID = strings.TrimSpace(fields[2])
		if cmd.ProposalID == "" {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
	default:
		return ProjectControlCommand{}, ErrInvalidControlCommand
	}
	return cmd, nil
}
