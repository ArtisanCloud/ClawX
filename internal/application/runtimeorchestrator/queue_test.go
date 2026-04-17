package runtimeorchestrator

import (
	"path/filepath"
	"testing"
)

func TestQueueEnqueueAssignAndUpdate(t *testing.T) {
	tmp := t.TempDir()
	q := NewQueue(filepath.Join(tmp, "tasks.jsonl"))

	task, err := q.Enqueue(RuntimeTask{
		Source: "nl",
		Intent: "runtime.bootstrap",
		Status: TaskQueued,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if task.TaskID == "" {
		t.Fatalf("task id should not be empty")
	}

	list, err := q.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("unexpected list len: %d", len(list))
	}
	if list[0].Status != TaskQueued {
		t.Fatalf("unexpected task status: %s", list[0].Status)
	}

	assigned, ok, err := q.AssignNext("w-executor-1")
	if err != nil {
		t.Fatalf("assign: %v", err)
	}
	if !ok {
		t.Fatalf("expected assign success")
	}
	if assigned.Status != TaskRunning {
		t.Fatalf("unexpected assigned status: %s", assigned.Status)
	}
	if assigned.AssignedWorkerID != "w-executor-1" {
		t.Fatalf("unexpected assigned worker: %s", assigned.AssignedWorkerID)
	}

	updated, ok, err := q.UpdateStatus(assigned.TaskID, TaskSucceeded)
	if err != nil {
		t.Fatalf("update status: %v", err)
	}
	if !ok {
		t.Fatalf("expected update success")
	}
	if updated.Status != TaskSucceeded {
		t.Fatalf("unexpected updated status: %s", updated.Status)
	}
}

func TestQueueAssignTaskByID(t *testing.T) {
	tmp := t.TempDir()
	q := NewQueue(filepath.Join(tmp, "tasks.jsonl"))
	first, err := q.Enqueue(RuntimeTask{Source: "nl", Intent: "a", Status: TaskQueued})
	if err != nil {
		t.Fatalf("enqueue first: %v", err)
	}
	second, err := q.Enqueue(RuntimeTask{Source: "nl", Intent: "b", Status: TaskQueued})
	if err != nil {
		t.Fatalf("enqueue second: %v", err)
	}

	assigned, ok, err := q.AssignTask(second.TaskID, "w-executor-2")
	if err != nil {
		t.Fatalf("assign task by id: %v", err)
	}
	if !ok {
		t.Fatalf("expected assign by id success")
	}
	if assigned.TaskID != second.TaskID {
		t.Fatalf("unexpected assigned task id: %s", assigned.TaskID)
	}
	if assigned.AssignedWorkerID != "w-executor-2" {
		t.Fatalf("unexpected assigned worker: %s", assigned.AssignedWorkerID)
	}
	list, err := q.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("unexpected list len: %d", len(list))
	}
	if list[0].TaskID != first.TaskID || list[0].Status != TaskQueued {
		t.Fatalf("first task should remain queued: %+v", list[0])
	}
	if list[1].TaskID != second.TaskID || list[1].Status != TaskRunning {
		t.Fatalf("second task should be running: %+v", list[1])
	}
}

func TestQueueRequeueTask(t *testing.T) {
	tmp := t.TempDir()
	q := NewQueue(filepath.Join(tmp, "tasks.jsonl"))
	task, err := q.Enqueue(RuntimeTask{
		Source: "nl",
		Intent: "recover-me",
		Status: TaskQueued,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	assigned, ok, err := q.AssignTask(task.TaskID, "w-executor-1")
	if err != nil {
		t.Fatalf("assign by id: %v", err)
	}
	if !ok || assigned.Status != TaskRunning {
		t.Fatalf("expected running assigned task, got=%+v ok=%v", assigned, ok)
	}

	requeued, ok, err := q.RequeueTask(task.TaskID)
	if err != nil {
		t.Fatalf("requeue: %v", err)
	}
	if !ok {
		t.Fatalf("expected requeue success")
	}
	if requeued.Status != TaskQueued {
		t.Fatalf("expected queued status after requeue, got=%s", requeued.Status)
	}
	if requeued.AssignedWorkerID != "" {
		t.Fatalf("expected cleared assigned worker after requeue, got=%s", requeued.AssignedWorkerID)
	}
	if requeued.Retry != 1 {
		t.Fatalf("expected retry increment to 1, got=%d", requeued.Retry)
	}
}

func TestQueueMergePayload(t *testing.T) {
	tmp := t.TempDir()
	q := NewQueue(filepath.Join(tmp, "tasks.jsonl"))
	task, err := q.Enqueue(RuntimeTask{
		Source: "nl",
		Intent: "payload-merge",
		Payload: map[string]interface{}{
			"keep":  "value",
			"drop":  "x",
			"count": 1,
		},
		Status: TaskQueued,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	merged, ok, err := q.MergePayload(task.TaskID, map[string]interface{}{
		"count":      2,
		"last_error": "boom",
		"drop":       nil,
	})
	if err != nil {
		t.Fatalf("merge payload: %v", err)
	}
	if !ok {
		t.Fatalf("expected merge payload success")
	}
	if merged.Payload["count"] != 2 {
		t.Fatalf("expected count=2, got=%v", merged.Payload["count"])
	}
	if merged.Payload["last_error"] != "boom" {
		t.Fatalf("expected last_error, got=%v", merged.Payload["last_error"])
	}
	if _, exists := merged.Payload["drop"]; exists {
		t.Fatalf("expected drop key removed, payload=%+v", merged.Payload)
	}
}

func TestQueueRetryTask(t *testing.T) {
	tmp := t.TempDir()
	q := NewQueue(filepath.Join(tmp, "tasks.jsonl"))
	task, err := q.Enqueue(RuntimeTask{
		Source:   "nl",
		Intent:   "retry-task",
		Status:   TaskFailed,
		Retry:    0,
		MaxRetry: 2,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	retried, ok, err := q.RetryTask(task.TaskID)
	if err != nil {
		t.Fatalf("retry task: %v", err)
	}
	if !ok {
		t.Fatalf("expected retry task success")
	}
	if retried.Status != TaskQueued {
		t.Fatalf("expected queued after retry, got=%s", retried.Status)
	}
	if retried.Retry != 1 {
		t.Fatalf("expected retry increment to 1, got=%d", retried.Retry)
	}
}

func TestQueueCancelTask(t *testing.T) {
	tmp := t.TempDir()
	q := NewQueue(filepath.Join(tmp, "tasks.jsonl"))
	task, err := q.Enqueue(RuntimeTask{
		Source: "nl",
		Intent: "cancel-task",
		Status: TaskQueued,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	canceled, ok, err := q.CancelTask(task.TaskID)
	if err != nil {
		t.Fatalf("cancel task: %v", err)
	}
	if !ok {
		t.Fatalf("expected cancel task success")
	}
	if canceled.Status != TaskCanceled {
		t.Fatalf("expected canceled status, got=%s", canceled.Status)
	}
}
