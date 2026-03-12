package contract

import (
	"context"
	"fmt"
	"testing"
	"time"

	"clawx/internal/application/intent"
	skilldomain "clawx/internal/domain/skill"
	chatiface "clawx/internal/interfaces/chat"
)

func TestIntentRouterContractPriority(t *testing.T) {
	registry := stubRegistry{
		snapshot: skilldomain.RegistrySnapshot{
			Version: 1,
			Entries: []skilldomain.CatalogEntry{
				{
					Key:       "user:echo:/tmp/echo",
					SkillName: "echo",
					Source:    skilldomain.SourceUser,
					Status:    skilldomain.StatusActive,
					Definition: &skilldomain.Definition{
						Name:            "echo",
						Description:     "echo text",
						InstructionBody: "echo",
						Source:          skilldomain.SourceUser,
						BaseDir:         "/tmp/echo",
						ManifestPath:    "/tmp/echo/SKILL.md",
						LoadedAt:        time.Now().UTC(),
					},
				},
			},
			GeneratedAt: time.Now().UTC(),
		},
	}
	checker := intent.NewPermissionChecker(intent.PermissionPolicy{
		Enabled:       true,
		AllowUsers:    map[string]struct{}{"user-1": {}},
		AllowChannels: map[string]struct{}{"-": {}},
		DefaultMode:   "channel_allowlist_dm_pairing",
		PairingTTL:    24 * time.Hour,
	}, nil)
	pipeline := intent.NewPipeline(registry, checker, intent.DefaultLLMFallback{}, 0.72)

	controlMessage := chatiface.Message{
		ConversationID: "discord:-:-:user-1",
		UserID:         "user-1",
		Text:           "/new",
		Channel:        "discord",
		ContextFlags: chatiface.ContextFlags{
			IsAllowed: true,
		},
	}
	result, err := pipeline.Decide(context.Background(), controlMessage)
	if err != nil {
		t.Fatalf("decide control: %v", err)
	}
	if result.Decision.Kind != skilldomain.IntentControl {
		t.Fatalf("expected control, got %s", result.Decision.Kind)
	}

	explicitMessage := controlMessage
	explicitMessage.Text = "/skill echo hi"
	result, err = pipeline.Decide(context.Background(), explicitMessage)
	if err != nil {
		t.Fatalf("decide explicit: %v", err)
	}
	if result.Decision.Kind != skilldomain.IntentSkill || result.Decision.SkillName != "echo" {
		t.Fatalf("expected explicit skill echo, got kind=%s skill=%s", result.Decision.Kind, result.Decision.SkillName)
	}
}

type stubRegistry struct {
	snapshot skilldomain.RegistrySnapshot
}

func (s stubRegistry) Snapshot() skilldomain.RegistrySnapshot {
	return s.snapshot
}

func (s stubRegistry) FindEntry(name string) (skilldomain.CatalogEntry, bool) {
	return s.snapshot.FindEntryByName(name)
}

func (s stubRegistry) FindActiveDefinition(name string) (skilldomain.Definition, error) {
	if def, ok := s.snapshot.FindActiveDefinition(name); ok {
		return def, nil
	}
	return skilldomain.Definition{}, fmt.Errorf("not found")
}
