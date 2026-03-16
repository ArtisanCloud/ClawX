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
	proposalTTL    time.Duration
	sessionChecker ActiveSessionChecker
	memoryTemplate MemoryTemplateManager
}

type Option func(*Service)

type ActiveSessionChecker func(ctx context.Context, projectID string) (bool, error)

type MemoryTemplateManager interface {
	EnsureProjectTemplate(ctx context.Context, projectID, workspacePath string) error
}

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

func WithProposalTTL(ttl time.Duration) Option {
	return func(s *Service) {
		s.proposalTTL = ttl
	}
}

func WithActiveSessionChecker(checker ActiveSessionChecker) Option {
	return func(s *Service) {
		s.sessionChecker = checker
	}
}

func WithMemoryTemplateManager(manager MemoryTemplateManager) Option {
	return func(s *Service) {
		s.memoryTemplate = manager
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
		proposalTTL:    10 * time.Minute,
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
	if svc.proposalTTL <= 0 {
		svc.proposalTTL = 10 * time.Minute
	}
	return svc
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
		if err := s.ensureMemoryTemplate(ctx, defaultID, workspacePath); err != nil {
			return projectdomain.Registry{}, false, err
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
	var brokenChanged bool
	registry, brokenChanged, err = s.markMissingWorkspaceProjects(ctx, registry)
	if err != nil {
		return projectdomain.Registry{}, false, err
	}
	if brokenChanged {
		changed = true
	}

	if changed {
		if err := s.registryRepo.Save(ctx, registry); err != nil {
			return projectdomain.Registry{}, false, err
		}
	}
	return registry, changed, nil
}

func (s *Service) markMissingWorkspaceProjects(_ context.Context, registry projectdomain.Registry) (projectdomain.Registry, bool, error) {
	if len(registry.Projects) == 0 {
		return registry, false, nil
	}

	changed := false
	updated := registry
	for projectID, record := range updated.Projects {
		if strings.TrimSpace(record.WorkspacePath) == "" {
			if record.Status != projectdomain.StatusBroken {
				record.Status = projectdomain.StatusBroken
				record.UpdatedAt = s.clock()
				updated.Projects[projectID] = record
				changed = true
			}
			continue
		}

		info, err := os.Stat(record.WorkspacePath)
		if err != nil {
			if os.IsNotExist(err) {
				if record.Status != projectdomain.StatusBroken {
					record.Status = projectdomain.StatusBroken
					record.UpdatedAt = s.clock()
					updated.Projects[projectID] = record
					changed = true
				}
				continue
			}
			return projectdomain.Registry{}, false, fmt.Errorf("stat project workspace %q: %w", record.WorkspacePath, err)
		}
		if !info.IsDir() && record.Status != projectdomain.StatusBroken {
			record.Status = projectdomain.StatusBroken
			record.UpdatedAt = s.clock()
			updated.Projects[projectID] = record
			changed = true
		}
	}

	if changed {
		updated.UpdatedAt = s.clock()
	}
	return updated, changed, nil
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

func (s *Service) ensureMemoryTemplate(ctx context.Context, projectID, workspacePath string) error {
	if s.memoryTemplate == nil {
		return nil
	}
	if err := s.memoryTemplate.EnsureProjectTemplate(ctx, projectID, workspacePath); err != nil {
		return fmt.Errorf("bootstrap project memory template: %w", err)
	}
	return nil
}
