package unit

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	projectapp "clawx/internal/application/project"
	"clawx/internal/application/service"
	"clawx/internal/domain/execution"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
)

func TestProjectCommandCreateListCurrent(t *testing.T) {
	ctx := context.Background()
	router, workspaceRoot := newProjectControlRouterForUnit(t)
	conversationID := "project-command-create-list-current"
	windowID := "window-project-command-create-list-current"
	routeKey := "telegram:default:direct:project-unit-user-1"

	created, err := router.HandleControlCommand(ctx, "/project create bid Bid", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if !strings.Contains(created.Message, "已创建项目: bid") {
		t.Fatalf("unexpected create response: %q", created.Message)
	}
	if !strings.Contains(created.Message, filepath.Join(workspaceRoot, "bid")) {
		t.Fatalf("create response should include workspace path: %q", created.Message)
	}

	listed, err := router.HandleControlCommand(ctx, "/project list", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if !strings.Contains(listed.Message, "项目列表:") {
		t.Fatalf("unexpected list response: %q", listed.Message)
	}
	if !strings.Contains(listed.Message, "main [active]") {
		t.Fatalf("list should include default project: %q", listed.Message)
	}
	if !strings.Contains(listed.Message, "bid [active]") {
		t.Fatalf("list should include created project: %q", listed.Message)
	}

	current, err := router.HandleControlCommand(ctx, "/project current", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("current project: %v", err)
	}
	if !strings.Contains(current.Message, "当前项目: main [active]") {
		t.Fatalf("unexpected current response: %q", current.Message)
	}
	if !strings.Contains(current.Message, "mode=fallback") {
		t.Fatalf("current response should report fallback mode: %q", current.Message)
	}
}

func newProjectControlRouterForUnit(t *testing.T) (*service.Router, string) {
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

	repo := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(repo, repo, nil)
	router := service.NewRouter(config.Snapshot{
		AllowedRoots:    []string{"."},
		DefaultCWD:      ".",
		Timeout:         2 * time.Second,
		TelegramEnabled: true,
		Projects: config.ProjectConfig{
			WorkspaceRoot:    workspaceRoot,
			DefaultProjectID: "main",
		},
	}, manager, unitProjectBackend{name: "unit-project"}, service.WithProjectResolver(projectService))

	return router, workspaceRoot
}

type unitProjectBackend struct {
	name string
}

func (b unitProjectBackend) Name() string {
	if strings.TrimSpace(b.name) == "" {
		return "unit-project-backend"
	}
	return b.name
}

func (b unitProjectBackend) Execute(_ context.Context, request execution.Request) (execution.Result, error) {
	now := time.Now().UTC()
	return execution.Result{
		BackendSessionID: "backend-" + request.SessionID,
		Output:           request.Input,
		State:            execution.ResultSuccess,
		StartedAt:        now,
		CompletedAt:      now,
	}, nil
}

func (b unitProjectBackend) Cancel(_ context.Context, _ string) error {
	return nil
}

func (b unitProjectBackend) HealthCheck(_ context.Context) error {
	return nil
}
