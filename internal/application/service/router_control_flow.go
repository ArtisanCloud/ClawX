package service

import (
	"context"
	"errors"

	"synapsex/internal/application/command"
	"synapsex/internal/domain/session"
)

type ControlFlowResult struct {
	CreatedSessionID   string
	ResumedSessionID   string
	CancelledSessionID string
	CancelNoop         bool
	Sessions           []SessionSummary
	CurrentSession     *SessionSummary
	CurrentChecked     bool
}

func (r *Router) HandleControlCommand(ctx context.Context, rawCommand, conversationID string) (ControlFlowResult, error) {
	parsed, err := command.ParseControlCommand(rawCommand, conversationID)
	if err != nil {
		return ControlFlowResult{}, err
	}

	switch parsed.Kind {
	case command.ControlNew:
		record, err := r.sessionManager.CreateSession(ctx, command.SessionCommand{
			Mode:           command.ModeNew,
			ConversationID: parsed.ConversationID,
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
			ResumeSessionID: parsed.TargetSessionID,
			Backend:         r.backend.Name(),
			CWD:             r.cfg.DefaultCWD,
		})
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{ResumedSessionID: record.ID}, nil
	case command.ControlList:
		summaries, err := r.sessionManager.ListSessionSummaries(ctx, parsed.ConversationID)
		if err != nil {
			return ControlFlowResult{}, err
		}
		result := ControlFlowResult{Sessions: summaries, CurrentChecked: true}
		if len(summaries) > 0 {
			current := summaries[0]
			result.CurrentSession = &current
		}
		return result, nil
	case command.ControlCancel:
		record, err := r.sessionManager.GetLatestByConversation(ctx, parsed.ConversationID)
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
		record, err := r.sessionManager.GetLatestByConversation(ctx, parsed.ConversationID)
		if err != nil {
			if errors.Is(err, session.ErrSessionNotFound) {
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
