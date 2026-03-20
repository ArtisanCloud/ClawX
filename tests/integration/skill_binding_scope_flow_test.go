package integration

import (
	"context"
	"path/filepath"
	"testing"

	"clawx/internal/application/skillorchestrator"
	skilldomain "clawx/internal/domain/skill"
	"clawx/internal/infrastructure/persistence"
)

func TestSkillBindingScopeFlowIntegration(t *testing.T) {
	dir := t.TempDir()
	registryStore, err := persistence.NewSkillRegistryFileStore(filepath.Join(dir, "registry.json"))
	if err != nil {
		t.Fatalf("new registry store: %v", err)
	}
	bindingStore, err := persistence.NewSkillBindingFileStore(filepath.Join(dir, "bindings.json"))
	if err != nil {
		t.Fatalf("new binding store: %v", err)
	}

	registry := skillorchestrator.NewRegistryService(registryStore)
	binder := skillorchestrator.NewBindingService(bindingStore, registry)
	executor := skillorchestrator.NewExecutor(bindingStore, skillorchestrator.NewPolicyEngine(nil), skillorchestrator.NewRiskGuard(0), skillorchestrator.NewAuditService())
	ctx := context.Background()

	if err := registry.Register(ctx, skilldomain.SkillMetadata{
		SkillID: "bid.collect", Version: "v2.0.0", Source: skilldomain.RegistrySourceBuiltin,
		Enabled: true, InputSchema: map[string]any{"type": "object"}, RiskLevel: skilldomain.RiskLow,
	}); err != nil {
		t.Fatalf("register metadata: %v", err)
	}
	for _, req := range []skillorchestrator.BindRequest{
		{SkillID: "bid.collect", Version: "v1.0.0", Scope: skilldomain.ScopeGlobal},
		{SkillID: "bid.collect", Version: "v2.0.0", Scope: skilldomain.ScopeAgentLocal, AgentID: "agent-a"},
	} {
		if _, err := binder.Bind(ctx, req); err != nil {
			t.Fatalf("bind: %v", err)
		}
	}

	resultA, err := executor.Execute(ctx, skillorchestrator.ExecuteRequest{
		Action: skilldomain.SkillAction{Intent: "run_skill", SkillID: "bid.collect", Confidence: 1, RiskLevel: skilldomain.RiskLow, Source: "test"},
		Metadata: skilldomain.SkillMetadata{
			SkillID: "bid.collect", Version: "v2.0.0", Source: skilldomain.RegistrySourceBuiltin,
			Enabled: true, InputSchema: map[string]any{"type": "object"}, RiskLevel: skilldomain.RiskLow,
		},
		ProjectID: "p1",
		AgentID:   "agent-a",
		Actor:     "agent-a",
	})
	if err != nil {
		t.Fatalf("execute agent-a: %v", err)
	}
	if resultA.Binding == nil || resultA.Binding.Scope != skilldomain.ScopeAgentLocal {
		t.Fatalf("agent-a should resolve agent-local binding")
	}

	resultB, err := executor.Execute(ctx, skillorchestrator.ExecuteRequest{
		Action: skilldomain.SkillAction{Intent: "run_skill", SkillID: "bid.collect", Confidence: 1, RiskLevel: skilldomain.RiskLow, Source: "test"},
		Metadata: skilldomain.SkillMetadata{
			SkillID: "bid.collect", Version: "v1.0.0", Source: skilldomain.RegistrySourceBuiltin,
			Enabled: true, InputSchema: map[string]any{"type": "object"}, RiskLevel: skilldomain.RiskLow,
		},
		ProjectID: "p1",
		AgentID:   "agent-b",
		Actor:     "agent-b",
	})
	if err != nil {
		t.Fatalf("execute agent-b: %v", err)
	}
	if resultB.Binding == nil || resultB.Binding.Scope != skilldomain.ScopeGlobal {
		t.Fatalf("agent-b should fallback to global binding")
	}
}
