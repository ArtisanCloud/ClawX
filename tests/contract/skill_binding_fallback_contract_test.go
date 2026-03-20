package contract

import (
	"context"
	"path/filepath"
	"testing"

	"clawx/internal/application/skillorchestrator"
	skilldomain "clawx/internal/domain/skill"
	"clawx/internal/infrastructure/persistence"
)

func TestSkillBindingFallbackContract(t *testing.T) {
	store, err := persistence.NewSkillBindingFileStore(filepath.Join(t.TempDir(), "bindings.json"))
	if err != nil {
		t.Fatalf("new binding store: %v", err)
	}
	svc := skillorchestrator.NewBindingService(store, nil)
	ctx := context.Background()

	agentReq := skillorchestrator.BindRequest{
		SkillID: "bid.collect", Version: "v2.0.0",
		Scope: skilldomain.ScopeAgentLocal, AgentID: "a1",
	}
	projectReq := skillorchestrator.BindRequest{
		SkillID: "bid.collect", Version: "v1.1.0",
		Scope: skilldomain.ScopeProject, ProjectID: "p1",
	}
	globalReq := skillorchestrator.BindRequest{
		SkillID: "bid.collect", Version: "v1.0.0",
		Scope: skilldomain.ScopeGlobal,
	}
	for _, req := range []skillorchestrator.BindRequest{globalReq, projectReq, agentReq} {
		if _, err := svc.Bind(ctx, req); err != nil {
			t.Fatalf("bind %+v: %v", req, err)
		}
	}

	resolver := skillorchestrator.NewBindingResolver()
	assertResolvedVersion := func(expected string) {
		t.Helper()
		items, err := svc.List(ctx, skillorchestrator.ResolveRequest{
			SkillID: "bid.collect", ProjectID: "p1", AgentID: "a1",
		})
		if err != nil {
			t.Fatalf("list bindings: %v", err)
		}
		resolved, ok := resolver.Resolve(items, skillorchestrator.ResolveRequest{
			SkillID: "bid.collect", ProjectID: "p1", AgentID: "a1",
		})
		if !ok {
			t.Fatalf("expected resolved binding")
		}
		if resolved.Version != expected {
			t.Fatalf("expected version %s, got %s", expected, resolved.Version)
		}
	}

	assertResolvedVersion("v2.0.0")
	if err := svc.Unbind(ctx, agentReq); err != nil {
		t.Fatalf("unbind agent-local: %v", err)
	}
	assertResolvedVersion("v1.1.0")
	if err := svc.Unbind(ctx, projectReq); err != nil {
		t.Fatalf("unbind project: %v", err)
	}
	assertResolvedVersion("v1.0.0")
}
