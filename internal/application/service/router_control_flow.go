package service

import (
	"context"
	"errors"

	"clawx/internal/application/command"
	"clawx/internal/domain/session"
)

type ControlFlowResult struct {
	CreatedSessionID   string
	ResumedSessionID   string
	SwitchedSessionID  string
	CancelledSessionID string
	CancelNoop         bool
	Sessions           []SessionSummary
	CurrentSession     *SessionSummary
	CurrentChecked     bool
}

func (r *Router) HandleControlCommand(ctx context.Context, rawCommand, conversationID string, windowID ...string) (ControlFlowResult, error) {
	parsed, err := command.ParseControlCommand(rawCommand, conversationID, windowID...)
	if err != nil {
		return ControlFlowResult{}, err
	}

	switch parsed.Kind {
	case command.ControlNew:
		record, err := r.sessionManager.CreateSession(ctx, command.SessionCommand{
			Mode:           command.ModeNew,
			ConversationID: parsed.ConversationID,
			WindowID:       parsed.WindowID,
			Backend:        r.backend.Name(),
			CWD:            r.cfg.DefaultCWD,
		})
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{CreatedSessionID: record.ID}, nil
	case command.ControlResume:
		record, err := r.sessionManager.ResumeSession(ctx, command.SessionCommand{
			Mode:            command.ModeResume,
			ConversationID:  parsed.ConversationID,
			WindowID:        parsed.WindowID,
			ResumeSessionID: parsed.TargetSessionID,
			Backend:         r.backend.Name(),
			CWD:             r.cfg.DefaultCWD,
		})
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{ResumedSessionID: record.ID}, nil
	case command.ControlSwitch:
		record, err := r.sessionManager.SwitchSession(ctx, parsed.ConversationID, parsed.WindowID, parsed.TargetSessionID)
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{
			SwitchedSessionID: record.ID,
			CurrentChecked:    true,
			CurrentSession: &SessionSummary{
				ID:               record.ID,
				Status:           record.Status,
				Backend:          record.Backend,
				BackendSessionID: record.BackendSessionID,
			},
		}, nil
	case command.ControlList:
		summaries, current, err := r.sessionManager.ListSessionSummariesByWindow(ctx, parsed.ConversationID, parsed.WindowID)
		if err != nil {
			return ControlFlowResult{}, err
		}
		result := ControlFlowResult{Sessions: summaries, CurrentChecked: true}
		result.CurrentSession = current
		return result, nil
	case command.ControlCancel:
		record, err := r.sessionManager.GetCurrentSessionByWindow(ctx, parsed.ConversationID, parsed.WindowID)
		if err != nil {
			return ControlFlowResult{}, err
		}
		if !record.IsRunning() {
			return ControlFlowResult{
				CancelledSessionID: record.ID,
				CancelNoop:         true,
			}, nil
		}
		cancelled, err := r.sessionManager.CancelExecution(ctx, r.backend, record.ID)
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{CancelledSessionID: cancelled.ID}, nil
	case command.ControlCurrent:
		record, err := r.sessionManager.GetCurrentSessionByWindow(ctx, parsed.ConversationID, parsed.WindowID)
		if err != nil {
			if errors.Is(err, session.ErrSessionNotFound) || errors.Is(err, session.ErrWindowBindingNotFound) {
				return ControlFlowResult{CurrentChecked: true}, nil
			}
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{
			CurrentChecked: true,
			CurrentSession: &SessionSummary{
				ID:               record.ID,
				Status:           record.Status,
				Backend:          record.Backend,
				BackendSessionID: record.BackendSessionID,
			},
		}, nil
	default:
		return ControlFlowResult{}, command.ErrInvalidControlCommand
	}
}
