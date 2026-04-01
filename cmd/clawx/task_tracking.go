package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	taskPlanDocName         = "TASK_PLAN.md"
	taskExecutionDocName    = "TASK_EXECUTION.md"
	workspaceStateDirName   = ".clawx"
	workspaceStateFileName  = "workspace-state.json"
	workspaceStateFileVer   = 1
	maxTrackingTextLen      = 240
	maxTrackingReasonLen    = 160
	maxTrackingExecIDToKeep = 10
)

type workspaceTaskTrackingState struct {
	Version              int                    `json:"version"`
	LastTaskUpdatedAt    string                 `json:"lastTaskUpdatedAt,omitempty"`
	BootstrapSeededAt    string                 `json:"bootstrapSeededAt,omitempty"`
	OnboardingCompleted  string                 `json:"onboardingCompletedAt,omitempty"`
	TaskTrackingSnapshot map[string]interface{} `json:"taskTracking,omitempty"`
}

func resolveTaskTrackingRoot(runtime agentRuntime, fallbackCWD string) string {
	agentID := strings.TrimSpace(runtime.agentID)
	if agentID != "" {
		if agent, ok := runtime.cfgSnapshot.Agents[agentID]; ok {
			workspace := strings.TrimSpace(agent.Workspace)
			if workspace != "" && runtime.cfgSnapshot.ValidateWorkingDirectory(workspace) == nil {
				return filepath.Clean(workspace)
			}
		}
	}
	if cwd := strings.TrimSpace(fallbackCWD); cwd != "" {
		if runtime.cfgSnapshot.ValidateWorkingDirectory(cwd) == nil {
			return filepath.Clean(cwd)
		}
	}
	if cwd := strings.TrimSpace(runtime.cwd); cwd != "" {
		if runtime.cfgSnapshot.ValidateWorkingDirectory(cwd) == nil {
			return filepath.Clean(cwd)
		}
	}
	return ""
}

func appendTrackingGoal(runtime agentRuntime, fallbackCWD, conversationID, goal string) {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return
	}
	root := resolveTaskTrackingRoot(runtime, fallbackCWD)
	if strings.TrimSpace(root) == "" {
		return
	}
	now := time.Now().UTC()
	if err := ensureTaskTrackingScaffold(root, now); err != nil {
		return
	}
	line := fmt.Sprintf("- [%s] agent=%s conversation=%s goal=%s",
		now.Format(time.RFC3339),
		fallbackValue(strings.TrimSpace(runtime.agentID), "main"),
		fallbackValue(strings.TrimSpace(conversationID), "-"),
		summarizeText(goal, maxTrackingTextLen),
	)
	_ = appendMarkdownLine(filepath.Join(root, taskPlanDocName), line)
	_ = updateWorkspaceTaskState(root, now, map[string]interface{}{
		"agent_id":        strings.TrimSpace(runtime.agentID),
		"conversation_id": strings.TrimSpace(conversationID),
		"goal":            summarizeText(goal, maxTrackingTextLen),
		"status":          "running",
	})
}

func appendTrackingExecution(runtime agentRuntime, fallbackCWD, conversationID string, result runtimeExecApplyResult) {
	root := resolveTaskTrackingRoot(runtime, fallbackCWD)
	if strings.TrimSpace(root) == "" {
		return
	}
	now := time.Now().UTC()
	if err := ensureTaskTrackingScaffold(root, now); err != nil {
		return
	}
	ids := append([]string(nil), result.ExecIDs...)
	if len(ids) > maxTrackingExecIDToKeep {
		ids = ids[len(ids)-maxTrackingExecIDToKeep:]
	}
	execSummary := fmt.Sprintf(
		"- [%s] agent=%s conversation=%s status=%s total=%d success=%d failed=%d reason=%s",
		now.Format(time.RFC3339),
		fallbackValue(strings.TrimSpace(runtime.agentID), "main"),
		fallbackValue(strings.TrimSpace(conversationID), "-"),
		fallbackValue(strings.TrimSpace(result.Status), "unknown"),
		result.Total,
		result.Executed,
		result.Failed,
		summarizeText(strings.TrimSpace(result.FirstReason), maxTrackingReasonLen),
	)
	_ = appendMarkdownLine(filepath.Join(root, taskExecutionDocName), execSummary)
	if len(ids) > 0 {
		_ = appendMarkdownLine(filepath.Join(root, taskExecutionDocName), "  exec_ids: "+strings.Join(ids, ", "))
	}
	_ = updateWorkspaceTaskState(root, now, map[string]interface{}{
		"agent_id":        strings.TrimSpace(runtime.agentID),
		"conversation_id": strings.TrimSpace(conversationID),
		"status":          strings.TrimSpace(result.Status),
		"last_result":     summarizeText(strings.TrimSpace(result.FirstReason), maxTrackingReasonLen),
		"exec_total":      result.Total,
		"exec_success":    result.Executed,
		"exec_failed":     result.Failed,
		"exec_ids":        ids,
	})
}

func ensureTaskTrackingScaffold(root string, now time.Time) error {
	if err := os.MkdirAll(strings.TrimSpace(root), 0o755); err != nil {
		return err
	}
	planPath := filepath.Join(root, taskPlanDocName)
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		_ = os.WriteFile(planPath, []byte("# TASK_PLAN\n\n## Goals\n"), 0o644)
	}
	execPath := filepath.Join(root, taskExecutionDocName)
	if _, err := os.Stat(execPath); os.IsNotExist(err) {
		_ = os.WriteFile(execPath, []byte("# TASK_EXECUTION\n\n## Runs\n"), 0o644)
	}
	stateDir := filepath.Join(root, workspaceStateDirName)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	statePath := filepath.Join(stateDir, workspaceStateFileName)
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		state := workspaceTaskTrackingState{
			Version:           workspaceStateFileVer,
			BootstrapSeededAt: now.Format(time.RFC3339),
		}
		payload, _ := json.MarshalIndent(state, "", "  ")
		_ = os.WriteFile(statePath, append(payload, '\n'), 0o644)
	}
	return nil
}

func updateWorkspaceTaskState(root string, now time.Time, fields map[string]interface{}) error {
	statePath := filepath.Join(root, workspaceStateDirName, workspaceStateFileName)
	state := workspaceTaskTrackingState{}
	if body, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(body, &state)
	}
	if state.Version == 0 {
		state.Version = workspaceStateFileVer
	}
	if strings.TrimSpace(state.BootstrapSeededAt) == "" {
		state.BootstrapSeededAt = now.Format(time.RFC3339)
	}
	if state.TaskTrackingSnapshot == nil {
		state.TaskTrackingSnapshot = map[string]interface{}{}
	}
	for k, v := range fields {
		state.TaskTrackingSnapshot[k] = v
	}
	state.LastTaskUpdatedAt = now.Format(time.RFC3339)
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, append(payload, '\n'), 0o644)
}

func appendMarkdownLine(path, line string) error {
	path = strings.TrimSpace(path)
	line = strings.TrimSpace(line)
	if path == "" || line == "" {
		return nil
	}
	body, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := string(body)
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += line + "\n"
	return os.WriteFile(path, []byte(text), 0o644)
}
