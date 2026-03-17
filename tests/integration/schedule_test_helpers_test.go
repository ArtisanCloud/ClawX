package integration

import (
	"path/filepath"
	"testing"
	"time"

	managedservice "clawx/internal/application/managedservice"
	memoryapp "clawx/internal/application/memory"
	projectapp "clawx/internal/application/project"
	schedulerapp "clawx/internal/application/scheduler"
	"clawx/internal/application/service"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
)

func newScheduleControlRouter(t *testing.T) (*service.Router, string) {
	t.Helper()
	tempDir := t.TempDir()
	workspaceRoot := filepath.Join(tempDir, "workspaces")

	registryStore, err := persistence.NewProjectRegistryFileStore(filepath.Join(tempDir, "projects.json"))
	if err != nil {
		t.Fatalf("new registry store: %v", err)
	}
	bindingStore, err := persistence.NewProjectBindingFileStore(filepath.Join(tempDir, "bindings.json"))
	if err != nil {
		t.Fatalf("new binding store: %v", err)
	}
	proposalStore, err := persistence.NewProjectProposalFileStore(filepath.Join(tempDir, "proposals.json"))
	if err != nil {
		t.Fatalf("new proposal store: %v", err)
	}
	projectService := projectapp.NewService(
		registryStore,
		bindingStore,
		proposalStore,
		projectapp.WithWorkspaceRoot(workspaceRoot),
		projectapp.WithDefaultProjectID("main"),
		projectapp.WithMemoryTemplateManager(memoryapp.NewTemplateManager()),
	)
	scheduleStore, err := persistence.NewSchedulerFileStore(workspaceRoot)
	if err != nil {
		t.Fatalf("new scheduler store: %v", err)
	}
	scheduleService, err := schedulerapp.NewService(workspaceRoot, scheduleStore, scheduleStore)
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}
	serviceControl, err := managedServiceForTests(workspaceRoot)
	if err != nil {
		t.Fatalf("new managed service: %v", err)
	}

	templateStore, err := persistence.NewMemoryTemplateFileStore(workspaceRoot)
	if err != nil {
		t.Fatalf("new memory template store: %v", err)
	}
	journalStore, err := persistence.NewMemoryJournalFileStore(workspaceRoot)
	if err != nil {
		t.Fatalf("new memory journal store: %v", err)
	}
	auditStore, err := persistence.NewMemoryAuditFileStore(workspaceRoot)
	if err != nil {
		t.Fatalf("new memory audit store: %v", err)
	}
	digestStore, err := persistence.NewMemoryDigestFileStore(workspaceRoot)
	if err != nil {
		t.Fatalf("new memory digest store: %v", err)
	}
	memoryService := memoryapp.NewCommandService(
		templateStore,
		journalStore,
		auditStore,
		digestStore,
		memoryapp.WithCommandWorkspaceRoot(workspaceRoot),
		memoryapp.WithCommandProjectResolver(projectService),
	)

	repo := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(repo, repo, nil)
	router := service.NewRouter(config.Snapshot{
		AllowedRoots:    []string{"."},
		DefaultCWD:      ".",
		Timeout:         2 * time.Second,
		DefaultAgentID:  "main",
		TelegramEnabled: true,
		Projects: config.ProjectConfig{
			WorkspaceRoot:    workspaceRoot,
			DefaultProjectID: "main",
		},
	}, manager, serviceTestBackend{},
		service.WithProjectResolver(projectService),
		service.WithScheduleCommandService(scheduleService),
		service.WithServiceCommandService(serviceControl),
		service.WithMemoryCommandService(memoryService),
	)
	return router, workspaceRoot
}

func newScheduleControlRouterOnWorkspace(t *testing.T, workspaceRoot string) (*service.Router, string) {
	return newScheduleControlRouterOnWorkspaceWithAgent(t, workspaceRoot, "main")
}

func newScheduleControlRouterOnWorkspaceWithAgent(t *testing.T, workspaceRoot, agentID string) (*service.Router, string) {
	t.Helper()
	tempDir := t.TempDir()
	registryStore, err := persistence.NewProjectRegistryFileStore(filepath.Join(tempDir, "projects.json"))
	if err != nil {
		t.Fatalf("new registry store: %v", err)
	}
	bindingStore, err := persistence.NewProjectBindingFileStore(filepath.Join(tempDir, "bindings.json"))
	if err != nil {
		t.Fatalf("new binding store: %v", err)
	}
	proposalStore, err := persistence.NewProjectProposalFileStore(filepath.Join(tempDir, "proposals.json"))
	if err != nil {
		t.Fatalf("new proposal store: %v", err)
	}
	projectService := projectapp.NewService(
		registryStore,
		bindingStore,
		proposalStore,
		projectapp.WithWorkspaceRoot(workspaceRoot),
		projectapp.WithDefaultProjectID("main"),
	)
	scheduleStore, err := persistence.NewSchedulerFileStore(workspaceRoot)
	if err != nil {
		t.Fatalf("new scheduler store: %v", err)
	}
	scheduleService, err := schedulerapp.NewService(workspaceRoot, scheduleStore, scheduleStore)
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}
	repo := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(repo, repo, nil)
	router := service.NewRouter(config.Snapshot{
		AllowedRoots:    []string{"."},
		DefaultCWD:      ".",
		Timeout:         2 * time.Second,
		DefaultAgentID:  agentID,
		TelegramEnabled: true,
		Projects: config.ProjectConfig{
			WorkspaceRoot:    workspaceRoot,
			DefaultProjectID: "main",
		},
	}, manager, serviceTestBackend{}, service.WithProjectResolver(projectService), service.WithScheduleCommandService(scheduleService))
	return router, workspaceRoot
}

func managedServiceForTests(workspaceRoot string) (service.ServiceCommandService, error) {
	return managedservice.NewService(workspaceRoot)
}
