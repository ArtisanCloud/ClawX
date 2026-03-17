package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	managedservice "clawx/internal/application/managedservice"
	projectapp "clawx/internal/application/project"
	"clawx/internal/application/service"
	"clawx/internal/domain/execution"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
)

func TestServiceControlFlowStartStopAndLogs(t *testing.T) {
	ctx := context.Background()
	router, err := newServiceControlRouter(t)
	if err != nil {
		t.Fatalf("new service control router: %v", err)
	}

	conversationID := "service-control-conversation"
	windowID := "service-control-window"
	routeKey := "discord:default:direct:user-a:thread:service"

	if _, err := router.HandleControlCommand(ctx, "/project create image_tools 图片工具项目", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use image_tools", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("use project: %v", err)
	}

	start, err := router.HandleControlCommand(ctx, "/service start worker -- sleep 30", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("start service: %v", err)
	}
	if !strings.Contains(start.Message, "服务已启动: worker") {
		t.Fatalf("unexpected start message: %q", start.Message)
	}

	status, err := router.HandleControlCommand(ctx, "/service status worker", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("status service: %v", err)
	}
	if !strings.Contains(status.Message, "worker [running]") {
		t.Fatalf("unexpected status message: %q", status.Message)
	}

	stop, err := router.HandleControlCommand(ctx, "/service stop worker", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("stop service: %v", err)
	}
	if !strings.Contains(stop.Message, "服务已停止: worker") {
		t.Fatalf("unexpected stop message: %q", stop.Message)
	}

	if _, err := router.HandleControlCommand(ctx, "/service start logger -- echo hello-image", conversationID, windowID, routeKey); err != nil {
		t.Fatalf("start logger service: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	logs, err := router.HandleControlCommand(ctx, "/service logs logger --tail=20", conversationID, windowID, routeKey)
	if err != nil {
		t.Fatalf("logs service: %v", err)
	}
	if !strings.Contains(logs.Message, "hello-image") {
		t.Fatalf("unexpected logs message: %q", logs.Message)
	}
}

func newServiceControlRouter(t *testing.T) (*service.Router, error) {
	t.Helper()

	tempDir := t.TempDir()
	workspaceRoot := tempDir + "/workspaces"

	registryStore, err := persistence.NewProjectRegistryFileStore(tempDir + "/projects.json")
	if err != nil {
		return nil, err
	}
	bindingStore, err := persistence.NewProjectBindingFileStore(tempDir + "/bindings.json")
	if err != nil {
		return nil, err
	}
	proposalStore, err := persistence.NewProjectProposalFileStore(tempDir + "/proposals.json")
	if err != nil {
		return nil, err
	}
	projectService := projectapp.NewService(
		registryStore,
		bindingStore,
		proposalStore,
		projectapp.WithWorkspaceRoot(workspaceRoot),
		projectapp.WithDefaultProjectID("main"),
	)
	serviceControl, err := managedservice.NewService(workspaceRoot)
	if err != nil {
		return nil, err
	}
	repo := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(repo, repo, nil)
	router := service.NewRouter(config.Snapshot{
		AllowedRoots:    []string{"."},
		DefaultCWD:      ".",
		Timeout:         2 * time.Second,
		DefaultAgentID:  "main",
		TelegramEnabled: true,
		Projects: config.ProjectConfig{
			WorkspaceRoot:    workspaceRoot,
			DefaultProjectID: "main",
		},
	}, manager, serviceTestBackend{},
		service.WithProjectResolver(projectService),
		service.WithServiceCommandService(serviceControl),
	)
	return router, nil
}

type serviceTestBackend struct{}

func (serviceTestBackend) Name() string { return "service-test" }
func (serviceTestBackend) Execute(context.Context, execution.Request) (execution.Result, error) {
	now := time.Now().UTC()
	return execution.Result{
		BackendSessionID: "backend-service-test",
		Output:           "ok",
		State:            execution.ResultSuccess,
		StartedAt:        now,
		CompletedAt:      now,
	}, nil
}
func (serviceTestBackend) Cancel(context.Context, string) error { return nil }
func (serviceTestBackend) HealthCheck(context.Context) error    { return nil }
