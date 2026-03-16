package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	projectapp "clawx/internal/application/project"
	"clawx/internal/application/service"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
	chatiface "clawx/internal/interfaces/chat"
)

func TestProjectFallbackRouting(t *testing.T) {
	ctx := context.Background()
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
	}, manager, fakeBackend{name: "project-fallback"}, service.WithProjectResolver(projectService))

	message, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
		Channel:         "telegram",
		UserID:          "project-user-1",
		Text:            "first",
		IsDirectMessage: true,
		IsAllowed:       true,
	})
	if err != nil {
		t.Fatalf("normalize message: %v", err)
	}

	fallbackDecision, err := router.Route(ctx, message)
	if err != nil {
		t.Fatalf("route fallback: %v", err)
	}
	if fallbackDecision.ProjectID != "main" {
		t.Fatalf("unexpected fallback project: %q", fallbackDecision.ProjectID)
	}
	if fallbackDecision.ProjectMode != "fallback" {
		t.Fatalf("unexpected fallback project mode: %q", fallbackDecision.ProjectMode)
	}

	if _, err := projectService.CreateProject(ctx, "bid", "Bid", ""); err != nil {
		t.Fatalf("create bid project: %v", err)
	}
	if _, err := projectService.UseProject(ctx, message.RouteKey, "bid", "tester"); err != nil {
		t.Fatalf("bind route to bid project: %v", err)
	}

	boundDecision, err := router.Route(ctx, message)
	if err != nil {
		t.Fatalf("route bound: %v", err)
	}
	if boundDecision.ProjectID != "bid" {
		t.Fatalf("unexpected bound project: %q", boundDecision.ProjectID)
	}
	if boundDecision.ProjectMode != "binding" {
		t.Fatalf("unexpected bound project mode: %q", boundDecision.ProjectMode)
	}
}
