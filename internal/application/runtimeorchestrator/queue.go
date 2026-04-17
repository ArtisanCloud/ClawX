package runtimeorchestrator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type TaskStatus string

const (
	TaskQueued    TaskStatus = "queued"
	TaskRunning   TaskStatus = "running"
	TaskSucceeded TaskStatus = "succeeded"
	TaskFailed    TaskStatus = "failed"
	TaskCanceled  TaskStatus = "canceled"
)

type RuntimeTask struct {
	TaskID           string                 `json:"task_id"`
	Source           string                 `json:"source"`
	Intent           string                 `json:"intent"`
	Payload          map[string]interface{} `json:"payload,omitempty"`
	Status           TaskStatus             `json:"status"`
	AssignedWorkerID string                 `json:"assigned_worker_id,omitempty"`
	Retry            int                    `json:"retry"`
	MaxRetry         int                    `json:"max_retry"`
	CreatedAt        string                 `json:"created_at"`
	UpdatedAt        string                 `json:"updated_at"`
}

type Queue struct {
	mu   sync.Mutex
	file string
}

func NewQueue(file string) *Queue {
	return &Queue{file: strings.TrimSpace(file)}
}

func (q *Queue) Enqueue(task RuntimeTask) (RuntimeTask, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.ensureFile(); err != nil {
		return RuntimeTask{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if strings.TrimSpace(task.TaskID) == "" {
		task.TaskID = fmt.Sprintf("t-%d", time.Now().UTC().UnixNano())
	}
	task.TaskID = strings.TrimSpace(task.TaskID)
	if strings.TrimSpace(task.Source) == "" {
		task.Source = "nl"
	}
	task.Intent = strings.TrimSpace(task.Intent)
	task.AssignedWorkerID = strings.TrimSpace(task.AssignedWorkerID)
	if task.Status == "" {
		task.Status = TaskQueued
	}
	if task.MaxRetry <= 0 {
		task.MaxRetry = 3
	}
	if strings.TrimSpace(task.CreatedAt) == "" {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	if task.Payload == nil {
		task.Payload = map[string]interface{}{}
	}
	line, err := json.Marshal(task)
	if err != nil {
		return RuntimeTask{}, err
	}
	f, err := os.OpenFile(q.file, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return RuntimeTask{}, err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return RuntimeTask{}, err
	}
	return task, nil
}

func (q *Queue) List() ([]RuntimeTask, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.ensureFile(); err != nil {
		return nil, err
	}
	return q.readAll()
}

func (q *Queue) AssignNext(workerID string) (RuntimeTask, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.ensureFile(); err != nil {
		return RuntimeTask{}, false, err
	}
	items, err := q.readAll()
	if err != nil {
		return RuntimeTask{}, false, err
	}
	workerID = strings.TrimSpace(workerID)
	for idx := range items {
		if items[idx].Status != TaskQueued {
			continue
		}
		items[idx].Status = TaskRunning
		items[idx].AssignedWorkerID = workerID
		items[idx].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := q.writeAll(items); err != nil {
			return RuntimeTask{}, false, err
		}
		return items[idx], true, nil
	}
	return RuntimeTask{}, false, nil
}

func (q *Queue) AssignTask(taskID string, workerID string) (RuntimeTask, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.ensureFile(); err != nil {
		return RuntimeTask{}, false, err
	}
	items, err := q.readAll()
	if err != nil {
		return RuntimeTask{}, false, err
	}
	taskID = strings.TrimSpace(taskID)
	workerID = strings.TrimSpace(workerID)
	for idx := range items {
		if items[idx].TaskID != taskID {
			continue
		}
		if items[idx].Status != TaskQueued {
			return RuntimeTask{}, false, nil
		}
		items[idx].Status = TaskRunning
		items[idx].AssignedWorkerID = workerID
		items[idx].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := q.writeAll(items); err != nil {
			return RuntimeTask{}, false, err
		}
		return items[idx], true, nil
	}
	return RuntimeTask{}, false, nil
}

func (q *Queue) UpdateStatus(taskID string, status TaskStatus) (RuntimeTask, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.ensureFile(); err != nil {
		return RuntimeTask{}, false, err
	}
	items, err := q.readAll()
	if err != nil {
		return RuntimeTask{}, false, err
	}
	taskID = strings.TrimSpace(taskID)
	for idx := range items {
		if items[idx].TaskID != taskID {
			continue
		}
		items[idx].Status = status
		items[idx].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := q.writeAll(items); err != nil {
			return RuntimeTask{}, false, err
		}
		return items[idx], true, nil
	}
	return RuntimeTask{}, false, nil
}

func (q *Queue) MergePayload(taskID string, fields map[string]interface{}) (RuntimeTask, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.ensureFile(); err != nil {
		return RuntimeTask{}, false, err
	}
	items, err := q.readAll()
	if err != nil {
		return RuntimeTask{}, false, err
	}
	taskID = strings.TrimSpace(taskID)
	for idx := range items {
		if items[idx].TaskID != taskID {
			continue
		}
		if items[idx].Payload == nil {
			items[idx].Payload = map[string]interface{}{}
		}
		for key, value := range fields {
			normalizedKey := strings.TrimSpace(key)
			if normalizedKey == "" {
				continue
			}
			if value == nil {
				delete(items[idx].Payload, normalizedKey)
				continue
			}
			items[idx].Payload[normalizedKey] = value
		}
		items[idx].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := q.writeAll(items); err != nil {
			return RuntimeTask{}, false, err
		}
		return items[idx], true, nil
	}
	return RuntimeTask{}, false, nil
}

func (q *Queue) RequeueTask(taskID string) (RuntimeTask, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.ensureFile(); err != nil {
		return RuntimeTask{}, false, err
	}
	items, err := q.readAll()
	if err != nil {
		return RuntimeTask{}, false, err
	}
	taskID = strings.TrimSpace(taskID)
	for idx := range items {
		if items[idx].TaskID != taskID {
			continue
		}
		if items[idx].Status != TaskRunning {
			return RuntimeTask{}, false, nil
		}
		items[idx].Status = TaskQueued
		items[idx].AssignedWorkerID = ""
		items[idx].Retry++
		items[idx].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := q.writeAll(items); err != nil {
			return RuntimeTask{}, false, err
		}
		return items[idx], true, nil
	}
	return RuntimeTask{}, false, nil
}

func (q *Queue) RetryTask(taskID string) (RuntimeTask, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.ensureFile(); err != nil {
		return RuntimeTask{}, false, err
	}
	items, err := q.readAll()
	if err != nil {
		return RuntimeTask{}, false, err
	}
	taskID = strings.TrimSpace(taskID)
	for idx := range items {
		if items[idx].TaskID != taskID {
			continue
		}
		switch items[idx].Status {
		case TaskFailed, TaskCanceled:
			if items[idx].MaxRetry > 0 && items[idx].Retry >= items[idx].MaxRetry {
				return RuntimeTask{}, false, fmt.Errorf("task retry exceeded max_retry=%d", items[idx].MaxRetry)
			}
			items[idx].Status = TaskQueued
			items[idx].AssignedWorkerID = ""
			items[idx].Retry++
			items[idx].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if err := q.writeAll(items); err != nil {
				return RuntimeTask{}, false, err
			}
			return items[idx], true, nil
		default:
			return RuntimeTask{}, false, nil
		}
	}
	return RuntimeTask{}, false, nil
}

func (q *Queue) CancelTask(taskID string) (RuntimeTask, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.ensureFile(); err != nil {
		return RuntimeTask{}, false, err
	}
	items, err := q.readAll()
	if err != nil {
		return RuntimeTask{}, false, err
	}
	taskID = strings.TrimSpace(taskID)
	for idx := range items {
		if items[idx].TaskID != taskID {
			continue
		}
		switch items[idx].Status {
		case TaskQueued, TaskRunning:
			items[idx].Status = TaskCanceled
			items[idx].AssignedWorkerID = ""
			items[idx].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if err := q.writeAll(items); err != nil {
				return RuntimeTask{}, false, err
			}
			return items[idx], true, nil
		default:
			return RuntimeTask{}, false, nil
		}
	}
	return RuntimeTask{}, false, nil
}

func (q *Queue) ensureFile() error {
	if strings.TrimSpace(q.file) == "" {
		return fmt.Errorf("queue file is required")
	}
	if err := os.MkdirAll(filepath.Dir(q.file), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(q.file); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(q.file, []byte(""), 0o644)
}

func (q *Queue) readAll() ([]RuntimeTask, error) {
	f, err := os.Open(q.file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	out := make([]RuntimeTask, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item RuntimeTask
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (q *Queue) writeAll(items []RuntimeTask) error {
	lines := make([]byte, 0, 1024)
	for _, item := range items {
		line, err := json.Marshal(item)
		if err != nil {
			return err
		}
		lines = append(lines, line...)
		lines = append(lines, '\n')
	}
	return os.WriteFile(q.file, lines, 0o644)
}
