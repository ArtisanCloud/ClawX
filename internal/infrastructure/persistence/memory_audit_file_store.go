package persistence

import (
	"bufio"
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

type MemoryAuditFileStore struct {
	mu            sync.Mutex
	workspaceRoot string
}

func NewMemoryAuditFileStore(workspaceRoot string) (*MemoryAuditFileStore, error) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return nil, fmt.Errorf("workspace root is required")
	}
	return &MemoryAuditFileStore{workspaceRoot: filepath.Clean(workspaceRoot)}, nil
}

func (s *MemoryAuditFileStore) Append(_ context.Context, record memorydomain.AuditRecord) error {
	if err := record.ScopeKey.Validate(); err != nil {
		return err
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.auditLogPath(record.ScopeKey.ProjectID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create memory audit directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open memory audit log: %w", err)
	}
	defer file.Close()

	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal memory audit record: %w", err)
	}
	if _, err := file.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("append memory audit log: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync memory audit log: %w", err)
	}
	return nil
}

func (s *MemoryAuditFileStore) ListByProject(_ context.Context, projectID string, limit int) ([]memorydomain.AuditRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.auditLogPath(projectID)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open memory audit log: %w", err)
	}
	defer file.Close()

	results := make([]memorydomain.AuditRecord, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record memorydomain.AuditRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, fmt.Errorf("parse memory audit record: %w", err)
		}
		results = append(results, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan memory audit records: %w", err)
	}
	if limit > 0 && len(results) > limit {
		results = results[len(results)-limit:]
	}
	return results, nil
}

func (s *MemoryAuditFileStore) auditLogPath(projectID string) (string, error) {
	projectID = normalizeMemoryPathSegment(projectID)
	if projectID == "" {
		return "", memorydomain.ErrInvalidMemoryScope
	}
	projectRoot := filepath.Join(s.workspaceRoot, projectID)
	path := filepath.Join(projectRoot, ".memory", "audit", "records.jsonl")
	if err := ensurePathWithinRoot(projectRoot, path); err != nil {
		return "", err
	}
	return path, nil
}
