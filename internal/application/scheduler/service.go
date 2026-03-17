package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	appservice "clawx/internal/application/service"
	schedulerdomain "clawx/internal/domain/scheduler"
)

type Service struct {
	jobs          JobRepository
	runs          RunRepository
	workspaceRoot string
	now           NowFunc
	mu            sync.Mutex
	running       map[string]struct{}
}

func NewService(workspaceRoot string, jobs JobRepository, runs RunRepository) (*Service, error) {
	if strings.TrimSpace(workspaceRoot) == "" {
		return nil, fmt.Errorf("workspace root is required")
	}
	if jobs == nil || runs == nil {
		return nil, fmt.Errorf("repositories are required")
	}
	return &Service{jobs: jobs, runs: runs, workspaceRoot: strings.TrimSpace(workspaceRoot), now: func() time.Time { return time.Now().UTC() }, running: map[string]struct{}{}}, nil
}

func (s *Service) Add(ctx context.Context, input appservice.ScheduleAddInput) (appservice.ScheduleJobResult, error) {
	scope, err := s.scopeFromInput(input.Scope)
	if err != nil {
		return appservice.ScheduleJobResult{}, err
	}
	name := strings.TrimSpace(strings.ToLower(input.Name))
	if name == "" || strings.TrimSpace(input.ScheduleExpr) == "" || strings.TrimSpace(input.TaskType) == "" {
		return appservice.ScheduleJobResult{}, newCommandError("invalid_schedule_command", "missing required fields", mapNextAction("invalid_schedule_command"))
	}
	if _, err := parseCron(input.ScheduleExpr); err != nil {
		return appservice.ScheduleJobResult{}, newCommandError("invalid_schedule_command", err.Error(), mapNextAction("invalid_schedule_command"))
	}
	if existing, err := s.jobs.GetByName(ctx, scope, name); err == nil && existing.Status != schedulerdomain.JobStatusRemoved {
		return appservice.ScheduleJobResult{}, newCommandError("schedule_job_conflict", "job already exists", mapNextAction("schedule_job_conflict"))
	}
	args := map[string]string{}
	for k, v := range input.TaskArgs {
		args[strings.TrimSpace(strings.ToLower(k))] = strings.TrimSpace(v)
	}
	if strings.TrimSpace(strings.ToLower(input.TaskType)) == "image.cleanup" {
		if strings.TrimSpace(args["retention_days"]) == "" {
			args["retention_days"] = "30"
		}
	}
	next, err := nextRunFromExpr(input.ScheduleExpr, s.now(), time.UTC)
	if err != nil {
		return appservice.ScheduleJobResult{}, newCommandError("invalid_schedule_command", err.Error(), mapNextAction("invalid_schedule_command"))
	}
	now := s.now().UTC()
	job := Job{JobID: fmt.Sprintf("job-%d", now.UnixNano()), Name: name, Scope: scope, ScheduleExpr: strings.TrimSpace(input.ScheduleExpr), TaskType: strings.TrimSpace(strings.ToLower(input.TaskType)), TaskArgs: args, Status: schedulerdomain.JobStatusActive, NextRunAt: next, CreatedAt: now, UpdatedAt: now}
	saved, err := s.jobs.Save(ctx, job)
	if err != nil {
		return appservice.ScheduleJobResult{}, err
	}
	return s.toJobResult(ctx, saved), nil
}

func (s *Service) List(ctx context.Context, input appservice.ScheduleListInput) (appservice.ScheduleListResult, error) {
	scope, err := s.scopeFromInput(input.Scope)
	if err != nil {
		return appservice.ScheduleListResult{}, err
	}
	jobs, err := s.jobs.List(ctx, scope)
	if err != nil {
		return appservice.ScheduleListResult{}, err
	}
	result := make([]appservice.ScheduleJobResult, 0, len(jobs))
	for _, item := range jobs {
		result = append(result, s.toJobResult(ctx, item))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return appservice.ScheduleListResult{Jobs: result}, nil
}

func (s *Service) Status(ctx context.Context, input appservice.ScheduleStatusInput) (appservice.ScheduleStatusResult, error) {
	job, err := s.resolveJob(ctx, input.Scope, input.NameOrID)
	if err != nil {
		return appservice.ScheduleStatusResult{}, err
	}
	runs, _ := s.runs.RecentByJob(ctx, job.Scope, job.JobID, 1)
	status := appservice.ScheduleStatusResult{Job: s.toJobResult(ctx, job)}
	if len(runs) > 0 {
		status.LastError = strings.TrimSpace(runs[len(runs)-1].ErrorMessage)
		status.LastReason = strings.TrimSpace(runs[len(runs)-1].Summary)
	}
	return status, nil
}

func (s *Service) Pause(ctx context.Context, input appservice.ScheduleUpdateInput) (appservice.ScheduleJobResult, error) {
	job, err := s.resolveJob(ctx, input.Scope, input.NameOrID)
	if err != nil {
		return appservice.ScheduleJobResult{}, err
	}
	job.Status = schedulerdomain.JobStatusPaused
	job.UpdatedAt = s.now().UTC()
	saved, err := s.jobs.Save(ctx, job)
	if err != nil {
		return appservice.ScheduleJobResult{}, err
	}
	return s.toJobResult(ctx, saved), nil
}

func (s *Service) Resume(ctx context.Context, input appservice.ScheduleUpdateInput) (appservice.ScheduleJobResult, error) {
	job, err := s.resolveJob(ctx, input.Scope, input.NameOrID)
	if err != nil {
		return appservice.ScheduleJobResult{}, err
	}
	job.Status = schedulerdomain.JobStatusActive
	next, err := nextRunFromExpr(job.ScheduleExpr, s.now(), time.UTC)
	if err != nil {
		return appservice.ScheduleJobResult{}, err
	}
	job.NextRunAt = next
	job.UpdatedAt = s.now().UTC()
	saved, err := s.jobs.Save(ctx, job)
	if err != nil {
		return appservice.ScheduleJobResult{}, err
	}
	return s.toJobResult(ctx, saved), nil
}

func (s *Service) Remove(ctx context.Context, input appservice.ScheduleUpdateInput) (appservice.ScheduleJobResult, error) {
	job, err := s.resolveJob(ctx, input.Scope, input.NameOrID)
	if err != nil {
		return appservice.ScheduleJobResult{}, err
	}
	job.Status = schedulerdomain.JobStatusRemoved
	job.UpdatedAt = s.now().UTC()
	saved, err := s.jobs.Save(ctx, job)
	if err != nil {
		return appservice.ScheduleJobResult{}, err
	}
	return s.toJobResult(ctx, saved), nil
}

func (s *Service) RunNow(ctx context.Context, input appservice.ScheduleUpdateInput) (appservice.ScheduleRunNowResult, error) {
	job, err := s.resolveJob(ctx, input.Scope, input.NameOrID)
	if err != nil {
		return appservice.ScheduleRunNowResult{}, err
	}
	if job.Status == schedulerdomain.JobStatusRemoved {
		return appservice.ScheduleRunNowResult{}, newCommandError("schedule_job_not_found", "job removed", mapNextAction("schedule_job_not_found"))
	}
	run, err := s.executeJob(ctx, job, schedulerdomain.TriggerManual)
	if err != nil {
		return appservice.ScheduleRunNowResult{}, err
	}
	return appservice.ScheduleRunNowResult{JobID: job.JobID, Name: job.Name, Status: string(job.Status), Result: string(run.Result), Summary: run.Summary, StartedAt: run.StartedAt, FinishedAt: run.EndedAt}, nil
}

func (s *Service) Tick(ctx context.Context) error {
	jobs, err := s.jobs.ListAll(ctx)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	for _, job := range jobs {
		if job.Status != schedulerdomain.JobStatusActive {
			continue
		}
		if job.NextRunAt.IsZero() || now.Before(job.NextRunAt) {
			continue
		}
		_, _ = s.executeJob(ctx, job, schedulerdomain.TriggerScheduled)
	}
	return nil
}

func (s *Service) executeJob(ctx context.Context, job Job, mode schedulerdomain.TriggerMode) (RunRecord, error) {
	key := job.Scope.ProjectID + "::" + job.Scope.AgentID + "::" + job.JobID
	s.mu.Lock()
	if _, ok := s.running[key]; ok {
		s.mu.Unlock()
		return RunRecord{}, newCommandError("schedule_job_running", "job is already running", mapNextAction("schedule_job_running"))
	}
	s.running[key] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.running, key)
		s.mu.Unlock()
	}()

	now := s.now().UTC()
	run := RunRecord{RunID: fmt.Sprintf("run-%d", now.UnixNano()), JobID: job.JobID, StartedAt: now, TriggerMode: mode, Result: schedulerdomain.RunResultSuccess}
	summary := ""
	var runErr error
	switch job.TaskType {
	case "image.cleanup":
		summary, runErr = s.executeImageCleanup(job)
	default:
		runErr = newCommandError("schedule_run_failed", "unsupported task type", mapNextAction("schedule_run_failed"))
	}
	run.EndedAt = s.now().UTC()
	if runErr != nil {
		run.Result = schedulerdomain.RunResultFailed
		run.ErrorCode = "schedule_run_failed"
		run.ErrorMessage = runErr.Error()
		run.Summary = strings.TrimSpace(summary)
	} else {
		run.Result = schedulerdomain.RunResultSuccess
		run.Summary = strings.TrimSpace(summary)
	}
	_ = s.runs.Append(ctx, job.Scope, run)

	job.LastRunAt = run.EndedAt
	next, err := nextRunFromExpr(job.ScheduleExpr, run.EndedAt, time.UTC)
	if err == nil {
		job.NextRunAt = next
	}
	job.UpdatedAt = run.EndedAt
	if run.Result == schedulerdomain.RunResultFailed {
		// keep active by default; do not pause/delete on single failure.
		if job.Status == "" {
			job.Status = schedulerdomain.JobStatusActive
		}
	}
	_, _ = s.jobs.Save(ctx, job)

	if runErr != nil {
		if cerr, ok := runErr.(*CommandError); ok {
			return run, cerr
		}
		return run, newCommandError("schedule_run_failed", runErr.Error(), mapNextAction("schedule_run_failed"))
	}
	return run, nil
}

func (s *Service) resolveJob(ctx context.Context, scopeInput appservice.ScheduleScopeInput, nameOrID string) (Job, error) {
	scope, err := s.scopeFromInput(scopeInput)
	if err != nil {
		return Job{}, err
	}
	nameOrID = strings.TrimSpace(strings.ToLower(nameOrID))
	if nameOrID == "" {
		return Job{}, newCommandError("invalid_schedule_command", "name or id is required", mapNextAction("invalid_schedule_command"))
	}
	if job, err := s.jobs.GetByID(ctx, scope, nameOrID); err == nil {
		return job, nil
	}
	job, err := s.jobs.GetByName(ctx, scope, nameOrID)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return Job{}, err
		}
		return Job{}, newCommandError("schedule_job_not_found", "job not found", mapNextAction("schedule_job_not_found"))
	}
	return job, nil
}

func (s *Service) scopeFromInput(input appservice.ScheduleScopeInput) (Scope, error) {
	scope := Scope{ProjectID: input.ProjectID, AgentID: input.AgentID, RouteScope: input.RouteScope}.Normalize()
	if !scope.Valid() {
		return Scope{}, newCommandError("rejected_scope", "invalid project or agent scope", mapNextAction("rejected_scope"))
	}
	return scope, nil
}

func (s *Service) toJobResult(ctx context.Context, job Job) appservice.ScheduleJobResult {
	runs, _ := s.runs.RecentByJob(ctx, job.Scope, job.JobID, 1)
	lastResult := ""
	if len(runs) > 0 {
		lastResult = string(runs[len(runs)-1].Result)
	}
	return appservice.ScheduleJobResult{
		JobID:         job.JobID,
		Name:          job.Name,
		Status:        string(job.Status),
		ScheduleExpr:  job.ScheduleExpr,
		TaskType:      job.TaskType,
		TaskArgs:      copyArgs(job.TaskArgs),
		NextRunAt:     job.NextRunAt,
		LastRunAt:     job.LastRunAt,
		LastRunResult: lastResult,
		Timezone:      "UTC",
	}
}

func copyArgs(src map[string]string) map[string]string {
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
