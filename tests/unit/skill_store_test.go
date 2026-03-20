package unit

import (
	"context"
	"path/filepath"
	"testing"

	skilldomain "clawx/internal/domain/skill"
	"clawx/internal/infrastructure/persistence"
)

func TestSkillRegistryFileStoreCRUD(t *testing.T) {
	store, err := persistence.NewSkillRegistryFileStore(filepath.Join(t.TempDir(), "registry.json"))
	if err != nil {
		t.Fatalf("new registry store: %v", err)
	}

	ctx := context.Background()
	item := skilldomain.SkillMetadata{
		SkillID:      "bid.collect",
		Version:      "v1.0.0",
		Source:       skilldomain.RegistrySourceBuiltin,
		Enabled:      true,
		InputSchema:  map[string]any{"type": "object"},
		RiskLevel:    skilldomain.RiskLow,
		Capabilities: []string{"crawl"},
	}
	if err := store.Upsert(ctx, item); err != nil {
		t.Fatalf("upsert registry: %v", err)
	}
	loaded, err := store.GetByID(ctx, "bid.collect")
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if loaded.SkillID != "bid.collect" {
		t.Fatalf("unexpected skill id: %q", loaded.SkillID)
	}
	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list registry: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("unexpected list size: %d", len(listed))
	}
	if err := store.Delete(ctx, "bid.collect"); err != nil {
		t.Fatalf("delete registry: %v", err)
	}
	listed, err = store.List(ctx)
	if err != nil {
		t.Fatalf("list registry after delete: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("registry should be empty after delete")
	}
}

func TestSkillPolicyFileStoreSaveLoad(t *testing.T) {
	store, err := persistence.NewSkillPolicyFileStore(filepath.Join(t.TempDir(), "policy.json"))
	if err != nil {
		t.Fatalf("new policy store: %v", err)
	}
	ctx := context.Background()
	policy := skilldomain.SkillPolicy{
		AllowedSources:  []skilldomain.RegistrySource{skilldomain.RegistrySourceBuiltin},
		VersionStrategy: skilldomain.VersionStrategyLatest,
	}
	if err := store.Save(ctx, policy); err != nil {
		t.Fatalf("save policy: %v", err)
	}
	loaded, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	if !loaded.SourceAllowed(skilldomain.RegistrySourceBuiltin) {
		t.Fatalf("builtin source should be allowed")
	}
	if loaded.SourceAllowed(skilldomain.RegistrySourceClawHub) {
		t.Fatalf("clawhub source should be rejected")
	}
}

func TestSkillBindingFileStoreCRUD(t *testing.T) {
	store, err := persistence.NewSkillBindingFileStore(filepath.Join(t.TempDir(), "bindings.json"))
	if err != nil {
		t.Fatalf("new binding store: %v", err)
	}
	ctx := context.Background()
	binding := skilldomain.SkillBinding{
		SkillID:   "bid.collect",
		Version:   "v1.0.0",
		Scope:     skilldomain.ScopeProject,
		ProjectID: "bid",
	}
	if err := store.Upsert(ctx, binding); err != nil {
		t.Fatalf("upsert binding: %v", err)
	}
	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("unexpected binding size: %d", len(listed))
	}
	if err := store.Delete(ctx, binding); err != nil {
		t.Fatalf("delete binding: %v", err)
	}
	listed, err = store.List(ctx)
	if err != nil {
		t.Fatalf("list bindings after delete: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("bindings should be empty after delete")
	}
}
