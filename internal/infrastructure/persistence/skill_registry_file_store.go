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

	skilldomain "clawx/internal/domain/skill"
)

type skillRegistrySnapshot struct {
	Version int                         `json:"version"`
	Skills  []skilldomain.SkillMetadata `json:"skills"`
}

type SkillRegistryFileStore struct {
	mu   sync.Mutex
	path string
}

func NewSkillRegistryFileStore(path string) (*SkillRegistryFileStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("skill registry path is required")
	}
	return &SkillRegistryFileStore{path: path}, nil
}

func (s *SkillRegistryFileStore) Upsert(_ context.Context, metadata skilldomain.SkillMetadata) error {
	normalized := metadata.Normalize()
	if err := normalized.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return err
	}
	replaced := false
	for idx := range snapshot.Skills {
		if strings.TrimSpace(snapshot.Skills[idx].SkillID) == normalized.SkillID {
			snapshot.Skills[idx] = normalized
			replaced = true
			break
		}
	}
	if !replaced {
		snapshot.Skills = append(snapshot.Skills, normalized)
	}
	sort.Slice(snapshot.Skills, func(i, j int) bool {
		return snapshot.Skills[i].SkillID < snapshot.Skills[j].SkillID
	})
	return s.writeSnapshotLocked(snapshot)
}

func (s *SkillRegistryFileStore) GetByID(_ context.Context, skillID string) (skilldomain.SkillMetadata, error) {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return skilldomain.SkillMetadata{}, skilldomain.ErrSkillMetadataNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return skilldomain.SkillMetadata{}, err
	}
	for _, item := range snapshot.Skills {
		if strings.TrimSpace(item.SkillID) == skillID {
			return item, nil
		}
	}
	return skilldomain.SkillMetadata{}, skilldomain.ErrSkillMetadataNotFound
}

func (s *SkillRegistryFileStore) List(_ context.Context) ([]skilldomain.SkillMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return nil, err
	}
	out := make([]skilldomain.SkillMetadata, len(snapshot.Skills))
	copy(out, snapshot.Skills)
	return out, nil
}

func (s *SkillRegistryFileStore) Delete(_ context.Context, skillID string) error {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return err
	}
	filtered := snapshot.Skills[:0]
	for _, item := range snapshot.Skills {
		if strings.TrimSpace(item.SkillID) == skillID {
			continue
		}
		filtered = append(filtered, item)
	}
	snapshot.Skills = filtered
	return s.writeSnapshotLocked(snapshot)
}

func (s *SkillRegistryFileStore) readSnapshotLocked() (skillRegistrySnapshot, error) {
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return skillRegistrySnapshot{Version: 1}, nil
		}
		return skillRegistrySnapshot{}, fmt.Errorf("open skill registry: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var snapshot skillRegistrySnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return skillRegistrySnapshot{}, fmt.Errorf("parse skill registry: %w", err)
	}
	if decoder.More() {
		return skillRegistrySnapshot{}, fmt.Errorf("parse skill registry: trailing data")
	}
	if snapshot.Version <= 0 {
		snapshot.Version = 1
	}
	if snapshot.Skills == nil {
		snapshot.Skills = make([]skilldomain.SkillMetadata, 0)
	}
	return snapshot, nil
}

func (s *SkillRegistryFileStore) writeSnapshotLocked(snapshot skillRegistrySnapshot) error {
	if snapshot.Version <= 0 {
		snapshot.Version = 1
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal skill registry: %w", err)
	}
	data = append(data, '\n')
	return writeAtomicJSONFile(s.path, data, ".skill-registry-*.tmp")
}

func writeAtomicJSONFile(path string, data []byte, tempPattern string) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %q: %w", dir, err)
		}
	}
	temp, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tempName := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempName)
	}
	if _, err := temp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := temp.Chmod(0o644); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("replace file: %w", err)
	}
	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	return nil
}
