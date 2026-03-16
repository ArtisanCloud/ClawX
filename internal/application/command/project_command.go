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
	ProjectControlBind    ProjectControlKind = "bind"
	ProjectControlUnbind  ProjectControlKind = "unbind"
	ProjectControlCurrent ProjectControlKind = "current"
	ProjectControlAudit   ProjectControlKind = "audit"
	ProjectControlDelete  ProjectControlKind = "delete"
	ProjectControlRepair  ProjectControlKind = "repair"
	ProjectControlSuggest ProjectControlKind = "suggest"
	ProjectControlConfirm ProjectControlKind = "confirm"
)

type ProjectControlCommand struct {
	Kind        ProjectControlKind
	RouteKey    string
	ProjectID   string
	ProjectName string
	ProposalID  string
	Confidence  float64
	Reason      string
	Force       bool
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
	case "bind":
		if len(fields) < 4 {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Kind = ProjectControlBind
		cmd.RouteKey = strings.TrimSpace(fields[2])
		cmd.ProjectID = strings.TrimSpace(fields[3])
		if cmd.RouteKey == "" || cmd.ProjectID == "" {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
	case "unbind":
		if len(fields) < 3 {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Kind = ProjectControlUnbind
		cmd.RouteKey = strings.TrimSpace(fields[2])
		if cmd.RouteKey == "" {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
	case "current":
		cmd.Kind = ProjectControlCurrent
	case "audit":
		cmd.Kind = ProjectControlAudit
	case "delete":
		if len(fields) < 3 {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Kind = ProjectControlDelete
		cmd.ProjectID = strings.TrimSpace(fields[2])
		if cmd.ProjectID == "" {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
		for _, token := range fields[3:] {
			if strings.EqualFold(strings.TrimSpace(token), "--force") {
				cmd.Force = true
			}
		}
	case "repair":
		if len(fields) < 3 {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Kind = ProjectControlRepair
		cmd.ProjectID = strings.TrimSpace(fields[2])
		if cmd.ProjectID == "" {
			return ProjectControlCommand{}, ErrInvalidControlCommand
		}
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
