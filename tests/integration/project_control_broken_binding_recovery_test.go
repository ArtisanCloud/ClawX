package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clawx/internal/application/service"
	chatiface "clawx/internal/interfaces/chat"
)

func TestProjectControlCommandBypassesBrokenBindingResolution(t *testing.T) {
	ctx := context.Background()
	router, projectService, workspaceRoot := newProjectControlRouterForIntegration(t)

	message := mustNormalizeMessage(t, chatiface.NormalizeInput{
		Channel:         "discord",
		InstanceID:      "discord-main",
		UserID:          "project-broken-binding-user",
		GuildID:         "guild-42",
		ThreadID:        "thread-broken-binding",
		Text:            "/project create image-tools Legacy",
		IsDirectMessage: false,
		IsThread:        true,
		IsAllowed:       true,
	})

	if _, err := router.HandleControlCommand(ctx, "/project create image-tools Legacy", message.ConversationID, message.WindowID, message.RouteKey); err != nil {
		t.Fatalf("create legacy project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project create image_tools New", message.ConversationID, message.WindowID, message.RouteKey); err != nil {
		t.Fatalf("create new project: %v", err)
	}
	if _, err := router.HandleControlCommand(ctx, "/project use image-tools", message.ConversationID, message.WindowID, message.RouteKey); err != nil {
		t.Fatalf("bind route to legacy project: %v", err)
	}

	if err := os.RemoveAll(filepath.Join(workspaceRoot, "image-tools")); err != nil {
		t.Fatalf("remove legacy workspace: %v", err)
	}
	if _, _, err := projectService.ResolveProject(ctx, message.RouteKey); err == nil {
		t.Fatalf("expected resolve to fail after legacy workspace removal")
	}

	message.Text = "/project use image_tools"
	decision, err := router.Route(ctx, message)
	if err != nil {
		t.Fatalf("project control command should bypass broken binding precheck: %v", err)
	}
	if decision.Kind != service.DecisionControl {
		t.Fatalf("expected control decision, got %s", decision.Kind)
	}
	if decision.Command != "/project use image_tools" {
		t.Fatalf("unexpected command: %q", decision.Command)
	}

	used, err := router.HandleControlCommand(ctx, decision.Command, decision.ConversationID, decision.WindowID, decision.RouteKey)
	if err != nil {
		t.Fatalf("project use should recover route from broken binding: %v", err)
	}
	if !strings.Contains(used.Message, "已切换当前项目: image_tools") {
		t.Fatalf("unexpected use response: %q", used.Message)
	}

	current, err := router.HandleControlCommand(ctx, "/project current", decision.ConversationID, decision.WindowID, decision.RouteKey)
	if err != nil {
		t.Fatalf("project current after recovery: %v", err)
	}
	if !strings.Contains(current.Message, "当前项目: image_tools [active]") {
		t.Fatalf("route should now bind image_tools: %q", current.Message)
	}
}
