package health

import (
	"context"
	"time"
)

type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

type CheckFunc func(ctx context.Context) error

type Report struct {
	Status       Status
	CheckedAt    time.Time
	BackendProbe string
	Failure      string
}

type Probe struct {
	checkBackend CheckFunc
}

func NewProbe(checkBackend CheckFunc) *Probe {
	return &Probe{checkBackend: checkBackend}
}

func (p *Probe) Check(ctx context.Context) Report {
	report := Report{
		Status:    StatusHealthy,
		CheckedAt: time.Now().UTC(),
	}

	if p.checkBackend == nil {
		report.Status = StatusDegraded
		report.BackendProbe = "not_configured"
		report.Failure = "backend probe is not configured"
		return report
	}

	if err := p.checkBackend(ctx); err != nil {
		report.Status = StatusDegraded
		report.BackendProbe = "failed"
		report.Failure = err.Error()
		return report
	}

	report.BackendProbe = "ok"
	return report
}

