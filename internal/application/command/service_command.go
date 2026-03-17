package command

import (
	"errors"
	"strconv"
	"strings"
)

var ErrNotServiceControlCommand = errors.New("not service control command")

type ServiceControlKind string

const (
	ServiceControlStart  ServiceControlKind = "start"
	ServiceControlStop   ServiceControlKind = "stop"
	ServiceControlStatus ServiceControlKind = "status"
	ServiceControlLogs   ServiceControlKind = "logs"
)

type ServiceControlCommand struct {
	Kind    ServiceControlKind
	Name    string
	Command []string
	Tail    int
}

func ParseServiceControlCommand(raw string) (ServiceControlCommand, error) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return ServiceControlCommand{}, ErrInvalidControlCommand
	}
	if normalizeControlName(fields[0]) != "service" {
		return ServiceControlCommand{}, ErrNotServiceControlCommand
	}
	if len(fields) < 2 {
		return ServiceControlCommand{}, ErrInvalidControlCommand
	}

	cmd := ServiceControlCommand{Tail: 100}
	switch strings.ToLower(strings.TrimSpace(fields[1])) {
	case "start":
		if len(fields) < 5 {
			return ServiceControlCommand{}, ErrInvalidControlCommand
		}
		delimIndex := -1
		for idx := 3; idx < len(fields); idx++ {
			if fields[idx] == "--" {
				delimIndex = idx
				break
			}
		}
		if delimIndex == -1 || delimIndex == len(fields)-1 {
			return ServiceControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Kind = ServiceControlStart
		cmd.Name = strings.TrimSpace(fields[2])
		if cmd.Name == "" {
			return ServiceControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Command = append(cmd.Command, fields[delimIndex+1:]...)
	case "stop":
		if len(fields) != 3 {
			return ServiceControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Kind = ServiceControlStop
		cmd.Name = strings.TrimSpace(fields[2])
		if cmd.Name == "" {
			return ServiceControlCommand{}, ErrInvalidControlCommand
		}
	case "status":
		cmd.Kind = ServiceControlStatus
		if len(fields) > 3 {
			return ServiceControlCommand{}, ErrInvalidControlCommand
		}
		if len(fields) == 3 {
			cmd.Name = strings.TrimSpace(fields[2])
			if cmd.Name == "" {
				return ServiceControlCommand{}, ErrInvalidControlCommand
			}
		}
	case "logs":
		if len(fields) < 3 {
			return ServiceControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Kind = ServiceControlLogs
		cmd.Name = strings.TrimSpace(fields[2])
		if cmd.Name == "" {
			return ServiceControlCommand{}, ErrInvalidControlCommand
		}
		for i := 3; i < len(fields); i++ {
			token := strings.TrimSpace(fields[i])
			if !strings.HasPrefix(strings.ToLower(token), "--tail=") {
				return ServiceControlCommand{}, ErrInvalidControlCommand
			}
			rawValue := strings.TrimSpace(strings.TrimPrefix(token, "--tail="))
			value, err := strconv.Atoi(rawValue)
			if err != nil || value <= 0 {
				return ServiceControlCommand{}, ErrInvalidControlCommand
			}
			cmd.Tail = value
		}
	default:
		return ServiceControlCommand{}, ErrInvalidControlCommand
	}

	return cmd, nil
}
