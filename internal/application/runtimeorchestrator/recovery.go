package runtimeorchestrator

import (
	"fmt"
	"strings"
	"time"

	"clawx/internal/infrastructure/logging"
)

type RecoveryResult struct {
	StaleWorkers      []string
	RequeuedTaskIDs   []string
	ReassignedTaskIDs []string
}

type RecoveryEngine struct {
	queue      *Queue
	registry   *Registry
	dispatcher *Dispatcher
	recorder   *logging.RuntimeOrchestratorRecorder
	agentID    string
}

func NewRecoveryEngine(
	queue *Queue,
	registry *Registry,
	dispatcher *Dispatcher,
	recorder *logging.RuntimeOrchestratorRecorder,
	agentID string,
) *RecoveryEngine {
	return &RecoveryEngine{
		queue:      queue,
		registry:   registry,
		dispatcher: dispatcher,
		recorder:   recorder,
		agentID:    strings.TrimSpace(agentID),
	}
}

func (e *RecoveryEngine) Recover(now time.Time, staleAfter time.Duration, dispatchLimit int) (RecoveryResult, error) {
	if e == nil || e.queue == nil || e.registry == nil || e.dispatcher == nil {
		return RecoveryResult{}, fmt.Errorf("recovery engine is not initialized")
	}
	staleWorkers, err := e.registry.StaleWorkers(now.UTC(), staleAfter)
	if err != nil {
		return RecoveryResult{}, err
	}
	result := RecoveryResult{StaleWorkers: staleWorkers}
	if len(staleWorkers) == 0 {
		return result, nil
	}
	staleSet := make(map[string]struct{}, len(staleWorkers))
	for _, workerID := range staleWorkers {
		staleSet[workerID] = struct{}{}
		if recErr := e.registry.UpdateWorkerState(workerID, "offline", "", "executor", now.UTC()); recErr == nil {
			e.audit("worker_stale", "", workerID, "detected", "heartbeat_timeout")
		}
	}
	items, err := e.queue.List()
	if err != nil {
		return result, err
	}
	for _, task := range items {
		assigned := strings.TrimSpace(task.AssignedWorkerID)
		if task.Status != TaskRunning || assigned == "" {
			continue
		}
		if _, stale := staleSet[assigned]; !stale {
			continue
		}
		updated, ok, requeueErr := e.queue.RequeueTask(task.TaskID)
		if requeueErr != nil {
			return result, requeueErr
		}
		if !ok {
			continue
		}
		result.RequeuedTaskIDs = append(result.RequeuedTaskIDs, updated.TaskID)
		e.audit("task_requeue", updated.TaskID, assigned, "success", "worker_stale_requeue")
	}

	if dispatchLimit <= 0 {
		dispatchLimit = len(result.RequeuedTaskIDs)
	}
	if dispatchLimit <= 0 {
		return result, nil
	}
	decisions, err := e.dispatcher.DispatchQueued(now.UTC(), dispatchLimit)
	if err != nil {
		return result, err
	}
	for _, decision := range decisions {
		result.ReassignedTaskIDs = append(result.ReassignedTaskIDs, decision.TaskID)
		e.audit("task_reassign", decision.TaskID, decision.WorkerID, "success", decision.Reason)
	}
	return result, nil
}

func (e *RecoveryEngine) audit(event, taskID, workerID, status, reason string) {
	if e.recorder == nil {
		return
	}
	_ = e.recorder.Append(logging.RuntimeOrchestratorAuditRecord{
		AgentID:  strings.TrimSpace(e.agentID),
		Event:    strings.TrimSpace(event),
		TaskID:   strings.TrimSpace(taskID),
		WorkerID: strings.TrimSpace(workerID),
		Status:   strings.TrimSpace(status),
		Reason:   strings.TrimSpace(reason),
	})
}
