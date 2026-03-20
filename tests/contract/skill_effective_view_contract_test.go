package contract

import (
	"context"
	"path/filepath"
	"testing"

	"clawx/internal/application/skillorchestrator"
	skilldomain "clawx/internal/domain/skill"
	"clawx/internal/infrastructure/persistence"
)

func TestSkillEffectiveViewContract(t *testing.T) {
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
	effective := skillorchestrator.NewEffectiveViewService(bindingStore, registry)
	ctx := context.Background()

	for _, metadata := range []skilldomain.SkillMetadata{
		{SkillID: "bid.collect", Version: "v2.0.0", Source: skilldomain.RegistrySourceBuiltin, Enabled: true, InputSchema: map[string]any{"type": "object"}, RiskLevel: skilldomain.RiskLow},
		{SkillID: "news.sync", Version: "v1.0.0", Source: skilldomain.RegistrySourceBuiltin, Enabled: true, InputSchema: map[string]any{"type": "object"}, RiskLevel: skilldomain.RiskLow},
	} {
		if err := registry.Register(ctx, metadata); err != nil {
			t.Fatalf("register metadata: %v", err)
		}
	}
	for _, req := range []skillorchestrator.BindRequest{
		{SkillID: "bid.collect", Version: "v1.0.0", Scope: skilldomain.ScopeGlobal},
		{SkillID: "bid.collect", Version: "v1.1.0", Scope: skilldomain.ScopeProject, ProjectID: "p1"},
		{SkillID: "bid.collect", Version: "v2.0.0", Scope: skilldomain.ScopeAgentLocal, AgentID: "a1"},
		{SkillID: "news.sync", Version: "v1.0.0", Scope: skilldomain.ScopeGlobal},
	} {
		if _, err := binder.Bind(ctx, req); err != nil {
			t.Fatalf("bind: %v", err)
		}
	}

	view, err := effective.ListEffective(ctx, skillorchestrator.ResolveRequest{ProjectID: "p1", AgentID: "a1"})
	if err != nil {
		t.Fatalf("list effective: %v", err)
	}
	if len(view) != 2 {
		t.Fatalf("expected 2 effective skills, got %d", len(view))
	}
	collect, ok := findEffective(view, "bid.collect")
	if !ok {
		t.Fatalf("missing bid.collect in effective view")
	}
	if collect.Scope != skilldomain.ScopeAgentLocal || collect.Version != "v2.0.0" {
		t.Fatalf("unexpected collect resolution: scope=%s version=%s", collect.Scope, collect.Version)
	}
}

func findEffective(values []skillorchestrator.EffectiveSkill, skillID string) (skillorchestrator.EffectiveSkill, bool) {
	for _, item := range values {
		if item.SkillID == skillID {
			return item, true
		}
	}
	return skillorchestrator.EffectiveSkill{}, false
}
