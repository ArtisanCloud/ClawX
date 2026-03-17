package command

import (
	"errors"
	"regexp"
	"strings"
)

var ErrNotScheduleControlCommand = errors.New("not schedule control command")

type ScheduleControlKind string

const (
	ScheduleControlAdd    ScheduleControlKind = "add"
	ScheduleControlList   ScheduleControlKind = "list"
	ScheduleControlStatus ScheduleControlKind = "status"
	ScheduleControlPause  ScheduleControlKind = "pause"
	ScheduleControlResume ScheduleControlKind = "resume"
	ScheduleControlRun    ScheduleControlKind = "run"
	ScheduleControlRemove ScheduleControlKind = "remove"
)

type ScheduleControlCommand struct {
	Kind         ScheduleControlKind
	NameOrID     string
	CronExpr     string
	TaskType     string
	TaskArgs     map[string]string
	RouteScope   string
	TimezoneHint string
}

var addCronPattern = regexp.MustCompile(`(?i)(^|\s)--cron\s+("[^"]+"|'[^']+'|\S+)`)
var addTaskPattern = regexp.MustCompile(`(?i)(^|\s)--task\s+([^\s]+)`)
var addArgPattern = regexp.MustCompile(`(?i)(^|\s)--arg\s+([^\s]+)`)
var addRoutePattern = regexp.MustCompile(`(?i)(^|\s)--route\s+([^\s]+)`)

func ParseScheduleControlCommand(raw string) (ScheduleControlCommand, error) {
	text := strings.TrimSpace(raw)
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ScheduleControlCommand{}, ErrInvalidControlCommand
	}
	if normalizeControlName(fields[0]) != "schedule" {
		return ScheduleControlCommand{}, ErrNotScheduleControlCommand
	}
	if len(fields) < 2 {
		return ScheduleControlCommand{}, ErrInvalidControlCommand
	}

	verb := strings.ToLower(strings.TrimSpace(fields[1]))
	cmd := ScheduleControlCommand{TaskArgs: map[string]string{}}

	switch verb {
	case "list":
		if len(fields) != 2 {
			return ScheduleControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Kind = ScheduleControlList
		return cmd, nil
	case "status", "pause", "resume", "run", "remove":
		if len(fields) != 3 {
			return ScheduleControlCommand{}, ErrInvalidControlCommand
		}
		cmd.NameOrID = strings.TrimSpace(fields[2])
		if cmd.NameOrID == "" {
			return ScheduleControlCommand{}, ErrInvalidControlCommand
		}
		switch verb {
		case "status":
			cmd.Kind = ScheduleControlStatus
		case "pause":
			cmd.Kind = ScheduleControlPause
		case "resume":
			cmd.Kind = ScheduleControlResume
		case "run":
			cmd.Kind = ScheduleControlRun
		default:
			cmd.Kind = ScheduleControlRemove
		}
		return cmd, nil
	case "add":
		if len(fields) < 3 {
			return ScheduleControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Kind = ScheduleControlAdd
		cmd.NameOrID = strings.TrimSpace(fields[2])
		if cmd.NameOrID == "" {
			return ScheduleControlCommand{}, ErrInvalidControlCommand
		}
		tail := ""
		if len(fields) > 3 {
			tail = strings.TrimSpace(text[strings.Index(text, fields[3]):])
		}
		cronMatch := addCronPattern.FindStringSubmatch(tail)
		taskMatch := addTaskPattern.FindStringSubmatch(tail)
		if len(cronMatch) < 3 || len(taskMatch) < 3 {
			return ScheduleControlCommand{}, ErrInvalidControlCommand
		}
		cmd.CronExpr = trimQuotes(strings.TrimSpace(cronMatch[2]))
		cmd.TaskType = strings.TrimSpace(strings.ToLower(taskMatch[2]))
		if cmd.CronExpr == "" || cmd.TaskType == "" {
			return ScheduleControlCommand{}, ErrInvalidControlCommand
		}
		argMatches := addArgPattern.FindAllStringSubmatch(tail, -1)
		for _, match := range argMatches {
			if len(match) < 3 {
				continue
			}
			pair := strings.TrimSpace(match[2])
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) != 2 {
				return ScheduleControlCommand{}, ErrInvalidControlCommand
			}
			key := strings.TrimSpace(strings.ToLower(parts[0]))
			value := strings.TrimSpace(parts[1])
			if key == "" || value == "" {
				return ScheduleControlCommand{}, ErrInvalidControlCommand
			}
			cmd.TaskArgs[key] = value
		}
		if routeMatch := addRoutePattern.FindStringSubmatch(tail); len(routeMatch) >= 3 {
			cmd.RouteScope = strings.TrimSpace(routeMatch[2])
		}
		return cmd, nil
	default:
		return ScheduleControlCommand{}, ErrInvalidControlCommand
	}
}

func trimQuotes(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}
