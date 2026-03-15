package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	memorydomain "clawx/internal/domain/memory"
)

type DeniedCandidate struct {
	Layer  memorydomain.Layer
	Path   string
	Reason string
}

func BuildLayeredCandidates(scope memorydomain.MemoryScopeKey, guard *PathGuard, now time.Time) ([]memorydomain.MemoryLoadItem, []DeniedCandidate, error) {
	if err := scope.Validate(); err != nil {
		return nil, nil, err
	}
	if guard == nil {
		return nil, nil, memorydomain.ErrInvalidMemoryScope
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err := guard.EnsureAgentPrivateLayout(scope.AgentID); err != nil {
		return nil, nil, err
	}

	today := now.UTC().Format("2006-01-02") + ".md"
	ordered := []struct {
		layer memorydomain.Layer
		path  string
		kind  string
	}{
		{layer: memorydomain.LayerAgentPrivate, path: "IDENTITY.md", kind: "agent"},
		{layer: memorydomain.LayerAgentPrivate, path: "TOOLS.md", kind: "agent"},
		{layer: memorydomain.LayerAgentPrivate, path: "MEMORY.md", kind: "agent"},
		{layer: memorydomain.LayerAgentPrivate, path: filepath.ToSlash(filepath.Join("memory", today)), kind: "agent"},
		{layer: memorydomain.LayerProjectShare, path: "IDENTITY.md", kind: "project"},
		{layer: memorydomain.LayerProjectShare, path: "SOUL.md", kind: "project"},
		{layer: memorydomain.LayerProjectShare, path: "USER.md", kind: "project"},
		{layer: memorydomain.LayerProjectShare, path: "TOOLS.md", kind: "project"},
		{layer: memorydomain.LayerProjectShare, path: "AGENTS.md", kind: "project"},
		{layer: memorydomain.LayerProjectShare, path: "HEARTBEAT.md", kind: "project"},
		{layer: memorydomain.LayerProjectShare, path: filepath.ToSlash(filepath.Join("memory", today)), kind: "project"},
		{layer: memorydomain.LayerMainPrivate, path: "MEMORY.md", kind: "project"},
	}

	items := make([]memorydomain.MemoryLoadItem, 0, len(ordered))
	denied := make([]DeniedCandidate, 0)
	priority := 1
	for _, candidate := range ordered {
		var (
			absPath string
			err     error
		)
		switch candidate.kind {
		case "agent":
			absPath, err = guard.ResolveAgentPrivatePath(scope, candidate.path)
		default:
			absPath, err = guard.ResolveProjectSharedPath(candidate.path)
		}
		if err != nil {
			denied = append(denied, DeniedCandidate{Layer: candidate.layer, Path: candidate.path, Reason: mapDeniedReason(err)})
			continue
		}
		if _, err := os.Stat(absPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, nil, fmt.Errorf("stat memory candidate %q: %w", absPath, err)
		}
		if err := guard.ValidateCandidate(scope, candidate.layer, absPath); err != nil {
			denied = append(denied, DeniedCandidate{Layer: candidate.layer, Path: absPath, Reason: mapDeniedReason(err)})
			continue
		}
		items = append(items, memorydomain.MemoryLoadItem{
			Layer:    candidate.layer,
			Path:     absPath,
			Priority: priority,
		})
		priority++
	}
	return items, denied, nil
}

func mapDeniedReason(err error) string {
	switch {
	case errors.Is(err, ErrCrossAgentAccess):
		return "cross_agent_denied"
	case errors.Is(err, ErrCrossProjectAccess):
		return "cross_project_denied"
	case errors.Is(err, ErrPathEscape):
		return "path_escape_denied"
	default:
		value := strings.TrimSpace(err.Error())
		if value == "" {
			return "denied"
		}
		return value
	}
}
