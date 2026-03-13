package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"clawx/internal/application/command"
	"clawx/internal/domain/session"
)

func (m *SessionManager) CreateSession(ctx context.Context, cmd command.SessionCommand) (session.Record, error) {
	cmd.ConversationID = strings.TrimSpace(cmd.ConversationID)
	cmd.WindowID = strings.TrimSpace(cmd.WindowID)
	cmd.Backend = strings.TrimSpace(cmd.Backend)
	cmd.CWD = strings.TrimSpace(cmd.CWD)
	if cmd.ConversationID == "" {
		return session.Record{}, command.ErrInvalidSessionCommand
	}
	if cmd.WindowID == "" {
		cmd.WindowID = "compat:" + cmd.ConversationID
	}
	cmd.ProjectID = normalizeSessionProjectID(cmd.ProjectID)
	cmd.WindowID = buildSessionScopeWindowID(cmd.WindowID, cmd.ProjectID)
	if cmd.Backend == "" {
		cmd.Backend = "primary"
	}
	if cmd.CWD == "" {
		cmd.CWD = "."
	}

	record := session.Record{
		ID:             m.nextSessionID(cmd.ProjectID),
		WindowID:       cmd.WindowID,
		Backend:        cmd.Backend,
		ConversationID: cmd.ConversationID,
		CWD:            cmd.CWD,
		Status:         session.StatusIdle,
	}
	record.Touch(m.clock())

	if err := record.Validate(); err != nil {
		return session.Record{}, err
	}
	if err := m.repository.Create(ctx, record); err != nil {
		return session.Record{}, fmt.Errorf("create session: %w", err)
	}
	if _, err := m.BindWindowToSession(ctx, cmd.WindowID, cmd.ConversationID, record.ID); err != nil {
		return session.Record{}, err
	}
	return record, nil
}

func (m *SessionManager) nextSessionID(projectID string) string {
	now := m.clock()
	return fmt.Sprintf("sess-%s-%d", normalizeSessionProjectID(projectID), now.UTC().UnixNano())
}

func trimSessionID(value string) string {
	return strings.TrimSpace(value)
}

func normalizeSessionProjectID(projectID string) string {
	value := strings.ToLower(strings.TrimSpace(projectID))
	if value == "" {
		return "main"
	}

	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	normalized := strings.Trim(b.String(), "-_")
	if normalized == "" {
		return "main"
	}
	return normalized
}

func buildSessionScopeWindowID(windowID, projectID string) string {
	windowID = strings.TrimSpace(windowID)
	if windowID == "" {
		return ""
	}
	if strings.Contains(windowID, "|project:") {
		return windowID
	}
	return windowID + "|project:" + normalizeSessionProjectID(projectID)
}

func projectIDFromScopedWindow(windowID string) string {
	windowID = strings.TrimSpace(windowID)
	if windowID == "" {
		return "main"
	}
	idx := strings.LastIndex(windowID, "|project:")
	if idx < 0 {
		return "main"
	}
	return normalizeSessionProjectID(windowID[idx+len("|project:"):])
}

func projectIDFromSessionID(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "main"
	}
	if !strings.HasPrefix(sessionID, "sess-") {
		return "main"
	}
	tail := strings.TrimPrefix(sessionID, "sess-")
	lastDash := strings.LastIndexByte(tail, '-')
	if lastDash <= 0 {
		return "main"
	}
	rawProject := tail[:lastDash]
	rawTimestamp := tail[lastDash+1:]
	if rawProject == "" {
		return "main"
	}
	if _, err := strconv.ParseInt(rawTimestamp, 10, 64); err != nil {
		return "main"
	}
	return normalizeSessionProjectID(rawProject)
}

func sessionBelongsToProject(sessionID, projectID string) bool {
	return projectIDFromSessionID(sessionID) == normalizeSessionProjectID(projectID)
}
