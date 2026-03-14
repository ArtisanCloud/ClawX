package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	memorydomain "clawx/internal/domain/memory"
)

type MemoryTemplateFileStore struct {
	mu            sync.Mutex
	workspaceRoot string
}

func NewMemoryTemplateFileStore(workspaceRoot string) (*MemoryTemplateFileStore, error) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return nil, fmt.Errorf("workspace root is required")
	}
	return &MemoryTemplateFileStore{workspaceRoot: filepath.Clean(workspaceRoot)}, nil
}

func (s *MemoryTemplateFileStore) GetManifest(_ context.Context, projectID string) (memorydomain.TemplateManifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	manifestPath, err := s.manifestPath(projectID)
	if err != nil {
		return memorydomain.TemplateManifest{}, err
	}
	file, err := os.Open(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return memorydomain.TemplateManifest{}, memorydomain.ErrMemoryNotFound
		}
		return memorydomain.TemplateManifest{}, fmt.Errorf("open memory manifest: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest memorydomain.TemplateManifest
	if err := decoder.Decode(&manifest); err != nil {
		return memorydomain.TemplateManifest{}, fmt.Errorf("parse memory manifest: %w", err)
	}
	if decoder.More() {
		return memorydomain.TemplateManifest{}, fmt.Errorf("parse memory manifest: trailing data")
	}
	return manifest, nil
}

func (s *MemoryTemplateFileStore) SaveManifest(_ context.Context, projectID string, manifest memorydomain.TemplateManifest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	manifestPath, err := s.manifestPath(projectID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(manifest.TemplateVersion) == "" {
		manifest.TemplateVersion = "v1"
	}
	if manifest.UpdatedAt.IsZero() {
		manifest.UpdatedAt = time.Now().UTC()
	}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal memory manifest: %w", err)
	}
	payload = append(payload, '\n')
	return writeMemoryAtomic(manifestPath, payload, ".memory-manifest-*.tmp")
}

func (s *MemoryTemplateFileStore) ReadFile(_ context.Context, projectID, relativePath string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	absPath, err := s.resolveProjectRelativePath(projectID, relativePath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, memorydomain.ErrMemoryNotFound
		}
		return nil, fmt.Errorf("read memory template file: %w", err)
	}
	return data, nil
}

func (s *MemoryTemplateFileStore) WriteFile(_ context.Context, projectID, relativePath string, body []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	absPath, err := s.resolveProjectRelativePath(projectID, relativePath)
	if err != nil {
		return err
	}
	return writeMemoryAtomic(absPath, body, ".memory-template-*.tmp")
}

func (s *MemoryTemplateFileStore) manifestPath(projectID string) (string, error) {
	projectRoot, err := s.projectRoot(projectID)
	if err != nil {
		return "", err
	}
	return filepath.Join(projectRoot, ".memory", "manifest.json"), nil
}

func (s *MemoryTemplateFileStore) resolveProjectRelativePath(projectID, relativePath string) (string, error) {
	projectRoot, err := s.projectRoot(projectID)
	if err != nil {
		return "", err
	}
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" {
		return "", memorydomain.ErrInvalidMemoryScope
	}
	if filepath.IsAbs(relativePath) {
		return "", memorydomain.ErrInvalidMemoryScope
	}
	cleanRelative := filepath.Clean(relativePath)
	if cleanRelative == "." || cleanRelative == "" || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return "", memorydomain.ErrInvalidMemoryScope
	}
	absPath := filepath.Join(projectRoot, cleanRelative)
	if err := ensurePathWithinRoot(projectRoot, absPath); err != nil {
		return "", err
	}
	return absPath, nil
}

func (s *MemoryTemplateFileStore) projectRoot(projectID string) (string, error) {
	projectID = normalizeMemoryPathSegment(projectID)
	if projectID == "" {
		return "", memorydomain.ErrInvalidMemoryScope
	}
	root := filepath.Join(s.workspaceRoot, projectID)
	if err := ensurePathWithinRoot(s.workspaceRoot, root); err != nil {
		return "", err
	}
	return root, nil
}

func writeMemoryAtomic(path string, payload []byte, tempPattern string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create memory directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return fmt.Errorf("create memory temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.Write(payload); err != nil {
		cleanup()
		return fmt.Errorf("write memory temp file: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		cleanup()
		return fmt.Errorf("chmod memory temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync memory temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close memory temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("replace memory file: %w", err)
	}
	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	return nil
}

func normalizeMemoryPathSegment(raw string) string {
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
		}
	}
	return strings.Trim(b.String(), "-_")
}

func ensurePathWithinRoot(root, target string) error {
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
		return memorydomain.ErrInvalidMemoryScope
	}
	return nil
}
