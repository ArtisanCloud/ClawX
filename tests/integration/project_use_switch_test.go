package integration

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	projectapp "clawx/internal/application/project"
	"clawx/internal/application/service"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
)

func TestProjectUseSwitch(t *testing.T) {
	ctx := context.Background()
	router, projectService, _ := newProjectControlRouterForIntegration(t)

	routeKeyA := "discord:discord-main:channel:guild-42:thread:thread-a"
	conversationA := "project-use-switch-conversation-a"
	windowA := "project-use-switch-window-a"

	if _, err := router.HandleControlCommand(ctx, "/project create bid Bid", conversationA, windowA, routeKeyA); err != nil {
		t.Fatalf("create bid project: %v", err)
	}

	used, err := router.HandleControlCommand(ctx, "/project use bid", conversationA, windowA, routeKeyA)
	if err != nil {
		t.Fatalf("use bid project: %v", err)
	}
	if !strings.Contains(used.Message, "已切换当前项目: bid") {
		t.Fatalf("unexpected use response: %q", used.Message)
	}

	projectID, mode, err := projectService.ResolveProject(ctx, routeKeyA)
	if err != nil {
		t.Fatalf("resolve route key A: %v", err)
	}
	if projectID != "bid" || mode != "binding" {
		t.Fatalf("unexpected routing after use: project=%q mode=%q", projectID, mode)
	}

	currentA, err := router.HandleControlCommand(ctx, "/project current", conversationA, windowA, routeKeyA)
	if err != nil {
		t.Fatalf("current route key A: %v", err)
	}
	if !strings.Contains(currentA.Message, "当前项目: bid [active]") {
		t.Fatalf("unexpected current A response: %q", currentA.Message)
	}
	if !strings.Contains(currentA.Message, "mode=binding") {
		t.Fatalf("current A response should report binding mode: %q", currentA.Message)
	}

	routeKeyB := "discord:discord-main:channel:guild-42:thread:thread-b"
	conversationB := "project-use-switch-conversation-b"
	windowB := "project-use-switch-window-b"
	currentB, err := router.HandleControlCommand(ctx, "/project current", conversationB, windowB, routeKeyB)
	if err != nil {
		t.Fatalf("current route key B: %v", err)
	}
	if !strings.Contains(currentB.Message, "当前项目: main [active]") {
		t.Fatalf("unexpected current B response: %q", currentB.Message)
	}
	if !strings.Contains(currentB.Message, "mode=fallback") {
		t.Fatalf("current B response should report fallback mode: %q", currentB.Message)
	}
}

func newProjectControlRouterForIntegration(t *testing.T) (*service.Router, *projectapp.Service, string) {
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
		AllowedRoots:   []string{"."},
		DefaultCWD:     ".",
		Timeout:        2 * time.Second,
		DiscordEnabled: true,
		Projects: config.ProjectConfig{
			WorkspaceRoot:    workspaceRoot,
			DefaultProjectID: "main",
		},
	}, manager, fakeBackend{name: "project-control"}, service.WithProjectResolver(projectService))

	return router, projectService, workspaceRoot
}
