package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"clawx/internal/infrastructure/config"
)

type executionGoalState struct {
	Goal       string
	Status     string
	LastResult string
	UpdatedAt  time.Time
}

type executionGoalStore struct {
	mu     sync.RWMutex
	byConv map[string]executionGoalState
	loaded bool
}

var globalExecutionGoalStore = &executionGoalStore{
	byConv: map[string]executionGoalState{},
}

func setExecutionGoalState(conversationID string, state executionGoalState) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return
	}
	state.Goal = strings.TrimSpace(state.Goal)
	state.Status = strings.TrimSpace(strings.ToLower(state.Status))
	state.LastResult = strings.TrimSpace(state.LastResult)
	state.UpdatedAt = time.Now().UTC()
	globalExecutionGoalStore.mu.Lock()
	defer globalExecutionGoalStore.mu.Unlock()
	globalExecutionGoalStore.ensureLoadedLocked()
	globalExecutionGoalStore.byConv[conversationID] = state
	globalExecutionGoalStore.persistLocked()
}

func getExecutionGoalState(conversationID string) (executionGoalState, bool) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return executionGoalState{}, false
	}
	globalExecutionGoalStore.mu.RLock()
	if !globalExecutionGoalStore.loaded {
		globalExecutionGoalStore.mu.RUnlock()
		globalExecutionGoalStore.mu.Lock()
		globalExecutionGoalStore.ensureLoadedLocked()
		globalExecutionGoalStore.mu.Unlock()
		globalExecutionGoalStore.mu.RLock()
	}
	defer globalExecutionGoalStore.mu.RUnlock()
	state, ok := globalExecutionGoalStore.byConv[conversationID]
	return state, ok
}

func (s *executionGoalStore) ensureLoadedLocked() {
	if s.loaded {
		return
	}
	s.loaded = true
	path := executionGoalStatePath()
	body, err := os.ReadFile(path)
	if err != nil {
		return
	}
	raw := map[string]executionGoalState{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return
	}
	if s.byConv == nil {
		s.byConv = map[string]executionGoalState{}
	}
	for k, v := range raw {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		s.byConv[key] = v
	}
}

func (s *executionGoalStore) persistLocked() {
	path := executionGoalStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	payload, err := json.MarshalIndent(s.byConv, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, append(payload, '\n'), 0o644)
}

func executionGoalStatePath() string {
	return filepath.Join(config.StateDir(), "state", "execution_goals.json")
}
