package memory

import (
	"path/filepath"
	"time"

	memorydomain "clawx/internal/domain/memory"
)

type WriteScope string

const (
	WriteScopeAgentPrivate WriteScope = "agent_private"
	WriteScopeProjectShare WriteScope = "project_shared"
)

func ResolveNoteWriteScope(explicitShared bool) WriteScope {
	if explicitShared {
		return WriteScopeProjectShare
	}
	return WriteScopeAgentPrivate
}

func ResolveDailyJournalWritePath(scope memorydomain.MemoryScopeKey, guard *PathGuard, day time.Time, writeScope WriteScope) (string, error) {
	if day.IsZero() {
		day = time.Now().UTC()
	}
	name := day.UTC().Format("2006-01-02") + ".md"
	switch writeScope {
	case WriteScopeProjectShare:
		return guard.ResolveProjectSharedPath(filepath.ToSlash(filepath.Join("memory", name)))
	default:
		return guard.ResolveAgentPrivatePath(scope, filepath.ToSlash(filepath.Join("memory", name)))
	}
}
