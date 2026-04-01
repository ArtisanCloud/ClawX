package main

import (
	"strings"
	"sync"
	"time"
)

type specKitFlowState struct {
	Active    bool
	UpdatedAt time.Time
}

type specKitFlowStore struct {
	mu     sync.RWMutex
	byConv map[string]specKitFlowState
}

var globalSpecKitFlowStore = &specKitFlowStore{
	byConv: map[string]specKitFlowState{},
}

func setSpecKitFlowActive(conversationID string, active bool) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return
	}
	globalSpecKitFlowStore.mu.Lock()
	defer globalSpecKitFlowStore.mu.Unlock()
	globalSpecKitFlowStore.byConv[conversationID] = specKitFlowState{
		Active:    active,
		UpdatedAt: time.Now().UTC(),
	}
}

func isSpecKitFlowActive(conversationID string) bool {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return false
	}
	globalSpecKitFlowStore.mu.RLock()
	defer globalSpecKitFlowStore.mu.RUnlock()
	state, ok := globalSpecKitFlowStore.byConv[conversationID]
	if !ok {
		return false
	}
	// Avoid stale sticky state forever.
	if time.Since(state.UpdatedAt) > 6*time.Hour {
		return false
	}
	return state.Active
}
