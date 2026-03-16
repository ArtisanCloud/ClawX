package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	memorydomain "clawx/internal/domain/memory"
)

var (
	ErrPathEscape         = errors.New("memory path escapes project root")
	ErrCrossAgentAccess   = errors.New("cross-agent memory access denied")
	ErrCrossProjectAccess = errors.New("cross-project memory access denied")
)

type PathGuard struct {
	projectRoot string
}

func NewPathGuard(projectRoot string) (*PathGuard, error) {
	projectRoot = filepath.Clean(strings.TrimSpace(projectRoot))
	if projectRoot == "" || projectRoot == "." {
		return nil, memorydomain.ErrInvalidMemoryScope
	}
	return &PathGuard{projectRoot: projectRoot}, nil
}

func (g *PathGuard) ProjectRoot() string {
	if g == nil {
		return ""
	}
	return g.projectRoot
}

func (g *PathGuard) AgentRoot(agentID string) (string, error) {
	if g == nil {
		return "", memorydomain.ErrInvalidMemoryScope
	}
	agentID = sanitizePathSegment(agentID)
	if agentID == "" {
		return "", memorydomain.ErrInvalidMemoryScope
	}
	root := filepath.Join(g.projectRoot, ".agents", agentID)
	if err := ensureWithinRoot(g.projectRoot, root); err != nil {
		return "", err
	}
	return root, nil
}

func (g *PathGuard) ResolveAgentPrivatePath(scope memorydomain.MemoryScopeKey, relativePath string) (string, error) {
	if err := scope.Validate(); err != nil {
		return "", err
	}
	agentRoot, err := g.AgentRoot(scope.AgentID)
	if err != nil {
		return "", err
	}
	target, err := resolveRelative(agentRoot, relativePath)
	if err != nil {
		return "", err
	}
	if err := ensureWithinRoot(g.projectRoot, target); err != nil {
		return "", err
	}
	return target, nil
}

func (g *PathGuard) ResolveProjectSharedPath(relativePath string) (string, error) {
	if g == nil {
		return "", memorydomain.ErrInvalidMemoryScope
	}
	target, err := resolveRelative(g.projectRoot, relativePath)
	if err != nil {
		return "", err
	}
	if err := ensureWithinRoot(g.projectRoot, target); err != nil {
		return "", err
	}
	return target, nil
}

func (g *PathGuard) EnsureAgentPrivateLayout(agentID string) error {
	agentRoot, err := g.AgentRoot(agentID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(agentRoot, "memory"), 0o755); err != nil {
		return fmt.Errorf("create agent memory directory: %w", err)
	}
	agentID = sanitizePathSegment(agentID)
	templates := map[string]string{
		"IDENTITY.md": fmt.Sprintf("# IDENTITY\n\n- agent: %s\n", agentID),
		"TOOLS.md":    "# TOOLS\n\n- scope: agent-private\n",
		"MEMORY.md":   "# MEMORY\n\n- private notes\n",
	}
	for relative, content := range templates {
		path := filepath.Join(agentRoot, relative)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat agent memory template %q: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write agent memory template %q: %w", path, err)
		}
	}
	readme := filepath.Join(agentRoot, "memory", "README.md")
	if _, err := os.Stat(readme); os.IsNotExist(err) {
		if err := os.WriteFile(readme, []byte("# memory\n\nDaily agent-private notes.\n"), 0o644); err != nil {
			return fmt.Errorf("write agent memory readme: %w", err)
		}
	}
	return nil
}

func (g *PathGuard) ValidateCandidate(scope memorydomain.MemoryScopeKey, layer memorydomain.Layer, path string) error {
	if g == nil {
		return memorydomain.ErrInvalidMemoryScope
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return memorydomain.ErrInvalidMemoryScope
	}

	if err := ensureWithinRoot(g.projectRoot, path); err != nil {
		if errors.Is(err, ErrPathEscape) {
			return ErrCrossProjectAccess
		}
		return err
	}

	if layer == memorydomain.LayerAgentPrivate {
		agentRoot, err := g.AgentRoot(scope.AgentID)
		if err != nil {
			return err
		}
		if err := ensureWithinRoot(agentRoot, path); err != nil {
			return ErrCrossAgentAccess
		}
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("resolve memory path symlink: %w", err)
	}
	resolved = filepath.Clean(strings.TrimSpace(resolved))
	if err := ensureWithinRoot(g.projectRoot, resolved); err != nil {
		return ErrCrossProjectAccess
	}
	if layer == memorydomain.LayerAgentPrivate {
		agentRoot, err := g.AgentRoot(scope.AgentID)
		if err != nil {
			return err
		}
		if err := ensureWithinRoot(agentRoot, resolved); err != nil {
			return ErrCrossAgentAccess
		}
	}
	return nil
}

func resolveRelative(baseRoot, relativePath string) (string, error) {
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" {
		return "", memorydomain.ErrInvalidMemoryScope
	}
	if filepath.IsAbs(relativePath) {
		return "", ErrPathEscape
	}
	cleanRelative := filepath.Clean(relativePath)
	if cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return "", ErrPathEscape
	}
	return filepath.Join(baseRoot, cleanRelative), nil
}

func ensureWithinRoot(root, target string) error {
	root = filepath.Clean(strings.TrimSpace(root))
	target = filepath.Clean(strings.TrimSpace(target))
	if root == "" || target == "" {
		return memorydomain.ErrInvalidMemoryScope
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return memorydomain.ErrInvalidMemoryScope
	}
	rel = filepath.Clean(rel)
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ErrPathEscape
	}
	return nil
}

func sanitizePathSegment(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return ""
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
	return strings.Trim(b.String(), "-_")
}
