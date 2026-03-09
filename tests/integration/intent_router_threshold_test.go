package integration

import (
	"context"
	"testing"
	"time"

	"synapsex/internal/application/intent"
	"synapsex/internal/application/service"
	"synapsex/internal/application/skillregistry"
	skilldomain "synapsex/internal/domain/skill"
	"synapsex/internal/infrastructure/config"
	"synapsex/internal/infrastructure/persistence"
	skillsinfra "synapsex/internal/infrastructure/skills"
	chatiface "synapsex/internal/interfaces/chat"
)

func TestIntentRouterThresholdFallbackToTask(t *testing.T) {
	root := t.TempDir()
	writeSkillFile(t, root, "user-skills/analyze/SKILL.md", `---
name: analyze
description: analyze text
---
执行分析`)

	registry, err := skillregistry.New(skillregistry.Config{
		Sources:    []skillsinfra.SourceSpec{{Source: skilldomain.SourceUser, Root: root + "/user-skills"}},
		IndexPath:  root + "/skills_index.json",
		RefreshNow: true,
	})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	policy := intent.PermissionPolicy{
		Enabled:       true,
		AllowUsers:    map[string]struct{}{"user-1": {}},
		AllowChannels: map[string]struct{}{"-": {}},
		DefaultMode:   "channel_allowlist_dm_pairing",
		PairingTTL:    24 * time.Hour,
	}
	pipeline := intent.NewPipeline(registry, intent.NewPermissionChecker(policy, nil), lowConfidenceFallback{}, 0.72)

	cfg := config.Snapshot{
		AllowedRoots:   []string{"."},
		DefaultCWD:     ".",
		Timeout:        5 * time.Second,
		ExecCommand:    "cat",
		DiscordEnabled: true,
	}
	store := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(store, store, nil)
	router := service.NewRouter(cfg, manager, fakeBackend{name: "threshold"}, service.WithIntentPipeline(pipeline))

	message := mustNormalizeMessage(t, chatiface.NormalizeInput{
		Channel:         "discord",
		UserID:          "user-1",
		Text:            "please analyze this quickly",
		IsDirectMessage: true,
		IsAllowed:       true,
	})
	decision, err := router.Route(context.Background(), message)
	if err != nil {
		t.Fatalf("route: %v", err)
	}
	if decision.Kind != service.DecisionExecute {
		t.Fatalf("expected task fallback execute, got %s", decision.Kind)
	}
}

type lowConfidenceFallback struct{}

func (lowConfidenceFallback) Match(_ context.Context, _ string, _ skilldomain.RegistrySnapshot) (intent.Candidate, bool, error) {
	return intent.Candidate{
		SkillName:  "analyze",
		Reason:     "llm_fallback",
		Confidence: 0.31,
	}, true, nil
}
