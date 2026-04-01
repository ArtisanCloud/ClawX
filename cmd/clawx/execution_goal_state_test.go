package main

import (
	"testing"
)

func TestExecutionGoalStatePersistsAcrossStoreReload(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	globalExecutionGoalStore = &executionGoalStore{
		byConv: map[string]executionGoalState{},
	}
	setExecutionGoalState("conv-persist", executionGoalState{
		Goal:       "持续修复 500 错误直到通过探针",
		Status:     "running",
		LastResult: "已执行第一轮修复",
	})

	// Simulate process restart by reinitializing in-memory store.
	globalExecutionGoalStore = &executionGoalStore{
		byConv: map[string]executionGoalState{},
	}
	state, ok := getExecutionGoalState("conv-persist")
	if !ok {
		t.Fatalf("expected persisted execution goal state")
	}
	if state.Goal != "持续修复 500 错误直到通过探针" {
		t.Fatalf("unexpected goal: %q", state.Goal)
	}
	if state.Status != "running" {
		t.Fatalf("unexpected status: %q", state.Status)
	}
}
