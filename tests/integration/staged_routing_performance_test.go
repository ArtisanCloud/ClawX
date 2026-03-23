package integration

import (
	"strings"
	"testing"
	"time"

	"clawx/internal/application/skillorchestrator"
)

func TestStagedRoutingPerformanceBaseline(t *testing.T) {
	router := skillorchestrator.NewStagedRouter()

	lowComplexity := "现在有多少个智能体？"
	highComplexity := strings.Repeat("请先梳理技能清单并给出执行计划，然后列出风险与回退方案。", 8)

	lowResult := router.Plan(skillorchestrator.StagedRoutingInput{Message: lowComplexity, CurrentAgentID: "main", ProjectID: "main"})
	highResult := router.Plan(skillorchestrator.StagedRoutingInput{Message: highComplexity, CurrentAgentID: "main", ProjectID: "main"})

	if !lowResult.SkipRoutePlanner {
		t.Fatalf("expected low complexity request to skip route planner")
	}
	if highResult.SkipRoutePlanner {
		t.Fatalf("expected high complexity request to keep route planner")
	}

	lowBefore, lowAfter := estimatePipelineBudget(lowResult)
	highBefore, highAfter := estimatePipelineBudget(highResult)

	if lowAfter.tokens >= lowBefore.tokens {
		t.Fatalf("expected token reduction on low complexity route: before=%d after=%d", lowBefore.tokens, lowAfter.tokens)
	}
	if lowAfter.latencyMS >= lowBefore.latencyMS {
		t.Fatalf("expected latency reduction on low complexity route: before=%d after=%d", lowBefore.latencyMS, lowAfter.latencyMS)
	}
	if highAfter.tokens != highBefore.tokens || highAfter.latencyMS != highBefore.latencyMS {
		t.Fatalf("high complexity route should not change budget when not short-circuited")
	}

	// runtime gate: staged planner should stay lightweight.
	const samples = 2000
	started := time.Now()
	for i := 0; i < samples; i++ {
		_ = router.Plan(skillorchestrator.StagedRoutingInput{
			Message:        lowComplexity,
			CurrentAgentID: "main",
			ProjectID:      "main",
		})
	}
	avgMicros := time.Since(started).Microseconds() / samples
	if avgMicros > 3000 {
		t.Fatalf("staged router average planning latency too high: %dµs", avgMicros)
	}
}

type pipelineBudget struct {
	tokens    int
	latencyMS int
}

func estimatePipelineBudget(result skillorchestrator.StagedRoutingResult) (before pipelineBudget, after pipelineBudget) {
	// Baseline: planner + route-planner + executor
	before = pipelineBudget{tokens: 120 + 180 + 260, latencyMS: 40 + 55 + 90}
	after = before
	if result.SkipRoutePlanner {
		after.tokens -= 180
		after.latencyMS -= 55
	}
	return before, after
}
