package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PairingState string

const (
	PairingPending PairingState = "pending"
	PairingPaired  PairingState = "paired"
	PairingExpired PairingState = "expired"
	PairingRevoked PairingState = "revoked"
)

type PairingRecord struct {
	PairingID     string       `json:"pairing_id"`
	Channel       string       `json:"channel"`
	UserID        string       `json:"user_id"`
	State         PairingState `json:"state"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	ExpiresAt     *time.Time   `json:"expires_at,omitempty"`
	RevokedReason string       `json:"revoked_reason,omitempty"`
}

type PairingStore struct {
	path    string
	mu      sync.RWMutex
	records map[string]PairingRecord
}

func NewPairingStore(path string) (*PairingStore, error) {
	store := &PairingStore{
		path:    strings.TrimSpace(path),
		records: make(map[string]PairingRecord),
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *PairingStore) Get(channel, userID string) (PairingRecord, bool) {
	key := pairingKey(channel, userID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[key]
	return record, ok
}

func (s *PairingStore) Upsert(record PairingRecord) error {
	record.Channel = strings.TrimSpace(record.Channel)
	record.UserID = strings.TrimSpace(record.UserID)
	if record.Channel == "" || record.UserID == "" {
		return fmt.Errorf("pairing channel and user_id are required")
	}
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	if record.PairingID == "" {
		record.PairingID = fmt.Sprintf("pair-%d", now.UnixNano())
	}
	if record.State == "" {
		record.State = PairingPending
	}

	key := pairingKey(record.Channel, record.UserID)
	s.mu.Lock()
	s.records[key] = record
	s.mu.Unlock()
	return s.persist()
}

func (s *PairingStore) MarkState(channel, userID string, state PairingState, reason string, expiresAt *time.Time) error {
	key := pairingKey(channel, userID)
	s.mu.Lock()
	record, ok := s.records[key]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("pairing not found")
	}
	record.State = state
	record.RevokedReason = strings.TrimSpace(reason)
	record.ExpiresAt = expiresAt
	record.UpdatedAt = time.Now().UTC()
	s.records[key] = record
	s.mu.Unlock()
	return s.persist()
}

func (s *PairingStore) ExpireDue(now time.Time) error {
	s.mu.Lock()
	changed := false
	for key, record := range s.records {
		if record.ExpiresAt == nil || record.State != PairingPaired {
			continue
		}
		if !record.ExpiresAt.Before(now) && !record.ExpiresAt.Equal(now) {
			continue
		}
		record.State = PairingExpired
		record.UpdatedAt = now.UTC()
		s.records[key] = record
		changed = true
	}
	s.mu.Unlock()
	if !changed {
		return nil
	}
	return s.persist()
}

func (s *PairingStore) load() error {
	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	body, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read pairing store: %w", err)
	}
	var payload map[string]PairingRecord
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("parse pairing store: %w", err)
	}
	s.records = payload
	return nil
}

func (s *PairingStore) persist() error {
	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	s.mu.RLock()
	payload := make(map[string]PairingRecord, len(s.records))
	for key, value := range s.records {
		payload[key] = value
	}
	s.mu.RUnlock()

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal pairing store: %w", err)
	}
	body = append(body, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create pairing store directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "pairing-*.tmp")
	if err != nil {
		return fmt.Errorf("create pairing temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write pairing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close pairing temp file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace pairing store: %w", err)
	}
	return nil
}

func pairingKey(channel, userID string) string {
	return strings.TrimSpace(channel) + ":" + strings.TrimSpace(userID)
}
