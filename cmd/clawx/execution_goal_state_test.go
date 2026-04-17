package main

import (
	"testing"
	"time"
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
	if state.TaskID == "" {
		t.Fatalf("expected generated task id")
	}
}

func TestExecutionGoalStateTaskIDRotatesWhenGoalChanges(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	globalExecutionGoalStore = &executionGoalStore{
		byConv: map[string]executionGoalState{},
	}

	setExecutionGoalState("conv-task-rotate", executionGoalState{
		Goal:   "完成发布",
		Status: "running",
	})
	first, ok := getExecutionGoalState("conv-task-rotate")
	if !ok {
		t.Fatalf("expected first state")
	}
	if first.TaskID == "" {
		t.Fatalf("expected first task id")
	}
	time.Sleep(2 * time.Millisecond)

	setExecutionGoalState("conv-task-rotate", executionGoalState{
		Goal:   "修复线上告警",
		Status: "pending",
	})
	second, ok := getExecutionGoalState("conv-task-rotate")
	if !ok {
		t.Fatalf("expected second state")
	}
	if second.TaskID == "" {
		t.Fatalf("expected second task id")
	}
	if second.TaskID == first.TaskID {
		t.Fatalf("expected task id rotation when goal changes, first=%s second=%s", first.TaskID, second.TaskID)
	}
}

func TestExecutionGoalStateTaskIDStaysWhenGoalUnchanged(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	globalExecutionGoalStore = &executionGoalStore{
		byConv: map[string]executionGoalState{},
	}

	setExecutionGoalState("conv-task-stable", executionGoalState{
		Goal:   "完成发布",
		Status: "running",
	})
	first, ok := getExecutionGoalState("conv-task-stable")
	if !ok {
		t.Fatalf("expected first state")
	}
	setExecutionGoalState("conv-task-stable", executionGoalState{
		Goal:       "完成发布",
		Status:     "running",
		LastResult: "执行完成一半",
	})
	second, ok := getExecutionGoalState("conv-task-stable")
	if !ok {
		t.Fatalf("expected second state")
	}
	if second.TaskID != first.TaskID {
		t.Fatalf("expected task id to remain stable, first=%s second=%s", first.TaskID, second.TaskID)
	}
}

func TestExecutionGoalStateKeepsRuntimeExecDecisionAcrossUpdates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	globalExecutionGoalStore = &executionGoalStore{
		byConv: map[string]executionGoalState{},
	}

	setExecutionGoalState("conv-decision-keep", executionGoalState{
		Goal:                                "持续修复服务异常",
		Status:                              "running",
		RuntimeExecDecisionMode:             "service.retry_only",
		RuntimeExecDecisionSource:           "user_phrase",
		RuntimeExecDecisionAlternateCommand: "",
	})
	setExecutionGoalState("conv-decision-keep", executionGoalState{
		Status:     "running",
		LastResult: "已完成一次重启与健康检查",
	})

	state, ok := getExecutionGoalState("conv-decision-keep")
	if !ok {
		t.Fatalf("expected decision state exists")
	}
	if state.RuntimeExecDecisionMode != "service.retry_only" {
		t.Fatalf("expected decision mode kept, got: %q", state.RuntimeExecDecisionMode)
	}
	if state.RuntimeExecDecisionSource != "user_phrase" {
		t.Fatalf("expected decision source kept, got: %q", state.RuntimeExecDecisionSource)
	}
}

func TestExecutionGoalStateClearsRuntimeExecDecisionWhenCompleted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	globalExecutionGoalStore = &executionGoalStore{
		byConv: map[string]executionGoalState{},
	}

	setExecutionGoalState("conv-decision-clear", executionGoalState{
		Goal:                                "完成构建修复",
		Status:                              "running",
		RuntimeExecDecisionMode:             "build.switch_source",
		RuntimeExecDecisionSource:           "user_phrase",
		RuntimeExecDecisionAlternateCommand: "echo override",
	})
	setExecutionGoalState("conv-decision-clear", executionGoalState{
		Status: "completed",
	})

	state, ok := getExecutionGoalState("conv-decision-clear")
	if !ok {
		t.Fatalf("expected completion state exists")
	}
	if state.RuntimeExecDecisionMode != "" {
		t.Fatalf("expected decision mode cleared on completion, got: %q", state.RuntimeExecDecisionMode)
	}
	if state.RuntimeExecDecisionSource != "" {
		t.Fatalf("expected decision source cleared on completion, got: %q", state.RuntimeExecDecisionSource)
	}
	if state.RuntimeExecDecisionAlternateCommand != "" {
		t.Fatalf("expected decision alternate command cleared on completion, got: %q", state.RuntimeExecDecisionAlternateCommand)
	}
}

func TestExecutionGoalStateClearsRuntimeExecDecisionWhenTaskRotates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	globalExecutionGoalStore = &executionGoalStore{
		byConv: map[string]executionGoalState{},
	}

	setExecutionGoalState("conv-decision-rotate", executionGoalState{
		Goal:                      "修复服务异常",
		Status:                    "running",
		RuntimeExecDecisionMode:   "service.retry_only",
		RuntimeExecDecisionSource: "user_phrase",
	})
	first, ok := getExecutionGoalState("conv-decision-rotate")
	if !ok {
		t.Fatalf("expected initial state")
	}
	if first.TaskID == "" {
		t.Fatalf("expected initial task id")
	}

	time.Sleep(2 * time.Millisecond)
	setExecutionGoalState("conv-decision-rotate", executionGoalState{
		Goal:   "修复构建失败",
		Status: "pending",
	})
	second, ok := getExecutionGoalState("conv-decision-rotate")
	if !ok {
		t.Fatalf("expected rotated state")
	}
	if second.TaskID == "" || second.TaskID == first.TaskID {
		t.Fatalf("expected task rotated, first=%s second=%s", first.TaskID, second.TaskID)
	}
	if second.RuntimeExecDecisionMode != "" {
		t.Fatalf("expected decision mode cleared on task rotation, got: %q", second.RuntimeExecDecisionMode)
	}
	if second.RuntimeExecDecisionSource != "" {
		t.Fatalf("expected decision source cleared on task rotation, got: %q", second.RuntimeExecDecisionSource)
	}
}

func TestExecutionGoalStateKeepsExplicitRuntimeExecDecisionOnTaskRotation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	globalExecutionGoalStore = &executionGoalStore{
		byConv: map[string]executionGoalState{},
	}

	setExecutionGoalState("conv-decision-rotate-explicit", executionGoalState{
		Goal:                      "修复服务异常",
		Status:                    "running",
		RuntimeExecDecisionMode:   "service.retry_only",
		RuntimeExecDecisionSource: "user_phrase",
	})

	time.Sleep(2 * time.Millisecond)
	setExecutionGoalState("conv-decision-rotate-explicit", executionGoalState{
		Goal:                      "修复构建失败",
		Status:                    "pending",
		RuntimeExecDecisionMode:   "build.switch_source",
		RuntimeExecDecisionSource: "user_phrase",
	})
	state, ok := getExecutionGoalState("conv-decision-rotate-explicit")
	if !ok {
		t.Fatalf("expected rotated explicit state")
	}
	if state.RuntimeExecDecisionMode != "build.switch_source" {
		t.Fatalf("expected explicit decision mode kept, got: %q", state.RuntimeExecDecisionMode)
	}
	if state.RuntimeExecDecisionSource != "user_phrase" {
		t.Fatalf("expected explicit decision source kept, got: %q", state.RuntimeExecDecisionSource)
	}
}
