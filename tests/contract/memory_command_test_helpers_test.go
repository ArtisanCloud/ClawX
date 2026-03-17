package contract

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	memoryapp "clawx/internal/application/memory"
	projectapp "clawx/internal/application/project"
	"clawx/internal/application/service"
	"clawx/internal/domain/execution"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
)

func newMemoryCommandContractRouter(t *testing.T, memoryCfg config.MemoryConfig) (*service.Router, string) {
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
		memoryapp.WithCommandOwnerAllowlist(memoryCfg.OwnerAllowlist),
		memoryapp.WithCommandAutoDigestEnabled(memoryCfg.AutoDigestEnabled),
	)
	repo := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(repo, repo, nil)
	router := service.NewRouter(config.Snapshot{
		AllowedRoots:    []string{"."},
		DefaultCWD:      ".",
		Timeout:         time.Second,
		DefaultAgentID:  "main",
		TelegramEnabled: true,
		Projects: config.ProjectConfig{
			WorkspaceRoot:    workspaceRoot,
			DefaultProjectID: "main",
		},
		Memory: memoryCfg,
	}, manager, contractMemoryBackend{name: "memory-contract"},
		service.WithProjectResolver(projectService),
		service.WithMemoryCommandService(memoryService),
	)
	return router, workspaceRoot
}

func createAndUseMemoryProject(t *testing.T, router *service.Router, conversationID, windowID, routeKey string) {
	t.Helper()
	ctx := context.Background()
	if _, err := router.HandleControlCommand(ctx, "/project create memo Memo", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use memo", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("use project: %v", err)
	}
}

func categorizedCode(err error) string {
	type coded interface {
		Code() string
	}
	if err == nil {
		return ""
	}
	if value, ok := err.(coded); ok {
		return value.Code()
	}
	return ""
}

type contractMemoryBackend struct {
	name string
}

func (b contractMemoryBackend) Name() string {
	if strings.TrimSpace(b.name) == "" {
		return "memory-contract-backend"
	}
	return b.name
}

func (b contractMemoryBackend) Execute(_ context.Context, request execution.Request) (execution.Result, error) {
	now := time.Now().UTC()
	return execution.Result{
		BackendSessionID: "backend-" + request.SessionID,
		Output:           request.Input,
		State:            execution.ResultSuccess,
		StartedAt:        now,
		CompletedAt:      now,
	}, nil
}

func (b contractMemoryBackend) Cancel(_ context.Context, _ string) error {
	return nil
}

func (b contractMemoryBackend) HealthCheck(_ context.Context) error {
	return nil
}
