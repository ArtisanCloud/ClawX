package integration

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"clawx/internal/application/command"
	memoryapp "clawx/internal/application/memory"
	projectapp "clawx/internal/application/project"
	"clawx/internal/application/service"
	"clawx/internal/domain/execution"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
)

func TestMemoryFirstTurnLoadInjectsContextOnce(t *testing.T) {
	ctx := context.Background()
	router, projectService, backendSpy, _ := newMemoryExecutionRouterForIntegration(t)

	conversationID := "memory-first-turn-conversation"
	windowID := "memory-first-turn-window"
	routeKey := "telegram:default:direct:memory-first-turn-user"

	if _, err := router.HandleControlCommand(ctx, "/project create memo Memo", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use memo", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("use project: %v", err)
	}
	projectID, mode, err := projectService.ResolveProject(ctx, routeKey)
	if err != nil {
		t.Fatalf("resolve project: %v", err)
	}
	if projectID != "memo" || mode != "binding" {
		t.Fatalf("unexpected project resolution: project=%s mode=%s", projectID, mode)
	}

	first, err := router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: conversationID,
		WindowID:       windowID,
		ProjectID:      "memo",
		Input:          "first-task",
		Backend:        "main",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("first session flow: %v", err)
	}
	if !strings.Contains(first.Execution.Output, "first-task") {
		t.Fatalf("unexpected first execution output: %q", first.Execution.Output)
	}

	firstReq := backendSpy.RequestAt(t, 0)
	if strings.TrimSpace(firstReq.MemoryContext) == "" {
		t.Fatalf("first request should include memory context")
	}
	if !strings.Contains(firstReq.MemoryContext, "# IDENTITY") {
		t.Fatalf("memory context should include template content, got: %q", firstReq.MemoryContext)
	}

	_, err = router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: conversationID,
		WindowID:       windowID,
		ProjectID:      "memo",
		Input:          "second-task",
		Backend:        "main",
		CWD:            ".",
	})
	if err != nil {
		t.Fatalf("second session flow: %v", err)
	}

	secondReq := backendSpy.RequestAt(t, 1)
	if strings.TrimSpace(secondReq.MemoryContext) != "" {
		t.Fatalf("second request should not include first-turn memory context")
	}
}

func newMemoryExecutionRouterForIntegration(t *testing.T) (*service.Router, *projectapp.Service, *recordingMemoryBackend, string) {
	t.Helper()

	tempDir := t.TempDir()
	workspaceRoot := tempDir + "/workspaces"

	registryStore, err := persistence.NewProjectRegistryFileStore(tempDir + "/projects.json")
	if err != nil {
		t.Fatalf("new registry store: %v", err)
	}
	bindingStore, err := persistence.NewProjectBindingFileStore(tempDir + "/bindings.json")
	if err != nil {
		t.Fatalf("new binding store: %v", err)
	}
	proposalStore, err := persistence.NewProjectProposalFileStore(tempDir + "/proposals.json")
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
	backendSpy := &recordingMemoryBackend{name: "memory-first-turn"}
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
	}, manager, backendSpy, service.WithProjectResolver(projectService))

	return router, projectService, backendSpy, workspaceRoot
}

type recordingMemoryBackend struct {
	name     string
	mu       sync.Mutex
	requests []execution.Request
}

func (b *recordingMemoryBackend) Name() string {
	if b.name == "" {
		return "recording-memory"
	}
	return b.name
}

func (b *recordingMemoryBackend) Execute(_ context.Context, request execution.Request) (execution.Result, error) {
	b.mu.Lock()
	b.requests = append(b.requests, request)
	b.mu.Unlock()

	now := time.Now().UTC()
	output := strings.TrimSpace(request.MemoryContext)
	if output != "" {
		output += "\n---\n"
	}
	output += request.Input

	return execution.Result{
		BackendSessionID: "backend-" + request.SessionID,
		Output:           output,
		State:            execution.ResultSuccess,
		StartedAt:        now,
		CompletedAt:      now,
	}, nil
}

func (b *recordingMemoryBackend) Cancel(_ context.Context, _ string) error {
	return nil
}

func (b *recordingMemoryBackend) HealthCheck(_ context.Context) error {
	return nil
}

func (b *recordingMemoryBackend) RequestAt(t *testing.T, index int) execution.Request {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	if index < 0 || index >= len(b.requests) {
		t.Fatalf("request index out of range: %d (len=%d)", index, len(b.requests))
	}
	return b.requests[index]
}

func (b *recordingMemoryBackend) RequestCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.requests)
}
