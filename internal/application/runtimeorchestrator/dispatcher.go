package runtimeorchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type DispatchDecision struct {
	TaskID     string `json:"task_id"`
	WorkerID   string `json:"worker_id"`
	Resource   string `json:"resource"`
	Reason     string `json:"reason"`
	AssignedAt string `json:"assigned_at"`
}

type dispatchStateEnvelope struct {
	UpdatedAt    string            `json:"updated_at"`
	StickyRoutes map[string]string `json:"sticky_routes"`
}

type Dispatcher struct {
	mu        sync.Mutex
	queue     *Queue
	registry  *Registry
	stateFile string
}

func NewDispatcher(queue *Queue, registry *Registry, stateFile string) *Dispatcher {
	return &Dispatcher{
		queue:     queue,
		registry:  registry,
		stateFile: strings.TrimSpace(stateFile),
	}
}

func (d *Dispatcher) DispatchQueued(now time.Time, limit int) ([]DispatchDecision, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.queue == nil || d.registry == nil {
		return nil, fmt.Errorf("dispatcher requires queue and registry")
	}
	if err := d.ensureStateFile(); err != nil {
		return nil, err
	}
	state, err := d.loadState()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 1
	}

	tasks, err := d.queue.List()
	if err != nil {
		return nil, err
	}
	decisions := make([]DispatchDecision, 0, limit)
	for _, task := range tasks {
		if len(decisions) >= limit {
			break
		}
		if task.Status != TaskQueued {
			continue
		}
		idleWorkers, workerErr := d.registry.IdleWorkers()
		if workerErr != nil {
			return decisions, workerErr
		}
		if len(idleWorkers) == 0 {
			break
		}

		resource := dispatchResourceKey(task)
		worker, reason := pickDispatchWorker(idleWorkers, state.StickyRoutes, resource)
		if strings.TrimSpace(worker.WorkerID) == "" {
			continue
		}
		updatedTask, assigned, assignErr := d.queue.AssignTask(task.TaskID, worker.WorkerID)
		if assignErr != nil {
			return decisions, assignErr
		}
		if !assigned {
			continue
		}
		ts := now.UTC().Format(time.RFC3339)
		if err := d.registry.UpdateWorkerState(worker.WorkerID, "busy", updatedTask.TaskID, worker.Role, now.UTC()); err != nil {
			return decisions, err
		}
		if resource != "" {
			state.StickyRoutes[resource] = worker.WorkerID
		}
		decisions = append(decisions, DispatchDecision{
			TaskID:     updatedTask.TaskID,
			WorkerID:   worker.WorkerID,
			Resource:   resource,
			Reason:     reason,
			AssignedAt: ts,
		})
	}
	state.UpdatedAt = now.UTC().Format(time.RFC3339)
	if err := d.writeState(state); err != nil {
		return decisions, err
	}
	return decisions, nil
}

func dispatchResourceKey(task RuntimeTask) string {
	if task.Payload != nil {
		for _, key := range []string{"resource_key", "source_key", "workspace"} {
			if raw, ok := task.Payload[key]; ok {
				if v := strings.TrimSpace(fmt.Sprint(raw)); v != "" {
					return strings.ToLower(v)
				}
			}
		}
	}
	if v := strings.TrimSpace(task.Source); v != "" {
		return strings.ToLower(v)
	}
	return strings.ToLower(strings.TrimSpace(task.Intent))
}

func pickDispatchWorker(idle []workerState, sticky map[string]string, resource string) (workerState, string) {
	if len(idle) == 0 {
		return workerState{}, "no_idle_worker"
	}
	if sticky != nil && resource != "" {
		if preferred := strings.TrimSpace(sticky[resource]); preferred != "" {
			for _, item := range idle {
				if item.WorkerID == preferred {
					return item, "sticky_resource"
				}
			}
		}
	}
	return idle[0], "idle_first"
}

func (d *Dispatcher) ensureStateFile() error {
	if strings.TrimSpace(d.stateFile) == "" {
		return fmt.Errorf("dispatch state file is required")
	}
	if err := os.MkdirAll(filepath.Dir(d.stateFile), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(d.stateFile); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	initial := dispatchStateEnvelope{
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
		StickyRoutes: map[string]string{},
	}
	return writeJSON(d.stateFile, initial)
}

func (d *Dispatcher) loadState() (dispatchStateEnvelope, error) {
	body, err := os.ReadFile(d.stateFile)
	if err != nil {
		return dispatchStateEnvelope{}, err
	}
	state := dispatchStateEnvelope{}
	if len(strings.TrimSpace(string(body))) == 0 {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		state.StickyRoutes = map[string]string{}
		return state, nil
	}
	if err := json.Unmarshal(body, &state); err != nil {
		return dispatchStateEnvelope{}, err
	}
	if state.StickyRoutes == nil {
		state.StickyRoutes = map[string]string{}
	}
	return state, nil
}

func (d *Dispatcher) writeState(state dispatchStateEnvelope) error {
	if state.StickyRoutes == nil {
		state.StickyRoutes = map[string]string{}
	}
	return writeJSON(d.stateFile, state)
}
