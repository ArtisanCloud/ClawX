package contract

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	projectapp "clawx/internal/application/project"
	schedulerapp "clawx/internal/application/scheduler"
	"clawx/internal/application/service"
	"clawx/internal/domain/execution"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
)

func newScheduleContractRouter(t *testing.T) *service.Router {
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
	)
	store, err := persistence.NewSchedulerFileStore(workspaceRoot)
	if err != nil {
		t.Fatalf("new scheduler store: %v", err)
	}
	scheduleService, err := schedulerapp.NewService(workspaceRoot, store, store)
	if err != nil {
		t.Fatalf("new scheduler service: %v", err)
	}
	repo := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(repo, repo, nil)
	return service.NewRouter(config.Snapshot{
		AllowedRoots:    []string{"."},
		DefaultCWD:      ".",
		Timeout:         time.Second,
		DefaultAgentID:  "main",
		TelegramEnabled: true,
		Projects:        config.ProjectConfig{WorkspaceRoot: workspaceRoot, DefaultProjectID: "main"},
	}, manager, scheduleContractBackend{}, service.WithProjectResolver(projectService), service.WithScheduleCommandService(scheduleService))
}

type scheduleContractBackend struct{}

func (scheduleContractBackend) Name() string { return "schedule-contract" }
func (scheduleContractBackend) Execute(_ context.Context, request execution.Request) (execution.Result, error) {
	now := time.Now().UTC()
	return execution.Result{BackendSessionID: "backend-" + request.SessionID, Output: request.Input, State: execution.ResultSuccess, StartedAt: now, CompletedAt: now}, nil
}
func (scheduleContractBackend) Cancel(context.Context, string) error { return nil }
func (scheduleContractBackend) HealthCheck(context.Context) error    { return nil }
