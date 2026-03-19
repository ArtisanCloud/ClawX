package configplan

import (
	"strings"
	"sync"
)

type MemoryStore struct {
	mu    sync.RWMutex
	plans map[string]Plan
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		plans: make(map[string]Plan),
	}
}

func (s *MemoryStore) Set(conversationID string, plan Plan) error {
	key := strings.TrimSpace(conversationID)
	if key == "" {
		return ErrInvalidPlan
	}
	plan = plan.Normalize()
	plan.ConversationID = key
	if err := plan.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.plans[key] = plan
	return nil
}

func (s *MemoryStore) Get(conversationID string) (Plan, bool) {
	key := strings.TrimSpace(conversationID)
	if key == "" {
		return Plan{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	plan, ok := s.plans[key]
	return plan, ok
}

func (s *MemoryStore) Pop(conversationID string) (Plan, bool) {
	key := strings.TrimSpace(conversationID)
	if key == "" {
		return Plan{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, ok := s.plans[key]
	if ok {
		delete(s.plans, key)
	}
	return plan, ok
}

func (s *MemoryStore) Delete(conversationID string) bool {
	key := strings.TrimSpace(conversationID)
	if key == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.plans[key]; ok {
		delete(s.plans, key)
		return true
	}
	return false
}
