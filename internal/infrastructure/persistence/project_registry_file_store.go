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

type projectRegistrySnapshot struct {
	Version          int                    `json:"version"`
	DefaultProjectID string                 `json:"defaultProjectID"`
	Projects         []projectdomain.Record `json:"projects"`
	UpdatedAt        time.Time              `json:"updatedAt"`
}

type ProjectRegistryFileStore struct {
	mu   sync.Mutex
	path string
}

func NewProjectRegistryFileStore(path string) (*ProjectRegistryFileStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("registry path is required")
	}
	return &ProjectRegistryFileStore{path: path}, nil
}

func (s *ProjectRegistryFileStore) Get(_ context.Context) (projectdomain.Registry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return projectdomain.Registry{}, err
	}
	return registryFromSnapshot(snapshot), nil
}

func (s *ProjectRegistryFileStore) Save(_ context.Context, registry projectdomain.Registry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := snapshotFromRegistry(registry)
	if snapshot.Version <= 0 {
		snapshot.Version = 1
	}
	return s.writeSnapshotLocked(snapshot)
}

func (s *ProjectRegistryFileStore) readSnapshotLocked() (projectRegistrySnapshot, error) {
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return projectRegistrySnapshot{Version: 1}, nil
		}
		return projectRegistrySnapshot{}, fmt.Errorf("open project registry: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var snapshot projectRegistrySnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return projectRegistrySnapshot{}, fmt.Errorf("parse project registry: %w", err)
	}
	if decoder.More() {
		return projectRegistrySnapshot{}, fmt.Errorf("parse project registry: trailing data")
	}
	if snapshot.Version <= 0 {
		snapshot.Version = 1
	}
	return snapshot, nil
}

func (s *ProjectRegistryFileStore) writeSnapshotLocked(snapshot projectRegistrySnapshot) error {
	if snapshot.Version <= 0 {
		snapshot.Version = 1
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project registry: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create project registry directory: %w", err)
		}
	}
	temp, err := os.CreateTemp(dir, ".project-registry-*.tmp")
	if err != nil {
		return fmt.Errorf("create project registry temp file: %w", err)
	}
	tempName := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempName)
	}
	if _, err := temp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write project registry temp file: %w", err)
	}
	if err := temp.Chmod(0o644); err != nil {
		cleanup()
		return fmt.Errorf("chmod project registry temp file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync project registry temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("close project registry temp file: %w", err)
	}
	if err := os.Rename(tempName, s.path); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("replace project registry file: %w", err)
	}
	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	return nil
}

func registryFromSnapshot(snapshot projectRegistrySnapshot) projectdomain.Registry {
	registry := projectdomain.Registry{
		Version:          snapshot.Version,
		DefaultProjectID: strings.TrimSpace(snapshot.DefaultProjectID),
		Projects:         make(map[string]projectdomain.Record, len(snapshot.Projects)),
		UpdatedAt:        snapshot.UpdatedAt,
	}
	for _, item := range snapshot.Projects {
		record := item
		record.ID = strings.TrimSpace(record.ID)
		if record.ID == "" {
			continue
		}
		registry.Projects[record.ID] = record
	}
	if registry.Version <= 0 {
		registry.Version = 1
	}
	return registry
}

func snapshotFromRegistry(registry projectdomain.Registry) projectRegistrySnapshot {
	snapshot := projectRegistrySnapshot{
		Version:          registry.Version,
		DefaultProjectID: strings.TrimSpace(registry.DefaultProjectID),
		UpdatedAt:        registry.UpdatedAt,
	}
	if snapshot.Version <= 0 {
		snapshot.Version = 1
	}

	if len(registry.Projects) == 0 {
		return snapshot
	}

	keys := make([]string, 0, len(registry.Projects))
	for key := range registry.Projects {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	snapshot.Projects = make([]projectdomain.Record, 0, len(keys))
	for _, key := range keys {
		record := registry.Projects[key]
		snapshot.Projects = append(snapshot.Projects, record)
	}
	return snapshot
}
