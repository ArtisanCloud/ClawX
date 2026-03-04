package session

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidSession  = errors.New("invalid session")
	ErrSessionBusy     = errors.New("session is already running")
	ErrSessionNotBusy  = errors.New("session is not running")
	ErrInvalidLock     = errors.New("invalid lock token")
	ErrInvalidStatus   = errors.New("invalid session status")
	ErrMissingIdentity = errors.New("missing session identity")
)

type Status string

const (
	StatusIdle    Status = "idle"
	StatusRunning Status = "running"
	StatusError   Status = "error"
)

type Record struct {
	ID               string
	WindowID         string
	AgentID          string
	Backend          string
	BackendSessionID string
	ConversationID   string
	CWD              string
	Status           Status
	LockToken        string
	LastUsedAt       time.Time
}

func (r Record) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return ErrMissingIdentity
	}
	if strings.TrimSpace(r.ConversationID) == "" {
		return ErrInvalidSession
	}
	if strings.TrimSpace(r.Backend) == "" {
		return ErrInvalidSession
	}
	if strings.TrimSpace(r.CWD) == "" {
		return ErrInvalidSession
	}
	switch r.Status {
	case StatusIdle, StatusRunning, StatusError:
	default:
		return ErrInvalidStatus
	}
	if r.Status != StatusRunning && r.LockToken != "" {
		return ErrInvalidLock
	}
	return nil
}

func (r Record) IsRunning() bool {
	return r.Status == StatusRunning
}

func (r *Record) Touch(now time.Time) {
	r.LastUsedAt = now.UTC()
}

func (r *Record) BindBackendSessionID(backendSessionID string) {
	r.BackendSessionID = strings.TrimSpace(backendSessionID)
}

func (r *Record) StartExecution(lockToken string, now time.Time) error {
	if r.IsRunning() {
		return ErrSessionBusy
	}
	lockToken = strings.TrimSpace(lockToken)
	if lockToken == "" {
		return ErrInvalidLock
	}
	r.LockToken = lockToken
	r.Status = StatusRunning
	r.Touch(now)
	return nil
}

func (r *Record) FinishExecution(nextStatus Status, now time.Time) error {
	if !r.IsRunning() {
		return ErrSessionNotBusy
	}
	switch nextStatus {
	case StatusIdle, StatusError:
	default:
		return ErrInvalidStatus
	}
	r.LockToken = ""
	r.Status = nextStatus
	r.Touch(now)
	return nil
}

