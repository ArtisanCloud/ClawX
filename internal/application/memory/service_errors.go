package memory

import "strings"

type CommandError struct {
	code   string
	reason string
}

func (e *CommandError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.reason) == "" {
		return e.code
	}
	return e.code + ": " + e.reason
}

func (e *CommandError) Code() string {
	if e == nil {
		return ""
	}
	return e.code
}

func newCommandError(code, reason string) error {
	return &CommandError{
		code:   strings.TrimSpace(code),
		reason: strings.TrimSpace(reason),
	}
}
