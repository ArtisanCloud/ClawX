package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"clawx/internal/application/command"
	projectdomain "clawx/internal/domain/project"
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
	Message            string
}

func (r *Router) HandleControlCommand(ctx context.Context, rawCommand, conversationID string, windowID ...string) (ControlFlowResult, error) {
	window := ""
	if len(windowID) > 0 {
		window = strings.TrimSpace(windowID[0])
	}
	routeKey := ""
	if len(windowID) > 1 {
		routeKey = strings.TrimSpace(windowID[1])
	}
	if routeKey == "" {
		routeKey = window
	}
	if routeKey == "" {
		routeKey = "compat:" + strings.TrimSpace(conversationID)
	}

	projectCommand, err := command.ParseProjectControlCommand(rawCommand)
	if err == nil {
		return r.handleProjectControlCommand(ctx, projectCommand, routeKey)
	}
	if !errors.Is(err, command.ErrNotProjectControlCommand) {
		return ControlFlowResult{}, err
	}

	parsed, err := command.ParseControlCommand(rawCommand, conversationID, windowID...)
	if err != nil {
		return ControlFlowResult{}, err
	}
	activeProjectID, _, err := r.resolveControlProject(ctx, routeKey)
	if err != nil {
		return ControlFlowResult{}, err
	}
	scopedWindowID := buildSessionScopeWindowID(parsed.WindowID, activeProjectID)

	switch parsed.Kind {
	case command.ControlNew:
		record, err := r.sessionManager.CreateSession(ctx, command.SessionCommand{
			Mode:           command.ModeNew,
			ConversationID: parsed.ConversationID,
			WindowID:       scopedWindowID,
			ProjectID:      activeProjectID,
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
			WindowID:        scopedWindowID,
			ProjectID:       activeProjectID,
			ResumeSessionID: parsed.TargetSessionID,
			Backend:         r.backend.Name(),
			CWD:             r.cfg.DefaultCWD,
		})
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{ResumedSessionID: record.ID}, nil
	case command.ControlSwitch:
		record, err := r.sessionManager.SwitchSession(ctx, parsed.ConversationID, scopedWindowID, parsed.TargetSessionID)
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
		summaries, current, err := r.sessionManager.ListSessionSummariesByWindow(ctx, parsed.ConversationID, scopedWindowID)
		if err != nil {
			return ControlFlowResult{}, err
		}
		result := ControlFlowResult{Sessions: summaries, CurrentChecked: true}
		result.CurrentSession = current
		return result, nil
	case command.ControlCancel:
		record, err := r.sessionManager.GetCurrentSessionByWindow(ctx, parsed.ConversationID, scopedWindowID)
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
		record, err := r.sessionManager.GetCurrentSessionByWindow(ctx, parsed.ConversationID, scopedWindowID)
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

func (r *Router) resolveControlProject(ctx context.Context, routeKey string) (string, string, error) {
	projectID := strings.TrimSpace(r.cfg.Projects.DefaultProjectID)
	if projectID == "" {
		projectID = "main"
	}
	mode := "fallback"
	if r.project != nil {
		resolvedID, resolvedMode, err := r.project.ResolveProject(ctx, routeKey)
		if err != nil {
			return "", "", err
		}
		if strings.TrimSpace(resolvedID) != "" {
			projectID = strings.TrimSpace(resolvedID)
		}
		if strings.TrimSpace(resolvedMode) != "" {
			mode = strings.TrimSpace(resolvedMode)
		}
	}
	return normalizeSessionProjectID(projectID), mode, nil
}

func (r *Router) handleProjectControlCommand(ctx context.Context, cmd command.ProjectControlCommand, routeKey string) (ControlFlowResult, error) {
	if r.projectControl == nil {
		return ControlFlowResult{}, command.ErrInvalidControlCommand
	}

	switch cmd.Kind {
	case command.ProjectControlCreate:
		record, err := r.projectControl.CreateProject(ctx, cmd.ProjectID, cmd.ProjectName, "")
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{
			Message: fmt.Sprintf("已创建项目: %s (%s)", record.ID, record.WorkspacePath),
		}, nil
	case command.ProjectControlList:
		projects, err := r.projectControl.ListProjects(ctx)
		if err != nil {
			return ControlFlowResult{}, err
		}
		if len(projects) == 0 {
			return ControlFlowResult{Message: "当前没有可用项目"}, nil
		}
		lines := make([]string, 0, len(projects)+1)
		lines = append(lines, "项目列表:")
		for _, item := range projects {
			lines = append(lines, fmt.Sprintf("- %s [%s] %s", item.ID, normalizeProjectStatus(item.Status), item.WorkspacePath))
		}
		return ControlFlowResult{Message: strings.Join(lines, "\n")}, nil
	case command.ProjectControlUse:
		binding, err := r.projectControl.UseProject(ctx, routeKey, cmd.ProjectID, "chat-control")
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{
			Message: fmt.Sprintf("已切换当前项目: %s (route=%s)", binding.ProjectID, binding.RouteKey),
		}, nil
	case command.ProjectControlCurrent:
		projectID, mode, err := r.projectControl.ResolveProject(ctx, routeKey)
		if err != nil {
			return ControlFlowResult{}, err
		}
		record, err := r.projectControl.GetProject(ctx, projectID)
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{
			Message: fmt.Sprintf("当前项目: %s [%s] %s (mode=%s)", record.ID, normalizeProjectStatus(record.Status), record.WorkspacePath, mode),
		}, nil
	case command.ProjectControlSuggest:
		fromProjectID, _, err := r.resolveControlProject(ctx, routeKey)
		if err != nil {
			return ControlFlowResult{}, err
		}
		proposal, err := r.projectControl.SuggestProjectSwitch(
			ctx,
			routeKey,
			fromProjectID,
			cmd.ProjectID,
			cmd.Reason,
			cmd.Confidence,
			"intent-router",
		)
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{
			Message: fmt.Sprintf(
				"建议切换项目: %s -> %s\n原因: %s\n置信度: %.2f\n确认命令: /project confirm %s",
				proposal.FromProjectID,
				proposal.ToProjectID,
				proposal.Reason,
				proposal.Confidence,
				proposal.ID,
			),
		}, nil
	case command.ProjectControlConfirm:
		binding, err := r.projectControl.ConfirmProjectSwitch(ctx, cmd.ProposalID, "chat-control")
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{
			Message: fmt.Sprintf("已确认并切换项目: %s (route=%s)", binding.ProjectID, binding.RouteKey),
		}, nil
	default:
		return ControlFlowResult{}, command.ErrInvalidControlCommand
	}
}

func normalizeProjectStatus(status projectdomain.Status) string {
	value := strings.TrimSpace(strings.ToLower(string(status)))
	if value == "" {
		return string(projectdomain.StatusActive)
	}
	return value
}
