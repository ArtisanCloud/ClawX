package execution

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidRun = errors.New("invalid execution run")

type ResultState string

const (
	ResultSuccess         ResultState = "success"
	ResultFailed          ResultState = "failed"
	ResultTimeout         ResultState = "timeout"
	ResultCancelled       ResultState = "cancelled"
	ResultPartialDelivery ResultState = "partial_delivery"
)

type Run struct {
	ID               string
	SessionID        string
	RequestSummary   string
	StartAt          time.Time
	EndAt            time.Time
	DurationMS       int64
	ResultState      ResultState
	DeliveryAttempts int
	FailureReason    string
}

func (r Run) Validate() error {
	if strings.TrimSpace(r.ID) == "" || strings.TrimSpace(r.SessionID) == "" {
		return ErrInvalidRun
	}
	if strings.TrimSpace(r.RequestSummary) == "" {
		return ErrInvalidRun
	}
	return nil
}

func (r *Run) Start(now time.Time) {
	r.StartAt = now.UTC()
}

func (r *Run) Finish(now time.Time, state ResultState, failureReason string) {
	r.EndAt = now.UTC()
	if !r.StartAt.IsZero() {
		r.DurationMS = r.EndAt.Sub(r.StartAt).Milliseconds()
	}
	r.ResultState = state
	r.FailureReason = strings.TrimSpace(failureReason)
}

