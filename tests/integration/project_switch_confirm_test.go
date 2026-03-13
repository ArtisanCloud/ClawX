package integration

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"clawx/internal/application/intent"
	projectapp "clawx/internal/application/project"
	"clawx/internal/application/service"
	skilldomain "clawx/internal/domain/skill"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
	chatiface "clawx/internal/interfaces/chat"
)

func TestProjectSwitchConfirm(t *testing.T) {
	ctx := context.Background()
	router, projectService := newProjectIntentRouter(t, 10*time.Minute)

	conversationID := "project-switch-confirm-conversation"
	windowID := "project-switch-confirm-window"
	routeKey := "discord:discord-main:channel:guild-42:thread:thread-confirm"

	if _, err := router.HandleControlCommand(ctx, "/project create bid Bid", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create bid project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project create nba NBA", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create nba project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use bid", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("bind route to bid: %v", err)
	}

	message := mustNormalizeMessage(t, chatiface.NormalizeInput{
		Channel:         "discord",
		InstanceID:      "discord-main",
		UserID:          "project-switch-confirm-user",
		GuildID:         "guild-42",
		ThreadID:        "thread-confirm",
		Text:            "请处理 project:nba 的待办",
		IsDirectMessage: false,
		IsThread:        true,
		IsAllowed:       true,
	})
	decision, err := router.Route(ctx, message)
	if err != nil {
		t.Fatalf("route intent message: %v", err)
	}
	if decision.Kind != service.DecisionControl {
		t.Fatalf("expected suggestion control decision, got=%s", decision.Kind)
	}
	if !strings.HasPrefix(decision.Command, "/project suggest nba") {
		t.Fatalf("unexpected suggestion command: %q", decision.Command)
	}

	suggestion, err := router.HandleControlCommand(ctx, decision.Command, decision.ConversationID, decision.WindowID, decision.RouteKey)
	if err != nil {
		t.Fatalf("handle project suggest command: %v", err)
	}
	if !strings.Contains(suggestion.Message, "建议切换项目: bid -> nba") {
		t.Fatalf("unexpected suggestion message: %q", suggestion.Message)
	}

	proposalID := extractProposalID(t, suggestion.Message)
	confirmed, err := router.HandleControlCommand(ctx, "/project confirm "+proposalID, conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("confirm proposal: %v", err)
	}
	if !strings.Contains(confirmed.Message, "已确认并切换项目: nba") {
		t.Fatalf("unexpected confirm message: %q", confirmed.Message)
	}

	projectID, mode, err := projectService.ResolveProject(ctx, routeKey)
	if err != nil {
		t.Fatalf("resolve project after confirm: %v", err)
	}
	if projectID != "nba" || mode != "binding" {
		t.Fatalf("unexpected routing after confirm: project=%s mode=%s", projectID, mode)
	}
}

func newProjectIntentRouter(t *testing.T, proposalTTL time.Duration) (*service.Router, *projectapp.Service) {
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
		projectapp.WithProposalTTL(proposalTTL),
	)

	pipeline := intent.NewPipeline(projectIntentTestRegistry{}, nil, intent.DefaultLLMFallback{}, 0.72)
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
	}, manager, fakeBackend{name: "project-intent"}, service.WithProjectResolver(projectService), service.WithIntentPipeline(pipeline))

	return router, projectService
}

func extractProposalID(t *testing.T, message string) string {
	t.Helper()
	re := regexp.MustCompile(`/project confirm ([a-zA-Z0-9_-]+)`)
	match := re.FindStringSubmatch(message)
	if len(match) < 2 {
		t.Fatalf("proposal id not found in message: %q", message)
	}
	return strings.TrimSpace(match[1])
}

type projectIntentTestRegistry struct{}

func (projectIntentTestRegistry) Snapshot() skilldomain.RegistrySnapshot {
	return skilldomain.RegistrySnapshot{
		Version:     1,
		Entries:     nil,
		GeneratedAt: time.Now().UTC(),
	}
}

func (projectIntentTestRegistry) FindEntry(_ string) (skilldomain.CatalogEntry, bool) {
	return skilldomain.CatalogEntry{}, false
}

func (projectIntentTestRegistry) FindActiveDefinition(_ string) (skilldomain.Definition, error) {
	return skilldomain.Definition{}, nil
}
