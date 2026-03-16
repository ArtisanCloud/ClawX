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

	memorydomain "clawx/internal/domain/memory"
)

type memoryDigestSnapshot struct {
	Version int                      `json:"version"`
	Jobs    []memorydomain.DigestJob `json:"jobs"`
}

type MemoryDigestFileStore struct {
	mu            sync.Mutex
	workspaceRoot string
}

func NewMemoryDigestFileStore(workspaceRoot string) (*MemoryDigestFileStore, error) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return nil, fmt.Errorf("workspace root is required")
	}
	return &MemoryDigestFileStore{workspaceRoot: filepath.Clean(workspaceRoot)}, nil
}

func (s *MemoryDigestFileStore) Upsert(_ context.Context, job memorydomain.DigestJob) error {
	if err := job.ScopeKey.Validate(); err != nil {
		return err
	}
	job.JobID = strings.TrimSpace(job.JobID)
	if job.JobID == "" {
		return memorydomain.ErrInvalidMemoryItem
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.digestPath(job.ScopeKey.ProjectID)
	if err != nil {
		return err
	}
	snapshot, err := readDigestSnapshot(path)
	if err != nil {
		if err != memorydomain.ErrMemoryNotFound {
			return err
		}
		snapshot = memoryDigestSnapshot{Version: 1, Jobs: make([]memorydomain.DigestJob, 0)}
	}

	updated := false
	for idx := range snapshot.Jobs {
		if strings.TrimSpace(snapshot.Jobs[idx].JobID) == job.JobID {
			snapshot.Jobs[idx] = job
			updated = true
			break
		}
	}
	if !updated {
		snapshot.Jobs = append(snapshot.Jobs, job)
	}
	sort.Slice(snapshot.Jobs, func(i, j int) bool {
		return snapshot.Jobs[i].StartedAt.Before(snapshot.Jobs[j].StartedAt)
	})
	return writeDigestSnapshot(path, snapshot)
}

func (s *MemoryDigestFileStore) Get(_ context.Context, projectID, jobID string) (memorydomain.DigestJob, error) {
	projectID = normalizeMemoryPathSegment(projectID)
	jobID = strings.TrimSpace(jobID)
	if projectID == "" || jobID == "" {
		return memorydomain.DigestJob{}, memorydomain.ErrInvalidMemoryScope
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.digestPath(projectID)
	if err != nil {
		return memorydomain.DigestJob{}, err
	}
	snapshot, err := readDigestSnapshot(path)
	if err != nil {
		return memorydomain.DigestJob{}, err
	}
	for _, job := range snapshot.Jobs {
		if strings.TrimSpace(job.JobID) == jobID {
			return job, nil
		}
	}
	return memorydomain.DigestJob{}, memorydomain.ErrMemoryNotFound
}

func (s *MemoryDigestFileStore) ListByProject(_ context.Context, projectID string, limit int) ([]memorydomain.DigestJob, error) {
	projectID = normalizeMemoryPathSegment(projectID)
	if projectID == "" {
		return nil, memorydomain.ErrInvalidMemoryScope
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.digestPath(projectID)
	if err != nil {
		return nil, err
	}
	snapshot, err := readDigestSnapshot(path)
	if err != nil {
		if err == memorydomain.ErrMemoryNotFound {
			return nil, nil
		}
		return nil, err
	}
	jobs := make([]memorydomain.DigestJob, len(snapshot.Jobs))
	copy(jobs, snapshot.Jobs)
	if limit > 0 && len(jobs) > limit {
		jobs = jobs[len(jobs)-limit:]
	}
	return jobs, nil
}

func (s *MemoryDigestFileStore) digestPath(projectID string) (string, error) {
	projectID = normalizeMemoryPathSegment(projectID)
	if projectID == "" {
		return "", memorydomain.ErrInvalidMemoryScope
	}
	projectRoot := filepath.Join(s.workspaceRoot, projectID)
	path := filepath.Join(projectRoot, ".memory", "digest", "jobs.json")
	if err := ensurePathWithinRoot(projectRoot, path); err != nil {
		return "", err
	}
	return path, nil
}

func readDigestSnapshot(path string) (memoryDigestSnapshot, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return memoryDigestSnapshot{Version: 1, Jobs: make([]memorydomain.DigestJob, 0)}, memorydomain.ErrMemoryNotFound
		}
		return memoryDigestSnapshot{}, fmt.Errorf("open memory digest snapshot: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var snapshot memoryDigestSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return memoryDigestSnapshot{}, fmt.Errorf("parse memory digest snapshot: %w", err)
	}
	if decoder.More() {
		return memoryDigestSnapshot{}, fmt.Errorf("parse memory digest snapshot: trailing data")
	}
	if snapshot.Version <= 0 {
		snapshot.Version = 1
	}
	if snapshot.Jobs == nil {
		snapshot.Jobs = make([]memorydomain.DigestJob, 0)
	}
	return snapshot, nil
}

func writeDigestSnapshot(path string, snapshot memoryDigestSnapshot) error {
	if snapshot.Version <= 0 {
		snapshot.Version = 1
	}
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal memory digest snapshot: %w", err)
	}
	payload = append(payload, '\n')
	return writeMemoryAtomic(path, payload, ".memory-digest-*.tmp")
}
