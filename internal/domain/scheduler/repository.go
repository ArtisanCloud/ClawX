package scheduler

import "context"

type JobRepository interface {
	Save(context.Context, Job) (Job, error)
	GetByID(context.Context, Scope, string) (Job, error)
	GetByName(context.Context, Scope, string) (Job, error)
	List(context.Context, Scope) ([]Job, error)
	ListAll(context.Context) ([]Job, error)
	Delete(context.Context, Scope, string) error
}

type RunRepository interface {
	Append(context.Context, Scope, RunRecord) error
	RecentByJob(context.Context, Scope, string, int) ([]RunRecord, error)
}
