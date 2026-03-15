package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	memoryapp "clawx/internal/application/memory"
	projectapp "clawx/internal/application/project"
	"clawx/internal/application/service"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
)

func TestMemoryTemplateBootstrapOnProjectCreateAndRepair(t *testing.T) {
	ctx := context.Background()
	router, _, workspaceRoot := newMemoryProjectRouterForIntegration(t)

	conversationID := "memory-project-bootstrap-conversation"
	windowID := "memory-project-bootstrap-window"
	routeKey := "telegram:default:direct:memory-bootstrap-user"

	created, err := router.HandleControlCommand(ctx, "/project create tools Tools", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if !strings.Contains(created.Message, "已创建项目: tools") {
		t.Fatalf("unexpected create response: %q", created.Message)
	}

	workspace := filepath.Join(workspaceRoot, "tools")
	required := []string{"AGENTS.md", "HEARTBEAT.md", "IDENTITY.md", "SOUL.md", "TOOLS.md", "USER.md", "MEMORY.md"}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(workspace, name)); err != nil {
			t.Fatalf("expected bootstrap file %s: %v", name, err)
		}
	}

	if err := os.Remove(filepath.Join(workspace, "USER.md")); err != nil {
		t.Fatalf("remove USER.md: %v", err)
	}

	repaired, err := router.HandleControlCommand(ctx, "/project repair tools", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("repair project: %v", err)
	}
	if !strings.Contains(repaired.Message, "已修复项目: tools") {
		t.Fatalf("unexpected repair response: %q", repaired.Message)
	}
	if _, err := os.Stat(filepath.Join(workspace, "USER.md")); err != nil {
		t.Fatalf("USER.md should be healed by repair: %v", err)
	}
}

func newMemoryProjectRouterForIntegration(t *testing.T) (*service.Router, *projectapp.Service, string) {
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
		Memory: config.MemoryConfig{TokenBudget: 4096},
	}, manager, fakeBackend{name: "memory-project-bootstrap"}, service.WithProjectResolver(projectService))

	return router, projectService, workspaceRoot
}
