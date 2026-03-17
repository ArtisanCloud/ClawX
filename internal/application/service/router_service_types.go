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
