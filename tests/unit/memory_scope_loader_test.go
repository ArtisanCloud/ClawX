package unit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	memoryapp "clawx/internal/application/memory"
	memorydomain "clawx/internal/domain/memory"
)

func TestMemoryScopeResolverResolve(t *testing.T) {
	resolver := memoryapp.NewScopeResolver()

	t.Run("infer_main_mode_from_direct_route", func(t *testing.T) {
		scope, err := resolver.Resolve(memoryapp.ScopeInput{
			AgentID:   "Agent A",
			ProjectID: "Project 1",
			RouteKey:  "telegram:default:direct:user-1",
		})
		if err != nil {
			t.Fatalf("resolve scope: %v", err)
		}
		if scope.AgentID != "agent-a" {
			t.Fatalf("unexpected agent id: %q", scope.AgentID)
		}
		if scope.ProjectID != "project-1" {
			t.Fatalf("unexpected project id: %q", scope.ProjectID)
		}
		if scope.ChatMode != memorydomain.ChatModeMain {
			t.Fatalf("unexpected chat mode: %q", scope.ChatMode)
		}
	})

	t.Run("explicit_shared_mode_overrides_inference", func(t *testing.T) {
		scope, err := resolver.Resolve(memoryapp.ScopeInput{
			AgentID:   "main",
			ProjectID: "main",
			RouteKey:  "telegram:default:direct:user-1",
			ChatMode:  "shared",
		})
		if err != nil {
			t.Fatalf("resolve scope: %v", err)
		}
		if scope.ChatMode != memorydomain.ChatModeShared {
			t.Fatalf("unexpected chat mode: %q", scope.ChatMode)
		}
	})
}

func TestMemoryLoaderLoad(t *testing.T) {
	tempDir := t.TempDir()
	fileA := filepath.Join(tempDir, "a.txt")
	fileB := filepath.Join(tempDir, "b.txt")
	if err := os.WriteFile(fileA, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write fileA: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("beta"), 0o644); err != nil {
		t.Fatalf("write fileB: %v", err)
	}

	scope := memorydomain.MemoryScopeKey{
		AgentID:   "main",
		ProjectID: "main",
		RouteKey:  "telegram:default:direct:user-1",
		ChatMode:  memorydomain.ChatModeMain,
	}
	profile := memorydomain.MemoryProfile{
		ScopeKey:         scope,
		TokenBudget:      8,
		ACLMode:          memorydomain.ACLModeStrict,
		AllowMainPrivate: false,
	}

	loader := memoryapp.NewLoader()
	result, err := loader.Load(context.Background(), memoryapp.LoaderInput{
		ScopeKey: scope,
		Profile:  profile,
		Candidates: []memorydomain.MemoryLoadItem{
			{Layer: memorydomain.LayerAgentPrivate, Path: fileA},
			{Layer: memorydomain.LayerMainPrivate, Path: fileB},
			{Layer: memorydomain.LayerProjectShare, Path: filepath.Join(tempDir, "missing.txt")},
		},
	})
	if err != nil {
		t.Fatalf("load memory: %v", err)
	}
	if len(result.LoadedFiles) != 1 || result.LoadedFiles[0] != fileA {
		t.Fatalf("unexpected loaded files: %#v", result.LoadedFiles)
	}
	if len(result.DeniedFiles) != 1 || result.DeniedFiles[0] != fileB {
		t.Fatalf("unexpected denied files: %#v", result.DeniedFiles)
	}
	if !result.Degraded {
		t.Fatalf("expected degraded result due to missing file")
	}
	if !strings.Contains(result.ErrorSummary, "read_failed") {
		t.Fatalf("unexpected error summary: %q", result.ErrorSummary)
	}
	if !strings.Contains(result.PromptContext, "alpha") {
		t.Fatalf("prompt context should include loaded content")
	}

	if len(result.LoadItems) != 3 {
		t.Fatalf("unexpected load items count: %d", len(result.LoadItems))
	}
	if result.LoadItems[0].Decision != memorydomain.DecisionLoaded {
		t.Fatalf("unexpected decision for item0: %q", result.LoadItems[0].Decision)
	}
	if result.LoadItems[1].Decision != memorydomain.DecisionSkippedACL {
		t.Fatalf("unexpected decision for item1: %q", result.LoadItems[1].Decision)
	}
	if result.LoadItems[2].Decision != memorydomain.DecisionError {
		t.Fatalf("unexpected decision for item2: %q", result.LoadItems[2].Decision)
	}
}

func TestMemoryLoaderBudgetAndValidation(t *testing.T) {
	loader := memoryapp.NewLoader(memoryapp.WithLoaderReader(loaderReaderStub{
		items: map[string][]byte{
			"/x/first.txt":  []byte("1234"),
			"/x/second.txt": []byte("5678"),
		},
	}))

	scope := memorydomain.MemoryScopeKey{
		AgentID:   "main",
		ProjectID: "main",
		RouteKey:  "telegram:default:channel:group-1",
		ChatMode:  memorydomain.ChatModeShared,
	}
	profile := memorydomain.MemoryProfile{
		ScopeKey:    scope,
		TokenBudget: 4,
	}

	result, err := loader.Load(context.Background(), memoryapp.LoaderInput{
		ScopeKey: scope,
		Profile:  profile,
		Candidates: []memorydomain.MemoryLoadItem{
			{Layer: memorydomain.LayerProjectShare, Path: "/x/first.txt"},
			{Layer: memorydomain.LayerProjectShare, Path: "/x/second.txt"},
		},
	})
	if err != nil {
		t.Fatalf("load memory: %v", err)
	}
	if len(result.LoadedFiles) != 1 {
		t.Fatalf("expected one loaded file, got %d", len(result.LoadedFiles))
	}
	if len(result.DeniedFiles) != 1 {
		t.Fatalf("expected one denied file, got %d", len(result.DeniedFiles))
	}
	if result.LoadItems[1].Decision != memorydomain.DecisionSkippedBudget {
		t.Fatalf("expected skipped budget, got %q", result.LoadItems[1].Decision)
	}
}

type loaderReaderStub struct {
	items map[string][]byte
}

func (s loaderReaderStub) ReadFile(_ context.Context, path string) ([]byte, error) {
	if value, ok := s.items[path]; ok {
		return value, nil
	}
	return nil, errors.New("not found")
}
