package integration

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	projectapp "clawx/internal/application/project"
	"clawx/internal/application/service"
	projectdomain "clawx/internal/domain/project"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
)

func TestProjectDeleteGuard(t *testing.T) {
	ctx := context.Background()
	router, sessionBusy := newProjectDeleteGuardRouter(t)

	conversationID := "project-delete-guard-conversation"
	windowID := "project-delete-guard-window"
	routeKey := "discord:discord-main:channel:guild-42:thread:thread-delete"

	if _, err := router.HandleControlCommand(ctx, "/project create bid Bid", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create bid project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use bid", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("bind route to bid: %v", err)
	}

	_, err := router.HandleControlCommand(ctx, "/project delete bid", conversationID, windowID, routeKey)
	if err == nil {
		t.Fatalf("delete should fail while project has bindings")
	}
	if !errors.Is(err, projectdomain.ErrProjectInUse) {
		t.Fatalf("unexpected delete error with bindings: %v", err)
	}

	if _, err := router.HandleControlCommand(ctx, "/project unbind "+routeKey, conversationID, windowID, routeKey); err != nil {
		t.Fatalf("unbind route from bid: %v", err)
	}

	*sessionBusy = true
	_, err = router.HandleControlCommand(ctx, "/project delete bid", conversationID, windowID, routeKey)
	if err == nil {
		t.Fatalf("delete should fail while project has active sessions")
	}
	if !errors.Is(err, projectdomain.ErrProjectBusy) {
		t.Fatalf("unexpected delete error with sessions: %v", err)
	}

	*sessionBusy = false
	deleted, err := router.HandleControlCommand(ctx, "/project delete bid", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("delete bid project: %v", err)
	}
	if !strings.Contains(deleted.Message, "已删除项目: bid") {
		t.Fatalf("unexpected delete response: %q", deleted.Message)
	}

	listed, err := router.HandleControlCommand(ctx, "/project list", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("list projects after delete: %v", err)
	}
	if strings.Contains(listed.Message, "\n- bid [") {
		t.Fatalf("deleted project should not appear in list: %q", listed.Message)
	}
}

func newProjectDeleteGuardRouter(t *testing.T) (*service.Router, *bool) {
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

	sessionBusy := false
	projectService := projectapp.NewService(
		registryStore,
		bindingStore,
		proposalStore,
		projectapp.WithWorkspaceRoot(workspaceRoot),
		projectapp.WithDefaultProjectID("main"),
		projectapp.WithActiveSessionChecker(func(_ context.Context, projectID string) (bool, error) {
			return projectID == "bid" && sessionBusy, nil
		}),
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
	}, manager, fakeBackend{name: "project-delete-guard"}, service.WithProjectResolver(projectService))
	return router, &sessionBusy
}
