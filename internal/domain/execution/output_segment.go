package execution

import (
	"errors"
	"strings"
)

var ErrInvalidOutputSegment = errors.New("invalid output segment")

type DeliveryState string

const (
	DeliveryPending DeliveryState = "pending"
	DeliverySent    DeliveryState = "sent"
	DeliveryFailed  DeliveryState = "failed"
)

type OutputSegment struct {
	SessionID     string
	Sequence      int
	Content       string
	IsFinal       bool
	DeliveryState DeliveryState
}

func (s OutputSegment) Validate() error {
	if strings.TrimSpace(s.SessionID) == "" {
		return ErrInvalidOutputSegment
	}
	if s.Sequence <= 0 {
		return ErrInvalidOutputSegment
	}
	if s.Content == "" {
		return ErrInvalidOutputSegment
	}
	switch s.DeliveryState {
	case DeliveryPending, DeliverySent, DeliveryFailed:
	default:
		return ErrInvalidOutputSegment
	}
	return nil
}

