package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	projectdomain "clawx/internal/domain/project"
)

type Clock func() time.Time

type Resolution struct {
	Project     projectdomain.Record
	RouteKey    string
	RoutingMode string
}

type Service struct {
	registryRepo   projectdomain.RegistryRepository
	bindingRepo    projectdomain.BindingRepository
	proposalRepo   projectdomain.ProposalRepository
	clock          Clock
	workspaceRoot  string
	defaultProject string
}

type Option func(*Service)

func WithClock(clock Clock) Option {
	return func(s *Service) {
		s.clock = clock
	}
}

func WithWorkspaceRoot(root string) Option {
	return func(s *Service) {
		s.workspaceRoot = strings.TrimSpace(root)
	}
}

func WithDefaultProjectID(projectID string) Option {
	return func(s *Service) {
		s.defaultProject = normalizeProjectID(projectID)
	}
}

func NewService(
	registryRepo projectdomain.RegistryRepository,
	bindingRepo projectdomain.BindingRepository,
	proposalRepo projectdomain.ProposalRepository,
	opts ...Option,
) *Service {
	svc := &Service{
		registryRepo:   registryRepo,
		bindingRepo:    bindingRepo,
		proposalRepo:   proposalRepo,
		clock:          func() time.Time { return time.Now().UTC() },
		workspaceRoot:  defaultWorkspaceRoot(),
		defaultProject: "main",
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	if strings.TrimSpace(svc.workspaceRoot) == "" {
		svc.workspaceRoot = defaultWorkspaceRoot()
	}
	if strings.TrimSpace(svc.defaultProject) == "" {
		svc.defaultProject = "main"
	}
	return svc
}

func (s *Service) CreateProject(ctx context.Context, projectID, name, workspacePath string) (projectdomain.Record, error) {
	if s.registryRepo == nil {
		return projectdomain.Record{}, fmt.Errorf("project registry repository is required")
	}

	projectID = normalizeProjectID(projectID)
	if projectID == "" {
		return projectdomain.Record{}, projectdomain.ErrInvalidProject
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = projectID
	}
	workspacePath = strings.TrimSpace(workspacePath)
	if workspacePath == "" {
		workspacePath = filepath.Join(s.workspaceRoot, projectID)
	}
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		return projectdomain.Record{}, fmt.Errorf("create project workspace: %w", err)
	}

	registry, changed, err := s.loadAndEnsureRegistry(ctx)
	if err != nil {
		return projectdomain.Record{}, err
	}
	if _, exists := registry.Projects[projectID]; exists {
		return projectdomain.Record{}, fmt.Errorf("project %q already exists", projectID)
	}

	now := s.clock()
	record := projectdomain.Record{
		ID:            projectID,
		Name:          name,
		WorkspacePath: workspacePath,
		Status:        projectdomain.StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	registry.Projects[projectID] = record
	registry.UpdatedAt = now
	if strings.TrimSpace(registry.DefaultProjectID) == "" {
		registry.DefaultProjectID = projectID
	}
	if err := s.registryRepo.Save(ctx, registry); err != nil {
		return projectdomain.Record{}, err
	}
	_ = changed
	return record, nil
}

func (s *Service) ListProjects(ctx context.Context) ([]projectdomain.Record, error) {
	registry, _, err := s.loadAndEnsureRegistry(ctx)
	if err != nil {
		return nil, err
	}
	if len(registry.Projects) == 0 {
		return nil, nil
	}
	result := make([]projectdomain.Record, 0, len(registry.Projects))
	for _, item := range registry.Projects {
		result = append(result, item)
	}
	sortProjects(result)
	return result, nil
}

func (s *Service) UseProject(ctx context.Context, routeKey, projectID, updatedBy string) (projectdomain.RouteBinding, error) {
	if s.bindingRepo == nil {
		return projectdomain.RouteBinding{}, fmt.Errorf("project binding repository is required")
	}
	routeKey = normalizeRouteKey(routeKey)
	projectID = normalizeProjectID(projectID)
	if routeKey == "" || projectID == "" {
		return projectdomain.RouteBinding{}, projectdomain.ErrInvalidProject
	}

	registry, _, err := s.loadAndEnsureRegistry(ctx)
	if err != nil {
		return projectdomain.RouteBinding{}, err
	}
	record, ok := registry.Projects[projectID]
	if !ok {
		return projectdomain.RouteBinding{}, projectdomain.ErrProjectNotFound
	}
	if record.Status == projectdomain.StatusBroken {
		return projectdomain.RouteBinding{}, projectdomain.ErrInvalidProject
	}

	binding := projectdomain.RouteBinding{
		RouteKey:  routeKey,
		ProjectID: projectID,
		UpdatedAt: s.clock(),
		UpdatedBy: strings.TrimSpace(updatedBy),
		Source:    projectdomain.SourceManual,
	}
	if binding.UpdatedBy == "" {
		binding.UpdatedBy = "system"
	}
	if err := s.bindingRepo.Upsert(ctx, binding); err != nil {
		return projectdomain.RouteBinding{}, err
	}
	return binding, nil
}

func (s *Service) CurrentProject(ctx context.Context, routeKey string) (Resolution, error) {
	projectID, mode, err := s.ResolveProject(ctx, routeKey)
	if err != nil {
		return Resolution{}, err
	}

	registry, _, err := s.loadAndEnsureRegistry(ctx)
	if err != nil {
		return Resolution{}, err
	}
	record, ok := registry.Projects[projectID]
	if !ok {
		return Resolution{}, projectdomain.ErrProjectNotFound
	}
	return Resolution{
		Project:     record,
		RouteKey:    normalizeRouteKey(routeKey),
		RoutingMode: mode,
	}, nil
}

func (s *Service) ResolveProject(ctx context.Context, routeKey string) (string, string, error) {
	registry, _, err := s.loadAndEnsureRegistry(ctx)
	if err != nil {
		return "", "", err
	}

	routeKey = normalizeRouteKey(routeKey)
	if routeKey != "" && s.bindingRepo != nil {
		binding, err := s.bindingRepo.GetByRouteKey(ctx, routeKey)
		if err == nil {
			projectID := normalizeProjectID(binding.ProjectID)
			record, ok := registry.Projects[projectID]
			if !ok {
				return "", "", projectdomain.ErrProjectNotFound
			}
			if record.Status == projectdomain.StatusBroken {
				return "", "", projectdomain.ErrInvalidProject
			}
			return projectID, "binding", nil
		}
		if !errors.Is(err, projectdomain.ErrBindingNotFound) {
			return "", "", err
		}
	}

	defaultID := normalizeProjectID(registry.DefaultProjectID)
	if defaultID == "" {
		defaultID = s.defaultProject
	}
	if _, ok := registry.Projects[defaultID]; !ok {
		return "", "", projectdomain.ErrProjectNotFound
	}
	return defaultID, "fallback", nil
}

func (s *Service) loadAndEnsureRegistry(ctx context.Context) (projectdomain.Registry, bool, error) {
	registry, err := s.registryRepo.Get(ctx)
	if err != nil {
		return projectdomain.Registry{}, false, err
	}
	if registry.Version <= 0 {
		registry.Version = 1
	}
	if registry.Projects == nil {
		registry.Projects = make(map[string]projectdomain.Record)
	}

	changed := false
	defaultID := normalizeProjectID(registry.DefaultProjectID)
	if defaultID == "" {
		defaultID = s.defaultProject
		registry.DefaultProjectID = defaultID
		changed = true
	}
	if _, ok := registry.Projects[defaultID]; !ok {
		workspacePath := filepath.Join(s.workspaceRoot, defaultID)
		if err := os.MkdirAll(workspacePath, 0o755); err != nil {
			return projectdomain.Registry{}, false, fmt.Errorf("create default project workspace: %w", err)
		}
		now := s.clock()
		registry.Projects[defaultID] = projectdomain.Record{
			ID:            defaultID,
			Name:          defaultID,
			WorkspacePath: workspacePath,
			Status:        projectdomain.StatusActive,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		registry.UpdatedAt = now
		changed = true
	}

	if changed {
		if err := s.registryRepo.Save(ctx, registry); err != nil {
			return projectdomain.Registry{}, false, err
		}
	}
	return registry, changed, nil
}

func normalizeProjectID(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	builder := strings.Builder{}
	builder.Grow(len(raw))
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		case r == ' ':
			builder.WriteRune('-')
		}
	}
	return strings.Trim(builder.String(), "-_")
}

func normalizeRouteKey(routeKey string) string {
	return strings.TrimSpace(routeKey)
}

func sortProjects(items []projectdomain.Record) {
	if len(items) < 2 {
		return
	}
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].ID < items[i].ID {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func defaultWorkspaceRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".clawx", "workspaces")
	}
	return filepath.Join(home, ".clawx", "workspaces")
}
