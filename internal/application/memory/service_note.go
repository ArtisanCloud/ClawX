package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	memorydomain "clawx/internal/domain/memory"
)

func (s *CommandService) Note(ctx context.Context, input NoteInput) (NoteResult, error) {
	text := strings.TrimSpace(input.Text)
	if text == "" {
		return NoteResult{}, newCommandError("invalid_argument", "note text is empty")
	}

	scoped, err := s.resolveScopeContext(ctx, scopeInput{
		RouteKey:        input.RouteKey,
		ProjectID:       input.ProjectID,
		AgentID:         input.AgentID,
		UserID:          input.UserID,
		IsDirectMessage: input.IsDirectMessage,
	})
	if err != nil {
		return NoteResult{}, err
	}

	writeScope := ResolveNoteWriteScope(input.Shared)
	scopeLabel := "agent-private"
	if writeScope == WriteScopeProjectShare {
		scopeLabel = "project-shared"
		// Shared write is only allowed in main owner sessions.
		if !scoped.classification.AllowMainPrivate || scoped.classification.Degraded || !scoped.classification.OwnerMatched {
			s.appendAuditRecord(ctx, memorydomain.AuditRecord{
				Timestamp:   s.now().UTC(),
				ScopeKey:    scoped.scope,
				DeniedFiles: []string{"memory note --shared|rejected_acl"},
				ACLMode:     scoped.profile.ACLMode,
				Degraded:    true,
			})
			return NoteResult{}, newCommandError("rejected_acl", "shared note write is not allowed in current session")
		}
	}

	path, err := ResolveDailyJournalWritePath(scoped.scope, scoped.guard, s.now().UTC(), writeScope)
	if err != nil {
		return NoteResult{}, err
	}
	author := strings.TrimSpace(input.RequestedBy)
	if author == "" {
		author = "user"
	}
	now := s.now().UTC()
	if err := appendMarkdownNote(path, now, author, text); err != nil {
		s.appendAuditRecord(ctx, memorydomain.AuditRecord{
			Timestamp:  now,
			ScopeKey:   scoped.scope,
			ErrorFiles: []string{fmt.Sprintf("note_write_failed:%s", path)},
			ACLMode:    scoped.profile.ACLMode,
			Degraded:   true,
		})
		return NoteResult{}, err
	}

	s.appendAuditRecord(ctx, memorydomain.AuditRecord{
		Timestamp:   now,
		ScopeKey:    scoped.scope,
		LoadedFiles: []string{path},
		ACLMode:     scoped.profile.ACLMode,
		Degraded:    scoped.profile.ACLMode == memorydomain.ACLModeDegraded,
	})

	return NoteResult{
		Scope:     scopeLabel,
		Path:      path,
		Timestamp: now,
	}, nil
}

func appendMarkdownNote(path string, now time.Time, author, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create memory note directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open memory note file: %w", err)
	}
	defer file.Close()

	line := fmt.Sprintf("- %s [%s] %s\n", now.Format(time.RFC3339), strings.TrimSpace(author), strings.TrimSpace(text))
	if _, err := file.WriteString(line); err != nil {
		return fmt.Errorf("append memory note: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync memory note: %w", err)
	}
	return nil
}
