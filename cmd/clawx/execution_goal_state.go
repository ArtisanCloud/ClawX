package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"clawx/internal/infrastructure/config"
)

type executionGoalState struct {
	TaskID                              string    `json:"task_id,omitempty"`
	AgentID                             string    `json:"agent_id,omitempty"`
	Goal                                string    `json:"goal,omitempty"`
	Status                              string    `json:"status,omitempty"`
	LastResult                          string    `json:"last_result,omitempty"`
	RemainingSteps                      []string  `json:"remaining_steps,omitempty"`
	DoneCriteria                        []string  `json:"done_criteria,omitempty"`
	Evidence                            []string  `json:"evidence,omitempty"`
	NextAction                          string    `json:"next_action,omitempty"`
	RuntimeExecDecisionMode             string    `json:"runtime_exec_decision_mode,omitempty"`
	RuntimeExecDecisionSource           string    `json:"runtime_exec_decision_source,omitempty"`
	RuntimeExecDecisionAlternateCommand string    `json:"runtime_exec_decision_alternate_command,omitempty"`
	LoopRound                           int       `json:"loop_round,omitempty"`
	LoopMaxRounds                       int       `json:"loop_max_rounds,omitempty"`
	StopReason                          string    `json:"stop_reason,omitempty"`
	StartedAt                           time.Time `json:"started_at,omitempty"`
	UpdatedAt                           time.Time `json:"updated_at,omitempty"`
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
	globalExecutionGoalStore.mu.Lock()
	defer globalExecutionGoalStore.mu.Unlock()
	globalExecutionGoalStore.ensureLoadedLocked()
	existing := globalExecutionGoalStore.byConv[conversationID]
	globalExecutionGoalStore.byConv[conversationID] = mergeExecutionGoalState(existing, state, conversationID)
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

func mergeExecutionGoalState(existing executionGoalState, incoming executionGoalState, conversationID string) executionGoalState {
	incoming.TaskID = strings.TrimSpace(incoming.TaskID)
	incoming.AgentID = strings.TrimSpace(incoming.AgentID)
	incoming.Goal = strings.TrimSpace(incoming.Goal)
	incoming.Status = normalizeExecutionTaskStatus(incoming.Status)
	incoming.LastResult = strings.TrimSpace(incoming.LastResult)
	incoming.RemainingSteps = normalizeExecutionStringList(incoming.RemainingSteps)
	incoming.DoneCriteria = normalizeExecutionStringList(incoming.DoneCriteria)
	incoming.Evidence = normalizeExecutionStringList(incoming.Evidence)
	incoming.NextAction = strings.TrimSpace(incoming.NextAction)
	incoming.RuntimeExecDecisionMode = normalizeRuntimeExecDecisionMode(incoming.RuntimeExecDecisionMode)
	explicitRuntimeExecDecision := incoming.RuntimeExecDecisionMode != ""
	incoming.RuntimeExecDecisionSource = strings.TrimSpace(incoming.RuntimeExecDecisionSource)
	clearRuntimeExecDecision := strings.EqualFold(incoming.RuntimeExecDecisionSource, "__clear__")
	incoming.RuntimeExecDecisionAlternateCommand = strings.TrimSpace(incoming.RuntimeExecDecisionAlternateCommand)
	incoming.StopReason = strings.TrimSpace(incoming.StopReason)

	if incoming.Goal == "" {
		incoming.Goal = strings.TrimSpace(existing.Goal)
	}
	if incoming.AgentID == "" {
		incoming.AgentID = strings.TrimSpace(existing.AgentID)
	}
	if incoming.Status == "" {
		incoming.Status = normalizeExecutionTaskStatus(existing.Status)
	}
	if incoming.Status == "" {
		incoming.Status = "pending"
	}
	taskRotated := shouldRotateExecutionTaskID(existing, incoming)
	if strings.TrimSpace(existing.TaskID) != "" && strings.TrimSpace(incoming.TaskID) != "" && strings.TrimSpace(existing.TaskID) != strings.TrimSpace(incoming.TaskID) {
		taskRotated = true
	}
	if incoming.LastResult == "" && strings.TrimSpace(existing.LastResult) != "" {
		incoming.LastResult = strings.TrimSpace(existing.LastResult)
	}
	if len(incoming.RemainingSteps) == 0 && len(existing.RemainingSteps) > 0 && incoming.Status != "completed" {
		incoming.RemainingSteps = append([]string(nil), existing.RemainingSteps...)
	}
	if len(incoming.DoneCriteria) == 0 && len(existing.DoneCriteria) > 0 && incoming.Status != "completed" {
		incoming.DoneCriteria = append([]string(nil), existing.DoneCriteria...)
	}
	if len(incoming.Evidence) == 0 && len(existing.Evidence) > 0 {
		incoming.Evidence = append([]string(nil), existing.Evidence...)
	}
	if incoming.NextAction == "" && existing.NextAction != "" && incoming.Status != "completed" {
		incoming.NextAction = strings.TrimSpace(existing.NextAction)
	}
	if !clearRuntimeExecDecision && incoming.RuntimeExecDecisionMode == "" && existing.RuntimeExecDecisionMode != "" && incoming.Status != "completed" {
		incoming.RuntimeExecDecisionMode = normalizeRuntimeExecDecisionMode(existing.RuntimeExecDecisionMode)
	}
	if !clearRuntimeExecDecision && incoming.RuntimeExecDecisionSource == "" && existing.RuntimeExecDecisionSource != "" && incoming.Status != "completed" {
		incoming.RuntimeExecDecisionSource = strings.TrimSpace(existing.RuntimeExecDecisionSource)
	}
	if !clearRuntimeExecDecision && incoming.RuntimeExecDecisionAlternateCommand == "" && existing.RuntimeExecDecisionAlternateCommand != "" && incoming.Status != "completed" {
		incoming.RuntimeExecDecisionAlternateCommand = strings.TrimSpace(existing.RuntimeExecDecisionAlternateCommand)
	}
	if shouldResetRuntimeExecDecisionLock(taskRotated, explicitRuntimeExecDecision) {
		incoming.RuntimeExecDecisionMode = ""
		incoming.RuntimeExecDecisionSource = ""
		incoming.RuntimeExecDecisionAlternateCommand = ""
	}
	if clearRuntimeExecDecision {
		incoming.RuntimeExecDecisionMode = ""
		incoming.RuntimeExecDecisionSource = ""
		incoming.RuntimeExecDecisionAlternateCommand = ""
	}
	if incoming.LoopRound <= 0 {
		incoming.LoopRound = existing.LoopRound
	}
	if incoming.LoopMaxRounds <= 0 {
		incoming.LoopMaxRounds = existing.LoopMaxRounds
	}
	if incoming.StopReason == "" && existing.StopReason != "" {
		incoming.StopReason = strings.TrimSpace(existing.StopReason)
	}

	if incoming.StartedAt.IsZero() {
		if !existing.StartedAt.IsZero() {
			incoming.StartedAt = existing.StartedAt
		} else {
			incoming.StartedAt = time.Now().UTC()
		}
	}
	incoming.UpdatedAt = time.Now().UTC()

	if incoming.TaskID == "" {
		if taskRotated {
			incoming.TaskID = newExecutionTaskID(conversationID)
		} else {
			incoming.TaskID = strings.TrimSpace(existing.TaskID)
		}
	}
	if incoming.TaskID == "" {
		incoming.TaskID = newExecutionTaskID(conversationID)
	}

	if incoming.Status == "completed" {
		incoming.RemainingSteps = nil
		incoming.NextAction = ""
		incoming.RuntimeExecDecisionMode = ""
		incoming.RuntimeExecDecisionSource = ""
		incoming.RuntimeExecDecisionAlternateCommand = ""
	}
	return incoming
}

func shouldResetRuntimeExecDecisionLock(taskRotated bool, explicitRuntimeExecDecision bool) bool {
	if explicitRuntimeExecDecision {
		return false
	}
	if taskRotated {
		return true
	}
	return false
}

func shouldRotateExecutionTaskID(existing executionGoalState, incoming executionGoalState) bool {
	if strings.TrimSpace(existing.TaskID) == "" {
		return true
	}
	existingGoal := strings.TrimSpace(existing.Goal)
	incomingGoal := strings.TrimSpace(incoming.Goal)
	incomingStatus := normalizeExecutionTaskStatus(incoming.Status)
	if existingGoal != "" && incomingGoal != "" && existingGoal != incomingGoal && (incomingStatus == "pending" || incomingStatus == "running") {
		return true
	}
	if incomingStatus == "pending" && existingGoal != "" && incomingGoal != "" && existingGoal != incomingGoal {
		return true
	}
	return false
}

func normalizeExecutionTaskStatus(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case "pending", "running", "blocked", "completed", "failed":
		return value
	case "applied", "done", "success", "succeeded":
		return "completed"
	case "partial":
		return "blocked"
	case "error":
		return "failed"
	case "in_progress", "in-progress":
		return "running"
	case "queued", "waiting":
		return "pending"
	default:
		return value
	}
}

func normalizeExecutionStringList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		normalized = append(normalized, item)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func newExecutionTaskID(conversationID string) string {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		conversationID = "task"
	}
	sum := sha1.Sum([]byte(conversationID))
	segment := hex.EncodeToString(sum[:])
	if len(segment) > 8 {
		segment = segment[:8]
	}
	return fmt.Sprintf("task-%s-%s", segment, time.Now().UTC().Format("20060102T150405.000Z"))
}
