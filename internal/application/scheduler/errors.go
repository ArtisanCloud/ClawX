package scheduler

import "strings"

type CommandError struct {
	code       string
	reason     string
	nextAction string
}

func (e *CommandError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{strings.TrimSpace(e.code)}
	if strings.TrimSpace(e.reason) != "" {
		parts = append(parts, strings.TrimSpace(e.reason))
	}
	if strings.TrimSpace(e.nextAction) != "" {
		parts = append(parts, "next_action="+strings.TrimSpace(e.nextAction))
	}
	return strings.Join(parts, ": ")
}

func (e *CommandError) Code() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.code)
}

func (e *CommandError) NextAction() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.nextAction)
}

func newCommandError(code, reason, nextAction string) error {
	return &CommandError{code: strings.TrimSpace(code), reason: strings.TrimSpace(reason), nextAction: strings.TrimSpace(nextAction)}
}
