package service

import (
	"context"
	"errors"
	"fmt"

	"synapsex/internal/application/command"
	"synapsex/internal/domain/execution"
	"synapsex/internal/domain/session"
)

type SessionFlowResult struct {
	Session   session.Record
	Execution execution.Result
}

func (r *Router) HandleSessionFlow(ctx context.Context, cmd command.SessionCommand) (SessionFlowResult, error) {
	cmd, err := cmd.Normalize()
	if err != nil {
		return SessionFlowResult{}, err
	}

	record, err := r.resolveSession(ctx, cmd)
	if err != nil {
		return SessionFlowResult{}, err
	}
	if cmd.Mode == command.ModeResume && cmd.Input == "" {
		return SessionFlowResult{Session: record}, nil
	}

	lockedSession, lockToken, err := r.sessionManager.AcquireExecution(ctx, record.ID)
	if err != nil {
		return SessionFlowResult{}, err
	}

	result, execErr := r.backend.Execute(ctx, execution.Request{
		SessionID:        lockedSession.ID,
		BackendSessionID: lockedSession.BackendSessionID,
		CWD:              lockedSession.CWD,
		Input:            cmd.Input,
		Timeout:          r.cfg.Timeout,
	})
	if execErr != nil {
		_ = r.sessionManager.MarkError(ctx, lockedSession.ID, lockToken, execErr.Error())
		return SessionFlowResult{}, fmt.Errorf("execute request: %w", execErr)
	}

	lockedSession.BindBackendSessionID(result.BackendSessionID)
	if saveErr := r.sessionManager.Save(ctx, lockedSession); saveErr != nil {
		_ = r.sessionManager.MarkError(ctx, lockedSession.ID, lockToken, saveErr.Error())
		return SessionFlowResult{}, saveErr
	}

	nextStatus := session.StatusIdle
	if result.State == execution.ResultFailed || result.State == execution.ResultTimeout {
		nextStatus = session.StatusError
	}

	updatedSession, err := r.sessionManager.FinishExecution(ctx, lockedSession.ID, lockToken, nextStatus)
	if err != nil {
		return SessionFlowResult{}, err
	}

	return SessionFlowResult{
		Session:   updatedSession,
		Execution: result,
	}, nil
}

func (r *Router) resolveSession(ctx context.Context, cmd command.SessionCommand) (session.Record, error) {
	switch cmd.Mode {
	case command.ModeNew:
		return r.sessionManager.CreateSession(ctx, cmd)
	case command.ModeResume:
		return r.sessionManager.ResumeSession(ctx, cmd)
	case command.ModeContinue:
		record, err := r.sessionManager.ContinueSession(ctx, cmd)
		if err == nil {
			return record, nil
		}
		if errors.Is(err, session.ErrSessionNotFound) {
			return r.sessionManager.CreateSession(ctx, command.SessionCommand{
				Mode:           command.ModeNew,
				ConversationID: cmd.ConversationID,
				Input:          cmd.Input,
				Backend:        cmd.Backend,
				CWD:            cmd.CWD,
			})
		}
		return session.Record{}, err
	default:
		return session.Record{}, command.ErrInvalidSessionCommand
	}
}
