package scheduler

import (
	"strings"
	"time"
)

type JobStatus string

const (
	JobStatusActive  JobStatus = "active"
	JobStatusPaused  JobStatus = "paused"
	JobStatusRemoved JobStatus = "removed"
)

type RunResult string

const (
	RunResultSuccess RunResult = "success"
	RunResultFailed  RunResult = "failed"
	RunResultSkipped RunResult = "skipped"
)

type TriggerMode string

const (
	TriggerScheduled TriggerMode = "scheduled"
	TriggerManual    TriggerMode = "manual"
)

type Scope struct {
	ProjectID  string `json:"project_id"`
	AgentID    string `json:"agent_id"`
	RouteScope string `json:"route_scope,omitempty"`
}

func (s Scope) Normalize() Scope {
	normalize := func(v string) string {
		value := strings.TrimSpace(strings.ToLower(v))
		if value == "" {
			return ""
		}
		var b strings.Builder
		b.Grow(len(value))
		for _, r := range value {
			switch {
			case r >= 'a' && r <= 'z':
				b.WriteRune(r)
			case r >= '0' && r <= '9':
				b.WriteRune(r)
			case r == '-' || r == '_' || r == ':':
				b.WriteRune(r)
			}
		}
		return strings.Trim(b.String(), "-_")
	}
	out := Scope{ProjectID: normalize(s.ProjectID), AgentID: normalize(s.AgentID), RouteScope: strings.TrimSpace(s.RouteScope)}
	if out.RouteScope == "" {
		out.RouteScope = "global"
	}
	return out
}

func (s Scope) Valid() bool {
	n := s.Normalize()
	return n.ProjectID != "" && n.AgentID != ""
}

type Job struct {
	JobID        string            `json:"job_id"`
	Name         string            `json:"name"`
	Scope        Scope             `json:"scope"`
	ScheduleExpr string            `json:"schedule_expr"`
	TaskType     string            `json:"task_type"`
	TaskArgs     map[string]string `json:"task_args,omitempty"`
	Status       JobStatus         `json:"status"`
	NextRunAt    time.Time         `json:"next_run_at"`
	LastRunAt    time.Time         `json:"last_run_at,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

func (j Job) Normalized() Job {
	j.JobID = strings.TrimSpace(j.JobID)
	j.Name = strings.TrimSpace(strings.ToLower(j.Name))
	j.Scope = j.Scope.Normalize()
	j.ScheduleExpr = strings.TrimSpace(j.ScheduleExpr)
	j.TaskType = strings.TrimSpace(strings.ToLower(j.TaskType))
	if j.TaskArgs == nil {
		j.TaskArgs = map[string]string{}
	}
	for k, v := range j.TaskArgs {
		key := strings.TrimSpace(strings.ToLower(k))
		if key == "" {
			delete(j.TaskArgs, k)
			continue
		}
		j.TaskArgs[key] = strings.TrimSpace(v)
		if key != k {
			delete(j.TaskArgs, k)
		}
	}
	if j.Status == "" {
		j.Status = JobStatusActive
	}
	return j
}

type RunRecord struct {
	RunID        string      `json:"run_id"`
	JobID        string      `json:"job_id"`
	StartedAt    time.Time   `json:"started_at"`
	EndedAt      time.Time   `json:"ended_at,omitempty"`
	Result       RunResult   `json:"result"`
	ErrorCode    string      `json:"error_code,omitempty"`
	ErrorMessage string      `json:"error_message,omitempty"`
	Summary      string      `json:"summary,omitempty"`
	TriggerMode  TriggerMode `json:"trigger_mode"`
}
