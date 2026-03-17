package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"clawx/internal/application/command"
	memoryapp "clawx/internal/application/memory"
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

	memoryCommand, err := command.ParseMemoryControlCommand(rawCommand)
	if err == nil {
		return r.handleMemoryControlCommand(ctx, memoryCommand, conversationID, window, routeKey)
	}
	if !errors.Is(err, command.ErrNotMemoryControlCommand) {
		return ControlFlowResult{}, err
	}

	serviceCommand, err := command.ParseServiceControlCommand(rawCommand)
	if err == nil {
		return r.handleServiceControlCommand(ctx, serviceCommand, conversationID, window, routeKey)
	}
	if !errors.Is(err, command.ErrNotServiceControlCommand) {
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
	case command.ProjectControlBind:
		binding, err := r.projectControl.BindRoute(ctx, cmd.RouteKey, cmd.ProjectID, "chat-control")
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{
			Message: fmt.Sprintf("已绑定路由: %s -> %s", binding.RouteKey, binding.ProjectID),
		}, nil
	case command.ProjectControlUnbind:
		if err := r.projectControl.UnbindRoute(ctx, cmd.RouteKey); err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{
			Message: fmt.Sprintf("已解绑路由: %s", strings.TrimSpace(cmd.RouteKey)),
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
	case command.ProjectControlAudit:
		report, err := r.projectControl.AuditProjects(ctx)
		if err != nil {
			return ControlFlowResult{}, err
		}
		lines := []string{
			fmt.Sprintf("项目审计: projects=%d active=%d broken=%d bindings=%d broken_bindings=%d", report.TotalProjects, report.ActiveProjects, report.BrokenProjects, report.TotalBindings, report.BrokenBindings),
		}
		for _, issue := range report.BindingIssues {
			lines = append(lines, fmt.Sprintf("- binding_issue route=%s project=%s reason=%s", issue.RouteKey, issue.ProjectID, issue.Reason))
		}
		return ControlFlowResult{Message: strings.Join(lines, "\n")}, nil
	case command.ProjectControlDelete:
		record, err := r.projectControl.DeleteProject(ctx, cmd.ProjectID, cmd.Force)
		if err != nil {
			return ControlFlowResult{}, err
		}
		if cmd.Force {
			return ControlFlowResult{Message: fmt.Sprintf("已删除项目: %s (force)", record.ID)}, nil
		}
		return ControlFlowResult{Message: fmt.Sprintf("已删除项目: %s", record.ID)}, nil
	case command.ProjectControlRepair:
		record, err := r.projectControl.RepairProject(ctx, cmd.ProjectID)
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{
			Message: fmt.Sprintf("已修复项目: %s [%s] %s", record.ID, normalizeProjectStatus(record.Status), record.WorkspacePath),
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

func (r *Router) handleMemoryControlCommand(ctx context.Context, cmd command.MemoryControlCommand, conversationID, windowID, routeKey string) (ControlFlowResult, error) {
	if r.memoryControl == nil {
		return ControlFlowResult{}, command.ErrInvalidControlCommand
	}

	projectID, _, err := r.resolveControlProject(ctx, routeKey)
	if err != nil {
		return ControlFlowResult{}, err
	}
	scopedWindowID := buildSessionScopeWindowID(windowID, projectID)
	agentID := r.resolveControlAgentID(ctx, conversationID, scopedWindowID)

	switch cmd.Kind {
	case command.MemoryControlNote:
		result, err := r.memoryControl.Note(ctx, memoryapp.NoteInput{
			RouteKey:        routeKey,
			ProjectID:       projectID,
			AgentID:         agentID,
			Text:            cmd.Text,
			Shared:          cmd.Shared,
			UserID:          "",
			IsDirectMessage: false,
			RequestedBy:     "chat-control",
		})
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{
			Message: fmt.Sprintf("记忆写入成功: scope=%s at=%s", result.Scope, result.Timestamp.Format(time.RFC3339)),
		}, nil
	case command.MemoryControlDigest:
		result, err := r.memoryControl.Digest(ctx, memoryapp.DigestInput{
			RouteKey:        routeKey,
			ProjectID:       projectID,
			AgentID:         agentID,
			UserID:          "",
			IsDirectMessage: false,
			RequestedBy:     "chat-control",
		})
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{
			Message: fmt.Sprintf(
				"记忆汇总完成: job=%s status=%s output=%s auto_digest=%t",
				result.JobID,
				result.Status,
				result.OutputFile,
				result.AutoDigestEnabled,
			),
		}, nil
	case command.MemoryControlAudit:
		result, err := r.memoryControl.Audit(ctx, memoryapp.AuditInput{
			RouteKey:        routeKey,
			ProjectID:       projectID,
			AgentID:         agentID,
			UserID:          "",
			IsDirectMessage: false,
			Limit:           50,
		})
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{
			Message: fmt.Sprintf(
				"记忆审计: template_version=%s required=%d missing=%d acl_denied=%d budget_skipped=%d recent_errors=%q",
				result.TemplateVersion,
				result.RequiredFiles,
				result.MissingRequired,
				result.ACLDeniedCount,
				result.BudgetSkippedCount,
				result.RecentErrors,
			),
		}, nil
	default:
		return ControlFlowResult{}, command.ErrInvalidControlCommand
	}
}

func (r *Router) handleServiceControlCommand(ctx context.Context, cmd command.ServiceControlCommand, conversationID, windowID, routeKey string) (ControlFlowResult, error) {
	if r.serviceControl == nil {
		return ControlFlowResult{}, command.ErrInvalidControlCommand
	}

	projectID, _, err := r.resolveControlProject(ctx, routeKey)
	if err != nil {
		return ControlFlowResult{}, err
	}
	scopedWindowID := buildSessionScopeWindowID(windowID, projectID)
	agentID := r.resolveControlAgentID(ctx, conversationID, scopedWindowID)
	workspace := filepath.Join(strings.TrimSpace(r.cfg.Projects.WorkspaceRoot), projectID, ".agents", agentID, "workspace")

	switch cmd.Kind {
	case command.ServiceControlStart:
		result, err := r.serviceControl.Start(ctx, ServiceStartInput{
			ProjectID: projectID,
			AgentID:   agentID,
			RouteKey:  routeKey,
			Name:      cmd.Name,
			Command:   cmd.Command,
			CWD:       workspace,
		})
		if err != nil {
			return ControlFlowResult{}, err
		}
		return ControlFlowResult{
			Message: fmt.Sprintf("服务已启动: %s pid=%d cwd=%s log=%s", result.Name, result.PID, workspace, result.LogPath),
		}, nil
	case command.ServiceControlStop:
		result, err := r.serviceControl.Stop(ctx, ServiceStopInput{
			ProjectID: projectID,
			AgentID:   agentID,
			Name:      cmd.Name,
		})
		if err != nil {
			return ControlFlowResult{}, err
		}
		if !result.Stopped {
			return ControlFlowResult{Message: fmt.Sprintf("服务未运行: %s", result.Name)}, nil
		}
		return ControlFlowResult{Message: fmt.Sprintf("服务已停止: %s", result.Name)}, nil
	case command.ServiceControlStatus:
		result, err := r.serviceControl.Status(ctx, ServiceStatusInput{
			ProjectID: projectID,
			AgentID:   agentID,
			Name:      cmd.Name,
		})
		if err != nil {
			return ControlFlowResult{}, err
		}
		if len(result.Services) == 0 {
			return ControlFlowResult{Message: "当前没有受管服务"}, nil
		}
		sort.Slice(result.Services, func(i, j int) bool {
			return result.Services[i].Name < result.Services[j].Name
		})
		lines := []string{"服务状态:"}
		for _, item := range result.Services {
			lines = append(lines, fmt.Sprintf("- %s [%s] pid=%d cwd=%s cmd=%s", item.Name, item.Status, item.PID, item.CWD, strings.Join(item.Command, " ")))
		}
		return ControlFlowResult{Message: strings.Join(lines, "\n")}, nil
	case command.ServiceControlLogs:
		result, err := r.serviceControl.Logs(ctx, ServiceLogsInput{
			ProjectID: projectID,
			AgentID:   agentID,
			Name:      cmd.Name,
			Tail:      cmd.Tail,
		})
		if err != nil {
			return ControlFlowResult{}, err
		}
		content := strings.TrimSpace(result.Content)
		if content == "" {
			return ControlFlowResult{Message: fmt.Sprintf("日志为空: %s (%s)", result.Name, result.LogPath)}, nil
		}
		return ControlFlowResult{
			Message: fmt.Sprintf("服务日志: %s (%s)\n%s", result.Name, result.LogPath, content),
		}, nil
	default:
		return ControlFlowResult{}, command.ErrInvalidControlCommand
	}
}

func (r *Router) resolveControlAgentID(ctx context.Context, conversationID, scopedWindowID string) string {
	if r.sessionManager != nil {
		record, err := r.sessionManager.GetCurrentSessionByWindow(ctx, strings.TrimSpace(conversationID), strings.TrimSpace(scopedWindowID))
		if err == nil && strings.TrimSpace(record.AgentID) != "" {
			return strings.TrimSpace(record.AgentID)
		}
	}
	if strings.TrimSpace(r.cfg.DefaultAgentID) != "" {
		return strings.TrimSpace(r.cfg.DefaultAgentID)
	}
	if strings.TrimSpace(r.backend.Name()) != "" {
		return strings.TrimSpace(r.backend.Name())
	}
	return "main"
}
