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

	projectdomain "clawx/internal/domain/project"
)

type projectProposalSnapshot struct {
	Version   int                      `json:"version"`
	Proposals []projectdomain.Proposal `json:"proposals"`
}

type ProjectProposalFileStore struct {
	mu   sync.Mutex
	path string
}

func NewProjectProposalFileStore(path string) (*ProjectProposalFileStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("proposal path is required")
	}
	return &ProjectProposalFileStore{path: path}, nil
}

func (s *ProjectProposalFileStore) GetByID(_ context.Context, proposalID string) (projectdomain.Proposal, error) {
	proposalID = strings.TrimSpace(proposalID)
	if proposalID == "" {
		return projectdomain.Proposal{}, projectdomain.ErrProposalNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return projectdomain.Proposal{}, err
	}
	for _, item := range snapshot.Proposals {
		if strings.TrimSpace(item.ID) == proposalID {
			return item, nil
		}
	}
	return projectdomain.Proposal{}, projectdomain.ErrProposalNotFound
}

func (s *ProjectProposalFileStore) Upsert(_ context.Context, proposal projectdomain.Proposal) error {
	if err := proposal.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return err
	}

	updated := false
	for idx := range snapshot.Proposals {
		if strings.TrimSpace(snapshot.Proposals[idx].ID) == proposal.ID {
			snapshot.Proposals[idx] = proposal
			updated = true
			break
		}
	}
	if !updated {
		snapshot.Proposals = append(snapshot.Proposals, proposal)
	}

	sort.Slice(snapshot.Proposals, func(i, j int) bool {
		return snapshot.Proposals[i].ID < snapshot.Proposals[j].ID
	})
	return s.writeSnapshotLocked(snapshot)
}

func (s *ProjectProposalFileStore) List(_ context.Context) ([]projectdomain.Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return nil, err
	}
	result := make([]projectdomain.Proposal, len(snapshot.Proposals))
	copy(result, snapshot.Proposals)
	return result, nil
}

func (s *ProjectProposalFileStore) readSnapshotLocked() (projectProposalSnapshot, error) {
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return projectProposalSnapshot{Version: 1}, nil
		}
		return projectProposalSnapshot{}, fmt.Errorf("open project proposals: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var snapshot projectProposalSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return projectProposalSnapshot{}, fmt.Errorf("parse project proposals: %w", err)
	}
	if decoder.More() {
		return projectProposalSnapshot{}, fmt.Errorf("parse project proposals: trailing data")
	}
	if snapshot.Version <= 0 {
		snapshot.Version = 1
	}
	if snapshot.Proposals == nil {
		snapshot.Proposals = make([]projectdomain.Proposal, 0)
	}
	return snapshot, nil
}

func (s *ProjectProposalFileStore) writeSnapshotLocked(snapshot projectProposalSnapshot) error {
	if snapshot.Version <= 0 {
		snapshot.Version = 1
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project proposals: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create project proposal directory: %w", err)
		}
	}
	temp, err := os.CreateTemp(dir, ".project-proposals-*.tmp")
	if err != nil {
		return fmt.Errorf("create project proposal temp file: %w", err)
	}
	tempName := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempName)
	}
	if _, err := temp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write project proposal temp file: %w", err)
	}
	if err := temp.Chmod(0o644); err != nil {
		cleanup()
		return fmt.Errorf("chmod project proposal temp file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync project proposal temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("close project proposal temp file: %w", err)
	}
	if err := os.Rename(tempName, s.path); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("replace project proposal file: %w", err)
	}
	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	return nil
}
