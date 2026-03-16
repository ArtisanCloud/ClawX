package contract

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

func TestProjectSessionKeyContract(t *testing.T) {
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
		Timeout:         time.Second,
		TelegramEnabled: true,
		Projects: config.ProjectConfig{
			WorkspaceRoot:    workspaceRoot,
			DefaultProjectID: "main",
		},
	}, manager, contractProjectBackend{name: "project-contract"}, service.WithProjectResolver(projectService))

	conversationID := "project-session-key-contract"
	windowID := "project-session-key-window"
	routeKey := "telegram:default:direct:project-contract-user-1"

	if _, err := router.HandleControlCommand(ctx, "/project create bid Bid", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create bid project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project create nba NBA", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create nba project: %v", err)
	}

	if _, err := router.HandleControlCommand(ctx, "/project use bid", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("switch route to bid: %v", err)
	}
	bidNew, err := router.HandleControlCommand(ctx, "/new", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("create bid session: %v", err)
	}
	if !strings.HasPrefix(bidNew.CreatedSessionID, "sess-bid-") {
		t.Fatalf("bid session key should include project id: %s", bidNew.CreatedSessionID)
	}

	if _, err := router.HandleControlCommand(ctx, "/project use nba", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("switch route to nba: %v", err)
	}
	nbaNew, err := router.HandleControlCommand(ctx, "/new", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("create nba session: %v", err)
	}
	if !strings.HasPrefix(nbaNew.CreatedSessionID, "sess-nba-") {
		t.Fatalf("nba session key should include project id: %s", nbaNew.CreatedSessionID)
	}
	if bidNew.CreatedSessionID == nbaNew.CreatedSessionID {
		t.Fatalf("project-scoped session keys should be distinct")
	}

	if _, err := router.HandleControlCommand(ctx, "/project use bid", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("switch route back to bid: %v", err)
	}
	currentBid, err := router.HandleControlCommand(ctx, "/current", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("query current bid session: %v", err)
	}
	if currentBid.CurrentSession == nil || currentBid.CurrentSession.ID != bidNew.CreatedSessionID {
		t.Fatalf("bid current session should be isolated and recoverable: got=%v want=%s", currentBid.CurrentSession, bidNew.CreatedSessionID)
	}
}

type contractProjectBackend struct {
	name string
}

func (b contractProjectBackend) Name() string {
	if strings.TrimSpace(b.name) == "" {
		return "project-contract-backend"
	}
	return b.name
}

func (b contractProjectBackend) Execute(_ context.Context, request execution.Request) (execution.Result, error) {
	now := time.Now().UTC()
	return execution.Result{
		BackendSessionID: "backend-" + request.SessionID,
		Output:           request.Input,
		State:            execution.ResultSuccess,
		StartedAt:        now,
		CompletedAt:      now,
	}, nil
}

func (b contractProjectBackend) Cancel(_ context.Context, _ string) error {
	return nil
}

func (b contractProjectBackend) HealthCheck(_ context.Context) error {
	return nil
}
