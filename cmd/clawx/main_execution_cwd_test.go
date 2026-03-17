package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"clawx/internal/application/service"
	projectdomain "clawx/internal/domain/project"
)

func TestResolveExecutionCWDUsesProjectWorkspace(t *testing.T) {
	tempDir := t.TempDir()
	projectWorkspace := filepath.Join(tempDir, "workspaces", "image_tools")
	runtime := agentRuntime{
		agentID: "main",
		cwd:     "/home/ubuntu/workspace/ClawX",
		projectCWD: fakeExecutionCWDResolver{
			record: projectdomain.Record{
				ID:            "image_tools",
				WorkspacePath: projectWorkspace,
			},
		},
	}
	decision := service.Decision{ProjectID: "image_tools"}

	got := resolveExecutionCWD(context.Background(), runtime, decision)
	want := filepath.Join(projectWorkspace, ".agents", "main", "workspace")
	if got != want {
		t.Fatalf("cwd mismatch: got %q want %q", got, want)
	}
}

func TestResolveExecutionCWDFallbackToRuntimeCWD(t *testing.T) {
	runtime := agentRuntime{
		agentID: "main",
		cwd:     "/home/ubuntu/workspace/ClawX",
		projectCWD: fakeExecutionCWDResolver{
			err: errors.New("project not found"),
		},
	}
	decision := service.Decision{ProjectID: "image_tools"}

	got := resolveExecutionCWD(context.Background(), runtime, decision)
	want := "/home/ubuntu/workspace/ClawX"
	if got != want {
		t.Fatalf("cwd fallback mismatch: got %q want %q", got, want)
	}
}

func TestResolveExecutionCWDUsesProjectWorkspaceWhenAgentIDEmpty(t *testing.T) {
	tempDir := t.TempDir()
	projectWorkspace := filepath.Join(tempDir, "workspaces", "image_tools")
	runtime := agentRuntime{
		cwd: "/home/ubuntu/workspace/ClawX",
		projectCWD: fakeExecutionCWDResolver{
			record: projectdomain.Record{
				ID:            "image_tools",
				WorkspacePath: projectWorkspace,
			},
		},
	}
	decision := service.Decision{ProjectID: "image_tools"}

	got := resolveExecutionCWD(context.Background(), runtime, decision)
	if got != projectWorkspace {
		t.Fatalf("project workspace mismatch: got %q want %q", got, projectWorkspace)
	}
}

func TestResolveExecutionCWDFallbackToDot(t *testing.T) {
	runtime := agentRuntime{agentID: "main"}
	decision := service.Decision{ProjectID: "image_tools"}

	got := resolveExecutionCWD(context.Background(), runtime, decision)
	if got != "." {
		t.Fatalf("cwd default mismatch: got %q want %q", got, ".")
	}
}

type fakeExecutionCWDResolver struct {
	record projectdomain.Record
	err    error
}

func (f fakeExecutionCWDResolver) GetProject(_ context.Context, _ string) (projectdomain.Record, error) {
	if f.err != nil {
		return projectdomain.Record{}, f.err
	}
	return f.record, nil
}
