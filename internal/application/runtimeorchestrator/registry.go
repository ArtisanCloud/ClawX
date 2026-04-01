package runtimeorchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Registry struct {
	mu              sync.Mutex
	workerStateFile string
	heartbeatsFile  string
}

func NewRegistry(workerStateFile string, heartbeatsFile string) *Registry {
	return &Registry{
		workerStateFile: strings.TrimSpace(workerStateFile),
		heartbeatsFile:  strings.TrimSpace(heartbeatsFile),
	}
}

func (r *Registry) Workers() ([]workerState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.ensureFiles(); err != nil {
		return nil, err
	}
	states, err := r.loadWorkerStates()
	if err != nil {
		return nil, err
	}
	sort.Slice(states.Workers, func(i, j int) bool {
		return states.Workers[i].WorkerID < states.Workers[j].WorkerID
	})
	return states.Workers, nil
}

func (r *Registry) IdleWorkers() ([]workerState, error) {
	workers, err := r.Workers()
	if err != nil {
		return nil, err
	}
	out := make([]workerState, 0, len(workers))
	for _, item := range workers {
		status := strings.TrimSpace(strings.ToLower(item.Status))
		taskID := strings.TrimSpace(item.CurrentTaskID)
		if taskID != "" {
			continue
		}
		if status == "idle" || status == "online" || status == "" {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].WorkerID < out[j].WorkerID
	})
	return out, nil
}

func (r *Registry) ReportHeartbeat(workerID string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.ensureFiles(); err != nil {
		return err
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return fmt.Errorf("worker id is required")
	}
	ts := at.UTC().Format(time.RFC3339)

	beats, err := r.loadHeartbeats()
	if err != nil {
		return err
	}
	if beats.Heartbeats == nil {
		beats.Heartbeats = map[string]string{}
	}
	beats.Heartbeats[workerID] = ts
	beats.UpdatedAt = ts
	if err := writeJSON(r.heartbeatsFile, beats); err != nil {
		return err
	}

	states, err := r.loadWorkerStates()
	if err != nil {
		return err
	}
	found := false
	for idx := range states.Workers {
		if states.Workers[idx].WorkerID != workerID {
			continue
		}
		states.Workers[idx].LastHeartbeatAt = ts
		if strings.TrimSpace(states.Workers[idx].Status) == "" {
			states.Workers[idx].Status = "online"
		}
		found = true
		break
	}
	if !found {
		states.Workers = append(states.Workers, workerState{
			WorkerID:        workerID,
			Role:            "executor",
			Status:          "online",
			LastHeartbeatAt: ts,
			CurrentTaskID:   "",
		})
	}
	states.UpdatedAt = ts
	return writeJSON(r.workerStateFile, states)
}

func (r *Registry) UpdateWorkerState(workerID string, status string, currentTaskID string, role string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.ensureFiles(); err != nil {
		return err
	}
	workerID = strings.TrimSpace(workerID)
	status = strings.TrimSpace(strings.ToLower(status))
	currentTaskID = strings.TrimSpace(currentTaskID)
	role = strings.TrimSpace(strings.ToLower(role))
	if workerID == "" {
		return fmt.Errorf("worker id is required")
	}
	if status == "" {
		status = "online"
	}
	if role == "" {
		role = "executor"
	}
	ts := at.UTC().Format(time.RFC3339)

	states, err := r.loadWorkerStates()
	if err != nil {
		return err
	}
	found := false
	for idx := range states.Workers {
		if states.Workers[idx].WorkerID != workerID {
			continue
		}
		states.Workers[idx].Status = status
		states.Workers[idx].Role = role
		states.Workers[idx].CurrentTaskID = currentTaskID
		states.Workers[idx].LastHeartbeatAt = ts
		found = true
		break
	}
	if !found {
		states.Workers = append(states.Workers, workerState{
			WorkerID:        workerID,
			Role:            role,
			Status:          status,
			LastHeartbeatAt: ts,
			CurrentTaskID:   currentTaskID,
		})
	}
	states.UpdatedAt = ts
	if err := writeJSON(r.workerStateFile, states); err != nil {
		return err
	}

	beats, err := r.loadHeartbeats()
	if err != nil {
		return err
	}
	if beats.Heartbeats == nil {
		beats.Heartbeats = map[string]string{}
	}
	beats.Heartbeats[workerID] = ts
	beats.UpdatedAt = ts
	return writeJSON(r.heartbeatsFile, beats)
}

func (r *Registry) StaleWorkers(now time.Time, staleAfter time.Duration) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.ensureFiles(); err != nil {
		return nil, err
	}
	if staleAfter <= 0 {
		staleAfter = 30 * time.Minute
	}
	beats, err := r.loadHeartbeats()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0)
	for workerID, raw := range beats.Heartbeats {
		ts, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(raw))
		if parseErr != nil {
			out = append(out, workerID)
			continue
		}
		if now.UTC().Sub(ts.UTC()) >= staleAfter {
			out = append(out, workerID)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (r *Registry) ensureFiles() error {
	if strings.TrimSpace(r.workerStateFile) == "" || strings.TrimSpace(r.heartbeatsFile) == "" {
		return fmt.Errorf("registry files are required")
	}
	for _, p := range []string{r.workerStateFile, r.heartbeatsFile} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if _, err := os.Stat(p); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		switch filepath.Base(p) {
		case "worker_states.json":
			initial := workerStateEnvelope{
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
				Workers:   []workerState{},
			}
			if err := writeJSON(p, initial); err != nil {
				return err
			}
		case "heartbeats.json":
			initial := heartbeatEnvelope{
				UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
				Heartbeats: map[string]string{},
			}
			if err := writeJSON(p, initial); err != nil {
				return err
			}
		default:
			if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Registry) loadWorkerStates() (workerStateEnvelope, error) {
	body, err := os.ReadFile(r.workerStateFile)
	if err != nil {
		return workerStateEnvelope{}, err
	}
	state := workerStateEnvelope{}
	if len(strings.TrimSpace(string(body))) == 0 {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		state.Workers = []workerState{}
		return state, nil
	}
	if err := json.Unmarshal(body, &state); err != nil {
		return workerStateEnvelope{}, err
	}
	if state.Workers == nil {
		state.Workers = []workerState{}
	}
	return state, nil
}

func (r *Registry) loadHeartbeats() (heartbeatEnvelope, error) {
	body, err := os.ReadFile(r.heartbeatsFile)
	if err != nil {
		return heartbeatEnvelope{}, err
	}
	beats := heartbeatEnvelope{}
	if len(strings.TrimSpace(string(body))) == 0 {
		beats.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		beats.Heartbeats = map[string]string{}
		return beats, nil
	}
	if err := json.Unmarshal(body, &beats); err != nil {
		return heartbeatEnvelope{}, err
	}
	if beats.Heartbeats == nil {
		beats.Heartbeats = map[string]string{}
	}
	return beats, nil
}
