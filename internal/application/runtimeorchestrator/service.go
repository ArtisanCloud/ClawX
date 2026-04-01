package runtimeorchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultWorkerCount = 3
)

type Service struct{}

type BootstrapOptions struct {
	WorkspaceRoot string
	AgentID       string
	WorkerRoles   []string
}

type BootstrapResult struct {
	RuntimeDir        string
	TasksFile         string
	WorkerStatesFile  string
	HeartbeatsFile    string
	RuntimeMetaFile   string
	DispatchStateFile string
	WorkerCount       int
	WorkerIDs         []string
}

type workerState struct {
	WorkerID        string `json:"worker_id"`
	Role            string `json:"role"`
	Status          string `json:"status"`
	LastHeartbeatAt string `json:"last_heartbeat_at"`
	CurrentTaskID   string `json:"current_task_id"`
}

type workerStateEnvelope struct {
	UpdatedAt string        `json:"updated_at"`
	Workers   []workerState `json:"workers"`
}

type heartbeatEnvelope struct {
	UpdatedAt  string            `json:"updated_at"`
	Heartbeats map[string]string `json:"heartbeats"`
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Bootstrap(opts BootstrapOptions) (BootstrapResult, error) {
	_ = s
	root := filepath.Clean(strings.TrimSpace(opts.WorkspaceRoot))
	if root == "" || root == "." {
		return BootstrapResult{}, fmt.Errorf("workspace root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return BootstrapResult{}, err
	}

	runtimeDir := filepath.Join(root, ".clawx", "runtime")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		return BootstrapResult{}, err
	}

	tasksFile := filepath.Join(runtimeDir, "tasks.jsonl")
	workerStatesFile := filepath.Join(runtimeDir, "worker_states.json")
	heartbeatsFile := filepath.Join(runtimeDir, "heartbeats.json")
	runtimeMetaFile := filepath.Join(runtimeDir, "runtime_meta.json")
	dispatchStateFile := filepath.Join(runtimeDir, "dispatch_state.json")

	if err := ensureFile(tasksFile); err != nil {
		return BootstrapResult{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	roles := normalizeWorkerRoles(opts.WorkerRoles)
	workers := make([]workerState, 0, len(roles))
	heartbeats := make(map[string]string, len(roles))
	workerIDs := make([]string, 0, len(roles))
	for idx, role := range roles {
		id := fmt.Sprintf("w-%s-%d", role, idx+1)
		workers = append(workers, workerState{
			WorkerID:        id,
			Role:            role,
			Status:          "idle",
			LastHeartbeatAt: now,
			CurrentTaskID:   "",
		})
		heartbeats[id] = now
		workerIDs = append(workerIDs, id)
	}

	if err := writeJSON(workerStatesFile, workerStateEnvelope{
		UpdatedAt: now,
		Workers:   workers,
	}); err != nil {
		return BootstrapResult{}, err
	}
	if err := writeJSON(heartbeatsFile, heartbeatEnvelope{
		UpdatedAt:  now,
		Heartbeats: heartbeats,
	}); err != nil {
		return BootstrapResult{}, err
	}
	if err := writeJSON(runtimeMetaFile, map[string]any{
		"updated_at": now,
		"agent_id":   strings.TrimSpace(opts.AgentID),
		"lead_mode":  true,
	}); err != nil {
		return BootstrapResult{}, err
	}
	if err := writeJSON(dispatchStateFile, map[string]any{
		"updated_at":    now,
		"sticky_routes": map[string]string{},
	}); err != nil {
		return BootstrapResult{}, err
	}

	return BootstrapResult{
		RuntimeDir:        runtimeDir,
		TasksFile:         tasksFile,
		WorkerStatesFile:  workerStatesFile,
		HeartbeatsFile:    heartbeatsFile,
		RuntimeMetaFile:   runtimeMetaFile,
		DispatchStateFile: dispatchStateFile,
		WorkerCount:       len(workerIDs),
		WorkerIDs:         workerIDs,
	}, nil
}

func normalizeWorkerRoles(input []string) []string {
	if len(input) == 0 {
		return []string{"planner", "executor", "reviewer"}
	}
	out := make([]string, 0, len(input))
	seen := map[string]struct{}{}
	for _, raw := range input {
		role := strings.ToLower(strings.TrimSpace(raw))
		if role == "" {
			continue
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		out = append(out, role)
	}
	if len(out) == 0 {
		return []string{"planner", "executor", "reviewer"}
	}
	return out
}

func ensureFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte(""), 0o644)
}

func writeJSON(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}
