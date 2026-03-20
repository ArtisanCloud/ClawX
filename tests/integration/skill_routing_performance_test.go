package integration

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"clawx/internal/application/skillorchestrator"
	skilldomain "clawx/internal/domain/skill"
)

func TestSkillRoutingPerformanceWithDigests(t *testing.T) {
	router := skillorchestrator.NewRouterFromEnv()
	catalogBuilder := skillorchestrator.NewCatalogDigestBuilder()
	projector := skillorchestrator.NewContextDigestProjector(50)

	metadata := make([]skilldomain.SkillMetadata, 0, 200)
	bindings := make([]skilldomain.SkillBinding, 0, 200)
	for i := 0; i < 200; i++ {
		skillID := fmt.Sprintf("skill.%03d", i)
		metadata = append(metadata, skilldomain.SkillMetadata{
			SkillID:   skillID,
			Version:   "v1.0.0",
			Source:    skilldomain.RegistrySourceBuiltin,
			Enabled:   true,
			RiskLevel: skilldomain.RiskLow,
		})
		bindings = append(bindings, skilldomain.SkillBinding{
			SkillID: skillID,
			Version: "v1.0.0",
			Scope:   skilldomain.ScopeGlobal,
		})
	}
	catalog := catalogBuilder.Build(metadata, bindings)

	now := time.Now().UTC()
	records := make([]skillorchestrator.AuditRecord, 0, 50)
	for i := 0; i < 50; i++ {
		records = append(records, skillorchestrator.AuditRecord{
			TraceID:        fmt.Sprintf("trace-%03d", i),
			EventType:      skilldomain.AuditEventExec,
			ConversationID: "conv-perf",
			Actor:          "u1",
			SkillID:        fmt.Sprintf("skill.%03d", i%10),
			Source:         "nl",
			Intent:         "install_skill",
			Result:         "success",
			OccurredAt:     now.Add(time.Duration(i) * time.Millisecond),
		})
	}
	contextDigest := projector.Project("conv-perf", records)

	samples := make([]int64, 0, 200)
	for i := 0; i < 200; i++ {
		started := time.Now()
		decision, err := router.Route(skillorchestrator.RouteInput{
			Message:          "请安装技能 bid.collect",
			ContextDigest:    contextDigest,
			SkillCatalogHash: catalog.Hash,
		})
		if err != nil {
			t.Fatalf("route failed: %v", err)
		}
		if !decision.Matched {
			t.Fatalf("expected matched decision")
		}
		samples = append(samples, time.Since(started).Milliseconds())
	}

	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	median := samples[len(samples)/2]
	if median > 300 {
		t.Fatalf("median routing time too slow: %dms", median)
	}
}
