package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	skilldomain "clawx/internal/domain/skill"
)

type skillBindingSnapshot struct {
	Version  int                        `json:"version"`
	Bindings []skilldomain.SkillBinding `json:"bindings"`
}

type SkillBindingFileStore struct {
	mu   sync.Mutex
	path string
}

func NewSkillBindingFileStore(path string) (*SkillBindingFileStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("skill binding path is required")
	}
	return &SkillBindingFileStore{path: path}, nil
}

func (s *SkillBindingFileStore) Upsert(_ context.Context, binding skilldomain.SkillBinding) error {
	normalized := binding.Normalize()
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
	key := normalized.Key()
	for idx := range snapshot.Bindings {
		if snapshot.Bindings[idx].Key() == key {
			snapshot.Bindings[idx] = normalized
			replaced = true
			break
		}
	}
	if !replaced {
		snapshot.Bindings = append(snapshot.Bindings, normalized)
	}
	sort.Slice(snapshot.Bindings, func(i, j int) bool {
		return snapshot.Bindings[i].Key() < snapshot.Bindings[j].Key()
	})
	return s.writeSnapshotLocked(snapshot)
}

func (s *SkillBindingFileStore) Delete(_ context.Context, binding skilldomain.SkillBinding) error {
	normalized := binding.Normalize()
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return err
	}
	target := normalized.Key()
	filtered := snapshot.Bindings[:0]
	for _, item := range snapshot.Bindings {
		if item.Key() == target {
			continue
		}
		filtered = append(filtered, item)
	}
	snapshot.Bindings = filtered
	return s.writeSnapshotLocked(snapshot)
}

func (s *SkillBindingFileStore) List(_ context.Context) ([]skilldomain.SkillBinding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return nil, err
	}
	out := make([]skilldomain.SkillBinding, len(snapshot.Bindings))
	copy(out, snapshot.Bindings)
	return out, nil
}

func (s *SkillBindingFileStore) readSnapshotLocked() (skillBindingSnapshot, error) {
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return skillBindingSnapshot{Version: 1}, nil
		}
		return skillBindingSnapshot{}, fmt.Errorf("open skill bindings: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var snapshot skillBindingSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return skillBindingSnapshot{}, fmt.Errorf("parse skill bindings: %w", err)
	}
	if decoder.More() {
		return skillBindingSnapshot{}, fmt.Errorf("parse skill bindings: trailing data")
	}
	if snapshot.Version <= 0 {
		snapshot.Version = 1
	}
	if snapshot.Bindings == nil {
		snapshot.Bindings = make([]skilldomain.SkillBinding, 0)
	}
	return snapshot, nil
}

func (s *SkillBindingFileStore) writeSnapshotLocked(snapshot skillBindingSnapshot) error {
	if snapshot.Version <= 0 {
		snapshot.Version = 1
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal skill bindings: %w", err)
	}
	data = append(data, '\n')
	return writeAtomicJSONFile(s.path, data, ".skill-bindings-*.tmp")
}
