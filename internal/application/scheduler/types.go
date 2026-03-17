package scheduler

import (
	"context"
	"time"

	schedulerdomain "clawx/internal/domain/scheduler"
)

type Job = schedulerdomain.Job

type RunRecord = schedulerdomain.RunRecord

type Scope = schedulerdomain.Scope

type JobRepository interface {
	schedulerdomain.JobRepository
}

type RunRepository interface {
	schedulerdomain.RunRepository
}

type NowFunc func() time.Time

type runnerState struct {
	jobID string
	pid   int
}

type Runner interface {
	Tick(context.Context) error
}
