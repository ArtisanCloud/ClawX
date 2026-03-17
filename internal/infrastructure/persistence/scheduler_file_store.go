package persistence

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	schedulerdomain "clawx/internal/domain/scheduler"
)

type SchedulerFileStore struct {
	workspaceRoot string
	mu            sync.Mutex
}

type schedulerJobsSnapshot struct {
	Version int                   `json:"version"`
	Jobs    []schedulerdomain.Job `json:"jobs"`
}

func NewSchedulerFileStore(workspaceRoot string) (*SchedulerFileStore, error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return nil, fmt.Errorf("workspace root is required")
	}
	return &SchedulerFileStore{workspaceRoot: filepath.Clean(root)}, nil
}

func (s *SchedulerFileStore) Save(_ context.Context, job schedulerdomain.Job) (schedulerdomain.Job, error) {
	job = job.Normalized()
	if !job.Scope.Valid() || job.Name == "" || job.ScheduleExpr == "" || job.TaskType == "" {
		return schedulerdomain.Job{}, fmt.Errorf("invalid scheduler job")
	}
	if strings.TrimSpace(job.JobID) == "" {
		job.JobID = fmt.Sprintf("job-%d", time.Now().UTC().UnixNano())
	}
	path, err := s.jobsPath(job.Scope)
	if err != nil {
		return schedulerdomain.Job{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.readJobs(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return schedulerdomain.Job{}, err
	}
	if errors.Is(err, os.ErrNotExist) {
		snapshot = schedulerJobsSnapshot{Version: 1, Jobs: make([]schedulerdomain.Job, 0)}
	}

	replaced := false
	for i := range snapshot.Jobs {
		if strings.TrimSpace(snapshot.Jobs[i].JobID) == job.JobID {
			snapshot.Jobs[i] = job
			replaced = true
			break
		}
	}
	if !replaced {
		snapshot.Jobs = append(snapshot.Jobs, job)
	}
	sort.Slice(snapshot.Jobs, func(i, j int) bool { return snapshot.Jobs[i].CreatedAt.Before(snapshot.Jobs[j].CreatedAt) })
	if err := s.writeJobs(path, snapshot); err != nil {
		return schedulerdomain.Job{}, err
	}
	return job, nil
}

func (s *SchedulerFileStore) GetByID(_ context.Context, scope schedulerdomain.Scope, jobID string) (schedulerdomain.Job, error) {
	scope = scope.Normalize()
	jobID = strings.TrimSpace(jobID)
	if !scope.Valid() || jobID == "" {
		return schedulerdomain.Job{}, fmt.Errorf("invalid scheduler scope")
	}
	path, err := s.jobsPath(scope)
	if err != nil {
		return schedulerdomain.Job{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, err := s.readJobs(path)
	if err != nil {
		return schedulerdomain.Job{}, err
	}
	for _, item := range snapshot.Jobs {
		if strings.TrimSpace(item.JobID) == jobID {
			return item, nil
		}
	}
	return schedulerdomain.Job{}, os.ErrNotExist
}

func (s *SchedulerFileStore) GetByName(_ context.Context, scope schedulerdomain.Scope, name string) (schedulerdomain.Job, error) {
	scope = scope.Normalize()
	name = strings.TrimSpace(strings.ToLower(name))
	if !scope.Valid() || name == "" {
		return schedulerdomain.Job{}, fmt.Errorf("invalid scheduler scope")
	}
	path, err := s.jobsPath(scope)
	if err != nil {
		return schedulerdomain.Job{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, err := s.readJobs(path)
	if err != nil {
		return schedulerdomain.Job{}, err
	}
	for _, item := range snapshot.Jobs {
		if strings.TrimSpace(strings.ToLower(item.Name)) == name {
			return item, nil
		}
	}
	return schedulerdomain.Job{}, os.ErrNotExist
}

func (s *SchedulerFileStore) List(_ context.Context, scope schedulerdomain.Scope) ([]schedulerdomain.Job, error) {
	scope = scope.Normalize()
	if !scope.Valid() {
		return nil, fmt.Errorf("invalid scheduler scope")
	}
	path, err := s.jobsPath(scope)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, err := s.readJobs(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []schedulerdomain.Job{}, nil
		}
		return nil, err
	}
	items := make([]schedulerdomain.Job, 0, len(snapshot.Jobs))
	for _, item := range snapshot.Jobs {
		if item.Status == schedulerdomain.JobStatusRemoved {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) })
	return items, nil
}

func (s *SchedulerFileStore) ListAll(_ context.Context) ([]schedulerdomain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	projects, err := os.ReadDir(s.workspaceRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []schedulerdomain.Job{}, nil
		}
		return nil, err
	}
	all := make([]schedulerdomain.Job, 0)
	for _, project := range projects {
		if !project.IsDir() {
			continue
		}
		agentsDir := filepath.Join(s.workspaceRoot, project.Name(), ".agents")
		agents, err := os.ReadDir(agentsDir)
		if err != nil {
			continue
		}
		for _, agent := range agents {
			if !agent.IsDir() {
				continue
			}
			path := filepath.Join(agentsDir, agent.Name(), "scheduler", "jobs.json")
			snapshot, err := s.readJobs(path)
			if err != nil {
				continue
			}
			for _, item := range snapshot.Jobs {
				if item.Status == schedulerdomain.JobStatusRemoved {
					continue
				}
				all = append(all, item)
			}
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Scope.ProjectID == all[j].Scope.ProjectID {
			if all[i].Scope.AgentID == all[j].Scope.AgentID {
				return all[i].Name < all[j].Name
			}
			return all[i].Scope.AgentID < all[j].Scope.AgentID
		}
		return all[i].Scope.ProjectID < all[j].Scope.ProjectID
	})
	return all, nil
}

func (s *SchedulerFileStore) Delete(_ context.Context, scope schedulerdomain.Scope, nameOrID string) error {
	scope = scope.Normalize()
	nameOrID = strings.TrimSpace(strings.ToLower(nameOrID))
	if !scope.Valid() || nameOrID == "" {
		return fmt.Errorf("invalid scheduler scope")
	}
	path, err := s.jobsPath(scope)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, err := s.readJobs(path)
	if err != nil {
		return err
	}
	idx := -1
	for i := range snapshot.Jobs {
		if strings.TrimSpace(strings.ToLower(snapshot.Jobs[i].JobID)) == nameOrID || strings.TrimSpace(strings.ToLower(snapshot.Jobs[i].Name)) == nameOrID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return os.ErrNotExist
	}
	snapshot.Jobs[idx].Status = schedulerdomain.JobStatusRemoved
	snapshot.Jobs[idx].UpdatedAt = time.Now().UTC()
	return s.writeJobs(path, snapshot)
}

func (s *SchedulerFileStore) Append(_ context.Context, scope schedulerdomain.Scope, run schedulerdomain.RunRecord) error {
	scope = scope.Normalize()
	if !scope.Valid() || strings.TrimSpace(run.JobID) == "" {
		return fmt.Errorf("invalid scheduler run")
	}
	path, err := s.runsPath(scope, run.JobID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	payload, err := json.Marshal(run)
	if err != nil {
		return err
	}
	_, err = f.Write(append(payload, '\n'))
	return err
}

func (s *SchedulerFileStore) RecentByJob(_ context.Context, scope schedulerdomain.Scope, jobID string, limit int) ([]schedulerdomain.RunRecord, error) {
	scope = scope.Normalize()
	jobID = strings.TrimSpace(jobID)
	if !scope.Valid() || jobID == "" {
		return nil, fmt.Errorf("invalid scheduler run query")
	}
	if limit <= 0 {
		limit = 20
	}
	path, err := s.runsPath(scope, jobID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []schedulerdomain.RunRecord{}, nil
		}
		return nil, err
	}
	defer f.Close()
	runs := make([]schedulerdomain.RunRecord, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec schedulerdomain.RunRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		runs = append(runs, rec)
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(runs) > limit {
		runs = runs[len(runs)-limit:]
	}
	return runs, nil
}

func (s *SchedulerFileStore) jobsPath(scope schedulerdomain.Scope) (string, error) {
	scope = scope.Normalize()
	if !scope.Valid() {
		return "", fmt.Errorf("invalid scheduler scope")
	}
	return filepath.Join(s.workspaceRoot, scope.ProjectID, ".agents", scope.AgentID, "scheduler", "jobs.json"), nil
}

func (s *SchedulerFileStore) runsPath(scope schedulerdomain.Scope, jobID string) (string, error) {
	scope = scope.Normalize()
	if !scope.Valid() {
		return "", fmt.Errorf("invalid scheduler scope")
	}
	name := strings.TrimSpace(strings.ToLower(jobID))
	if name == "" {
		return "", fmt.Errorf("invalid job id")
	}
	return filepath.Join(s.workspaceRoot, scope.ProjectID, ".agents", scope.AgentID, "scheduler", "runs", name+".jsonl"), nil
}

func (s *SchedulerFileStore) readJobs(path string) (schedulerJobsSnapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return schedulerJobsSnapshot{}, err
	}
	defer f.Close()
	var snapshot schedulerJobsSnapshot
	if err := json.NewDecoder(f).Decode(&snapshot); err != nil {
		return schedulerJobsSnapshot{}, err
	}
	if snapshot.Jobs == nil {
		snapshot.Jobs = make([]schedulerdomain.Job, 0)
	}
	if snapshot.Version == 0 {
		snapshot.Version = 1
	}
	return snapshot, nil
}

func (s *SchedulerFileStore) writeJobs(path string, snapshot schedulerJobsSnapshot) error {
	if snapshot.Version == 0 {
		snapshot.Version = 1
	}
	if snapshot.Jobs == nil {
		snapshot.Jobs = make([]schedulerdomain.Job, 0)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
