package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	skilldomain "clawx/internal/domain/skill"
)

type SkillPolicyFileStore struct {
	mu   sync.Mutex
	path string
}

func NewSkillPolicyFileStore(path string) (*SkillPolicyFileStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("skill policy path is required")
	}
	return &SkillPolicyFileStore{path: path}, nil
}

func (s *SkillPolicyFileStore) Load(_ context.Context) (skilldomain.SkillPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return skilldomain.SkillPolicy{}.Normalize(), nil
		}
		return skilldomain.SkillPolicy{}, fmt.Errorf("open skill policy: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var policy skilldomain.SkillPolicy
	if err := decoder.Decode(&policy); err != nil {
		return skilldomain.SkillPolicy{}, fmt.Errorf("parse skill policy: %w", err)
	}
	if decoder.More() {
		return skilldomain.SkillPolicy{}, fmt.Errorf("parse skill policy: trailing data")
	}
	policy = policy.Normalize()
	if err := policy.Validate(); err != nil {
		return skilldomain.SkillPolicy{}, err
	}
	return policy, nil
}

func (s *SkillPolicyFileStore) Save(_ context.Context, policy skilldomain.SkillPolicy) error {
	normalized := policy.Normalize()
	if err := normalized.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal skill policy: %w", err)
	}
	data = append(data, '\n')
	return writeAtomicJSONFile(s.path, data, ".skill-policy-*.tmp")
}
