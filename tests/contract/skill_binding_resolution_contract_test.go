package contract

import (
	"testing"

	"clawx/internal/application/skillorchestrator"
	skilldomain "clawx/internal/domain/skill"
)

func TestSkillBindingResolutionContract(t *testing.T) {
	resolver := skillorchestrator.NewBindingResolver()
	bindings := []skilldomain.SkillBinding{
		{SkillID: "bid.collect", Version: "v1.0.0", Scope: skilldomain.ScopeGlobal},
		{SkillID: "bid.collect", Version: "v1.1.0", Scope: skilldomain.ScopeProject, ProjectID: "p1"},
		{SkillID: "bid.collect", Version: "v2.0.0", Scope: skilldomain.ScopeAgentLocal, AgentID: "a1"},
	}
	resolved, ok := resolver.Resolve(bindings, skillorchestrator.ResolveRequest{
		SkillID:   "bid.collect",
		ProjectID: "p1",
		AgentID:   "a1",
	})
	if !ok {
		t.Fatalf("expected resolved binding")
	}
	if resolved.Scope != skilldomain.ScopeAgentLocal {
		t.Fatalf("expected agent-local priority, got %s", resolved.Scope)
	}
	if resolved.Version != "v2.0.0" {
		t.Fatalf("unexpected resolved version: %s", resolved.Version)
	}
}
