package service

import (
	"context"
	"time"
)

type ServiceCommandService interface {
	Start(context.Context, ServiceStartInput) (ServiceStartResult, error)
	Stop(context.Context, ServiceStopInput) (ServiceStopResult, error)
	Status(context.Context, ServiceStatusInput) (ServiceStatusResult, error)
	Logs(context.Context, ServiceLogsInput) (ServiceLogsResult, error)
}

type ServiceStartInput struct {
	ProjectID string
	AgentID   string
	RouteKey  string
	Name      string
	Command   []string
	CWD       string
}

type ServiceStartResult struct {
	Name      string
	PID       int
	LogPath   string
	StartedAt time.Time
}

type ServiceStopInput struct {
	ProjectID string
	AgentID   string
	Name      string
}

type ServiceStopResult struct {
	Name      string
	Stopped   bool
	StoppedAt time.Time
}

type ServiceStatusInput struct {
	ProjectID string
	AgentID   string
	Name      string
}

type ServiceRuntimeStatus struct {
	Name      string
	Command   []string
	CWD       string
	PID       int
	LogPath   string
	Status    string
	StartedAt time.Time
}

type ServiceStatusResult struct {
	Services []ServiceRuntimeStatus
}

type ServiceLogsInput struct {
	ProjectID string
	AgentID   string
	Name      string
	Tail      int
}

type ServiceLogsResult struct {
	Name    string
	LogPath string
	Content string
}

type ScheduleCommandService interface {
	Add(context.Context, ScheduleAddInput) (ScheduleJobResult, error)
	List(context.Context, ScheduleListInput) (ScheduleListResult, error)
	Status(context.Context, ScheduleStatusInput) (ScheduleStatusResult, error)
	Pause(context.Context, ScheduleUpdateInput) (ScheduleJobResult, error)
	Resume(context.Context, ScheduleUpdateInput) (ScheduleJobResult, error)
	RunNow(context.Context, ScheduleUpdateInput) (ScheduleRunNowResult, error)
	Remove(context.Context, ScheduleUpdateInput) (ScheduleJobResult, error)
}

type ScheduleScopeInput struct {
	ProjectID  string
	AgentID    string
	RouteScope string
}

type ScheduleAddInput struct {
	Scope        ScheduleScopeInput
	Name         string
	ScheduleExpr string
	TaskType     string
	TaskArgs     map[string]string
	RequestedBy  string
}

type ScheduleListInput struct {
	Scope ScheduleScopeInput
}

type ScheduleStatusInput struct {
	Scope    ScheduleScopeInput
	NameOrID string
}

type ScheduleUpdateInput struct {
	Scope    ScheduleScopeInput
	NameOrID string
}

type ScheduleJobResult struct {
	JobID         string
	Name          string
	Status        string
	ScheduleExpr  string
	TaskType      string
	TaskArgs      map[string]string
	NextRunAt     time.Time
	LastRunAt     time.Time
	LastRunResult string
	Timezone      string
}

type ScheduleListResult struct {
	Jobs []ScheduleJobResult
}

type ScheduleStatusResult struct {
	Job        ScheduleJobResult
	LastError  string
	LastReason string
}

type ScheduleRunNowResult struct {
	JobID      string
	Name       string
	Status     string
	Result     string
	Summary    string
	StartedAt  time.Time
	FinishedAt time.Time
}
