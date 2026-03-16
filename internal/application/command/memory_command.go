package command

import (
	"errors"
	"strings"
)

var ErrNotMemoryControlCommand = errors.New("not memory control command")

type MemoryControlKind string

const (
	MemoryControlNote   MemoryControlKind = "note"
	MemoryControlDigest MemoryControlKind = "digest"
	MemoryControlAudit  MemoryControlKind = "audit"
)

type MemoryControlCommand struct {
	Kind   MemoryControlKind
	Text   string
	Shared bool
}

func ParseMemoryControlCommand(raw string) (MemoryControlCommand, error) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return MemoryControlCommand{}, ErrInvalidControlCommand
	}
	if normalizeControlName(fields[0]) != "memory" {
		return MemoryControlCommand{}, ErrNotMemoryControlCommand
	}
	if len(fields) < 2 {
		return MemoryControlCommand{}, ErrInvalidControlCommand
	}

	cmd := MemoryControlCommand{}
	switch strings.ToLower(strings.TrimSpace(fields[1])) {
	case "note":
		cmd.Kind = MemoryControlNote
		rest := fields[2:]
		for len(rest) > 0 && strings.HasPrefix(strings.TrimSpace(rest[0]), "--") {
			flag := strings.ToLower(strings.TrimSpace(rest[0]))
			if flag != "--shared" {
				return MemoryControlCommand{}, ErrInvalidControlCommand
			}
			cmd.Shared = true
			rest = rest[1:]
		}
		cmd.Text = strings.TrimSpace(strings.Join(rest, " "))
		if cmd.Text == "" {
			return MemoryControlCommand{}, ErrInvalidControlCommand
		}
	case "digest":
		if len(fields) != 2 {
			return MemoryControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Kind = MemoryControlDigest
	case "audit":
		if len(fields) != 2 {
			return MemoryControlCommand{}, ErrInvalidControlCommand
		}
		cmd.Kind = MemoryControlAudit
	default:
		return MemoryControlCommand{}, ErrInvalidControlCommand
	}

	return cmd, nil
}
