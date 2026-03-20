package unit

import (
	"testing"

	"clawx/internal/application/skillorchestrator"
	skilldomain "clawx/internal/domain/skill"
)

func TestSkillCatalogDigestConsistency(t *testing.T) {
	builder := skillorchestrator.NewCatalogDigestBuilder()
	metaA := []skilldomain.SkillMetadata{
		{SkillID: "bid.collect", Version: "v1.0.0", Source: skilldomain.RegistrySourceBuiltin, Enabled: true},
		{SkillID: "news.sync", Version: "v2.0.0", Source: skilldomain.RegistrySourceClawHub, Enabled: false},
	}
	bindingA := []skilldomain.SkillBinding{
		{SkillID: "bid.collect", Version: "v1.0.0", Scope: skilldomain.ScopeGlobal},
		{SkillID: "news.sync", Version: "v2.0.0", Scope: skilldomain.ScopeAgentLocal, AgentID: "a1"},
	}

	metaB := []skilldomain.SkillMetadata{metaA[1], metaA[0]}
	bindingB := []skilldomain.SkillBinding{bindingA[1], bindingA[0]}

	digestA := builder.Build(metaA, bindingA)
	digestB := builder.Build(metaB, bindingB)
	if digestA.Hash == "" {
		t.Fatalf("catalog digest should not be empty")
	}
	if digestA.Hash != digestB.Hash {
		t.Fatalf("catalog digest should be stable across ordering: %s != %s", digestA.Hash, digestB.Hash)
	}
}
