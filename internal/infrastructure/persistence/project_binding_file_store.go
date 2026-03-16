package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	projectdomain "clawx/internal/domain/project"
)

type projectBindingSnapshot struct {
	Version  int                          `json:"version"`
	Bindings []projectdomain.RouteBinding `json:"bindings"`
}

type ProjectBindingFileStore struct {
	mu   sync.Mutex
	path string
}

func NewProjectBindingFileStore(path string) (*ProjectBindingFileStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("binding path is required")
	}
	return &ProjectBindingFileStore{path: path}, nil
}

func (s *ProjectBindingFileStore) GetByRouteKey(_ context.Context, routeKey string) (projectdomain.RouteBinding, error) {
	routeKey = strings.TrimSpace(routeKey)
	if routeKey == "" {
		return projectdomain.RouteBinding{}, projectdomain.ErrBindingNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return projectdomain.RouteBinding{}, err
	}
	for _, item := range snapshot.Bindings {
		if strings.TrimSpace(item.RouteKey) == routeKey {
			return item, nil
		}
	}
	return projectdomain.RouteBinding{}, projectdomain.ErrBindingNotFound
}

func (s *ProjectBindingFileStore) Upsert(_ context.Context, binding projectdomain.RouteBinding) error {
	if err := binding.Validate(); err != nil {
		return err
	}
	if binding.UpdatedAt.IsZero() {
		binding.UpdatedAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return err
	}

	updated := false
	for idx := range snapshot.Bindings {
		if strings.TrimSpace(snapshot.Bindings[idx].RouteKey) == binding.RouteKey {
			snapshot.Bindings[idx] = binding
			updated = true
			break
		}
	}
	if !updated {
		snapshot.Bindings = append(snapshot.Bindings, binding)
	}

	sort.Slice(snapshot.Bindings, func(i, j int) bool {
		return snapshot.Bindings[i].RouteKey < snapshot.Bindings[j].RouteKey
	})

	return s.writeSnapshotLocked(snapshot)
}

func (s *ProjectBindingFileStore) DeleteByRouteKey(_ context.Context, routeKey string) error {
	routeKey = strings.TrimSpace(routeKey)
	if routeKey == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return err
	}
	filtered := snapshot.Bindings[:0]
	for _, item := range snapshot.Bindings {
		if strings.TrimSpace(item.RouteKey) == routeKey {
			continue
		}
		filtered = append(filtered, item)
	}
	snapshot.Bindings = filtered
	return s.writeSnapshotLocked(snapshot)
}

func (s *ProjectBindingFileStore) List(_ context.Context) ([]projectdomain.RouteBinding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return nil, err
	}
	results := make([]projectdomain.RouteBinding, len(snapshot.Bindings))
	copy(results, snapshot.Bindings)
	return results, nil
}

func (s *ProjectBindingFileStore) readSnapshotLocked() (projectBindingSnapshot, error) {
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return projectBindingSnapshot{Version: 1}, nil
		}
		return projectBindingSnapshot{}, fmt.Errorf("open project bindings: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var snapshot projectBindingSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return projectBindingSnapshot{}, fmt.Errorf("parse project bindings: %w", err)
	}
	if decoder.More() {
		return projectBindingSnapshot{}, fmt.Errorf("parse project bindings: trailing data")
	}
	if snapshot.Version <= 0 {
		snapshot.Version = 1
	}
	if snapshot.Bindings == nil {
		snapshot.Bindings = make([]projectdomain.RouteBinding, 0)
	}
	return snapshot, nil
}

func (s *ProjectBindingFileStore) writeSnapshotLocked(snapshot projectBindingSnapshot) error {
	if snapshot.Version <= 0 {
		snapshot.Version = 1
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project bindings: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create project binding directory: %w", err)
		}
	}
	temp, err := os.CreateTemp(dir, ".project-bindings-*.tmp")
	if err != nil {
		return fmt.Errorf("create project binding temp file: %w", err)
	}
	tempName := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempName)
	}
	if _, err := temp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write project binding temp file: %w", err)
	}
	if err := temp.Chmod(0o644); err != nil {
		cleanup()
		return fmt.Errorf("chmod project binding temp file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync project binding temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("close project binding temp file: %w", err)
	}
	if err := os.Rename(tempName, s.path); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("replace project binding file: %w", err)
	}
	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	return nil
}
