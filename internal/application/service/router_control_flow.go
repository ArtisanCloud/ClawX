package service

import (
	"context"

	"synapsex/internal/application/command"
)

type ControlFlowResult struct {
	CreatedSessionID   string
	ResumedSessionID   string
	CancelledSessionID string
	Sessions           []SessionSummary
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
		return ControlFlowResult{Sessions: summaries}, nil
	case command.ControlCancel:
		record, err := r.sessionManager.GetLatestByConversation(ctx, parsed.ConversationID)
		if err != nil {
			return ControlFlowResult{}, err
		}
		cancelled, err := r.sessionManager.CancelExecution(ctx, r.backend, record.ID)
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{CancelledSessionID: cancelled.ID}, nil
	default:
		return ControlFlowResult{}, command.ErrInvalidControlCommand
	}
}
