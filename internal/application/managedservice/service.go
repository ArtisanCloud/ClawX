package managedservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	appservice "clawx/internal/application/service"
)

var (
	ErrServiceAlreadyRunning = errors.New("service already running")
	ErrServiceNotFound       = errors.New("service not found")
)

type Service struct {
	mu            sync.Mutex
	workspaceRoot string
	now           func() time.Time
	running       map[string]*exec.Cmd
}

type record struct {
	Name      string    `json:"name"`
	ProjectID string    `json:"project_id"`
	AgentID   string    `json:"agent_id"`
	RouteKey  string    `json:"route_key,omitempty"`
	Command   []string  `json:"command"`
	CWD       string    `json:"cwd"`
	PID       int       `json:"pid"`
	LogPath   string    `json:"log_path"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	StoppedAt time.Time `json:"stopped_at,omitempty"`
}

func NewService(workspaceRoot string) (*Service, error) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return nil, fmt.Errorf("workspace root is required")
	}
	return &Service{
		workspaceRoot: filepath.Clean(workspaceRoot),
		now:           func() time.Time { return time.Now().UTC() },
		running:       make(map[string]*exec.Cmd),
	}, nil
}

func (s *Service) Start(_ context.Context, input appservice.ServiceStartInput) (appservice.ServiceStartResult, error) {
	input.ProjectID = normalizeSegment(input.ProjectID)
	input.AgentID = normalizeSegment(input.AgentID)
	input.Name = normalizeSegment(input.Name)
	if input.ProjectID == "" || input.AgentID == "" || input.Name == "" {
		return appservice.ServiceStartResult{}, fmt.Errorf("invalid start input")
	}
	if len(input.Command) == 0 || strings.TrimSpace(input.Command[0]) == "" {
		return appservice.ServiceStartResult{}, fmt.Errorf("service command is required")
	}
	if strings.TrimSpace(input.CWD) == "" {
		input.CWD = filepath.Join(s.workspaceRoot, input.ProjectID, ".agents", input.AgentID, "workspace")
	}

	key := serviceKey(input.ProjectID, input.AgentID, input.Name)
	s.mu.Lock()
	if _, ok := s.running[key]; ok {
		s.mu.Unlock()
		return appservice.ServiceStartResult{}, ErrServiceAlreadyRunning
	}
	s.mu.Unlock()

	metaPath := s.metaPath(input.ProjectID, input.AgentID, input.Name)
	logPath := s.logPath(input.ProjectID, input.AgentID, input.Name)
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		return appservice.ServiceStartResult{}, fmt.Errorf("create service state dir: %w", err)
	}
	if err := os.MkdirAll(strings.TrimSpace(input.CWD), 0o755); err != nil {
		return appservice.ServiceStartResult{}, fmt.Errorf("create service cwd: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return appservice.ServiceStartResult{}, fmt.Errorf("open service log: %w", err)
	}

	cmd := exec.Command(input.Command[0], input.Command[1:]...)
	cmd.Dir = input.CWD
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return appservice.ServiceStartResult{}, fmt.Errorf("start service process: %w", err)
	}

	startedAt := s.now()
	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	rec := record{
		Name:      input.Name,
		ProjectID: input.ProjectID,
		AgentID:   input.AgentID,
		RouteKey:  strings.TrimSpace(input.RouteKey),
		Command:   append([]string(nil), input.Command...),
		CWD:       input.CWD,
		PID:       pid,
		LogPath:   logPath,
		Status:    "running",
		StartedAt: startedAt,
	}
	if err := writeRecord(metaPath, rec); err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = logFile.Close()
		return appservice.ServiceStartResult{}, err
	}

	s.mu.Lock()
	s.running[key] = cmd
	s.mu.Unlock()

	go func() {
		waitErr := cmd.Wait()
		_ = logFile.Close()
		s.mu.Lock()
		delete(s.running, key)
		s.mu.Unlock()

		stopped := rec
		stopped.Status = "stopped"
		stopped.StoppedAt = s.now()
		if waitErr != nil {
			stopped.Status = "failed"
		}
		_ = writeRecord(metaPath, stopped)
	}()

	return appservice.ServiceStartResult{
		Name:      input.Name,
		PID:       pid,
		LogPath:   logPath,
		StartedAt: startedAt,
	}, nil
}

func (s *Service) Stop(ctx context.Context, input appservice.ServiceStopInput) (appservice.ServiceStopResult, error) {
	input.ProjectID = normalizeSegment(input.ProjectID)
	input.AgentID = normalizeSegment(input.AgentID)
	input.Name = normalizeSegment(input.Name)
	if input.ProjectID == "" || input.AgentID == "" || input.Name == "" {
		return appservice.ServiceStopResult{}, fmt.Errorf("invalid stop input")
	}

	key := serviceKey(input.ProjectID, input.AgentID, input.Name)
	metaPath := s.metaPath(input.ProjectID, input.AgentID, input.Name)

	s.mu.Lock()
	cmd := s.running[key]
	s.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		rec, err := readRecord(metaPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return appservice.ServiceStopResult{}, ErrServiceNotFound
			}
			return appservice.ServiceStopResult{}, err
		}
		if rec.Status == "running" {
			rec.Status = "stopped"
			rec.StoppedAt = s.now()
			_ = writeRecord(metaPath, rec)
		}
		return appservice.ServiceStopResult{Name: input.Name, Stopped: false}, nil
	}

	_ = cmd.Process.Signal(os.Interrupt)
	select {
	case <-ctx.Done():
		_ = cmd.Process.Kill()
	case <-time.After(1500 * time.Millisecond):
		_ = cmd.Process.Kill()
	}

	rec, err := readRecord(metaPath)
	if err == nil {
		rec.Status = "stopped"
		rec.StoppedAt = s.now()
		_ = writeRecord(metaPath, rec)
	}
	return appservice.ServiceStopResult{Name: input.Name, Stopped: true, StoppedAt: s.now()}, nil
}

func (s *Service) Status(_ context.Context, input appservice.ServiceStatusInput) (appservice.ServiceStatusResult, error) {
	input.ProjectID = normalizeSegment(input.ProjectID)
	input.AgentID = normalizeSegment(input.AgentID)
	input.Name = normalizeSegment(input.Name)
	if input.ProjectID == "" || input.AgentID == "" {
		return appservice.ServiceStatusResult{}, fmt.Errorf("invalid status input")
	}

	servicesDir := s.servicesDir(input.ProjectID, input.AgentID)
	entries, err := os.ReadDir(servicesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return appservice.ServiceStatusResult{}, nil
		}
		return appservice.ServiceStatusResult{}, fmt.Errorf("read services dir: %w", err)
	}

	result := make([]appservice.ServiceRuntimeStatus, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		if input.Name != "" && input.Name != name {
			continue
		}
		rec, err := readRecord(filepath.Join(servicesDir, entry.Name()))
		if err != nil {
			continue
		}
		key := serviceKey(input.ProjectID, input.AgentID, name)
		s.mu.Lock()
		cmd := s.running[key]
		s.mu.Unlock()
		status := rec.Status
		pid := rec.PID
		if cmd != nil && cmd.Process != nil {
			status = "running"
			pid = cmd.Process.Pid
		} else if status == "running" {
			status = "stopped"
		}
		result = append(result, appservice.ServiceRuntimeStatus{
			Name:      name,
			Command:   rec.Command,
			CWD:       rec.CWD,
			PID:       pid,
			LogPath:   rec.LogPath,
			Status:    status,
			StartedAt: rec.StartedAt,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return appservice.ServiceStatusResult{Services: result}, nil
}

func (s *Service) Logs(_ context.Context, input appservice.ServiceLogsInput) (appservice.ServiceLogsResult, error) {
	input.ProjectID = normalizeSegment(input.ProjectID)
	input.AgentID = normalizeSegment(input.AgentID)
	input.Name = normalizeSegment(input.Name)
	if input.ProjectID == "" || input.AgentID == "" || input.Name == "" {
		return appservice.ServiceLogsResult{}, fmt.Errorf("invalid logs input")
	}
	if input.Tail <= 0 {
		input.Tail = 100
	}

	logPath := s.logPath(input.ProjectID, input.AgentID, input.Name)
	content, err := tailFile(logPath, input.Tail)
	if err != nil {
		if os.IsNotExist(err) {
			return appservice.ServiceLogsResult{}, ErrServiceNotFound
		}
		return appservice.ServiceLogsResult{}, err
	}
	return appservice.ServiceLogsResult{
		Name:    input.Name,
		LogPath: logPath,
		Content: content,
	}, nil
}

func (s *Service) servicesDir(projectID, agentID string) string {
	return filepath.Join(s.workspaceRoot, projectID, ".agents", agentID, "services")
}

func (s *Service) metaPath(projectID, agentID, name string) string {
	return filepath.Join(s.servicesDir(projectID, agentID), name+".json")
}

func (s *Service) logPath(projectID, agentID, name string) string {
	return filepath.Join(s.servicesDir(projectID, agentID), name+".log")
}

func serviceKey(projectID, agentID, name string) string {
	return projectID + "::" + agentID + "::" + name
}

func normalizeSegment(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
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
		case r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-_")
}

func writeRecord(path string, rec record) error {
	payload, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal service record: %w", err)
	}
	payload = append(payload, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".service-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp service record: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp service record: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp service record: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp service record: %w", err)
	}
	cleanup = false
	return nil
}

func readRecord(path string) (record, error) {
	file, err := os.Open(path)
	if err != nil {
		return record{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var rec record
	if err := decoder.Decode(&rec); err != nil {
		return record{}, fmt.Errorf("parse service record: %w", err)
	}
	if decoder.More() {
		return record{}, fmt.Errorf("parse service record: trailing data")
	}
	return rec, nil
}

func tailFile(path string, lines int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	raw, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("read service log: %w", err)
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	parts := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(parts) <= lines {
		return strings.Join(parts, "\n"), nil
	}
	return strings.Join(parts[len(parts)-lines:], "\n"), nil
}
