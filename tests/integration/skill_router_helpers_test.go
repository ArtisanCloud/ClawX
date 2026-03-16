package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"clawx/internal/application/intent"
	"clawx/internal/application/service"
	"clawx/internal/application/skillregistry"
	"clawx/internal/domain/execution"
	skilldomain "clawx/internal/domain/skill"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/persistence"
	skillsinfra "clawx/internal/infrastructure/skills"
	chatiface "clawx/internal/interfaces/chat"
)

type skillTestStack struct {
	cfg      config.Snapshot
	registry *skillregistry.Service
	router   *service.Router
}

func newSkillTestStack(t *testing.T, setup func(root string) error, disabled []string) skillTestStack {
	t.Helper()

	root := t.TempDir()
	userSkills := filepath.Join(root, "user-skills")
	workspace := filepath.Join(root, "workspace")
	workspaceSkills := filepath.Join(workspace, ".clawx", "skills")
	indexPath := filepath.Join(root, "skills_index.json")
	if err := os.MkdirAll(userSkills, 0o755); err != nil {
		t.Fatalf("mkdir user skills: %v", err)
	}
	if err := os.MkdirAll(workspaceSkills, 0o755); err != nil {
		t.Fatalf("mkdir workspace skills: %v", err)
	}
	if setup != nil {
		if err := setup(root); err != nil {
			t.Fatalf("setup skills: %v", err)
		}
	}

	cfg := config.Snapshot{
		AllowedRoots:   []string{workspace},
		DefaultCWD:     workspace,
		Timeout:        5 * time.Second,
		ExecCommand:    "cat",
		DiscordEnabled: true,
		Skills: config.SkillConfig{
			Enabled: true,
			Sources: config.SkillSources{
				UserDir:        userSkills,
				WorkspaceDir:   ".clawx/skills",
				BuiltinEnabled: false,
			},
			DisabledNames: disabled,
			Allowlist: config.SkillAllowlist{
				Users:    []string{"user-1"},
				Channels: []string{"-"},
			},
			DefaultMode: "channel_allowlist_dm_pairing",
			PairingTTL:  24 * time.Hour,
		},
		IntentRouter: config.IntentRouterConfig{
			Mode: "rule_first_llm_fallback",
			LLMFallback: config.LLMFallbackConfig{
				Enabled:             true,
				ConfidenceThreshold: 0.72,
			},
		},
	}

	registry, err := skillregistry.New(skillregistry.Config{
		Sources: []skillsinfra.SourceSpec{
			{Source: skilldomain.SourceUser, Root: userSkills},
			{Source: skilldomain.SourceWorkspace, Root: workspaceSkills},
		},
		IndexPath:  indexPath,
		Disabled:   disabled,
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
	pipeline := intent.NewPipeline(registry, intent.NewPermissionChecker(policy, nil), intent.DefaultLLMFallback{}, 0.72)

	store := persistence.NewSessionMemoryRepository()
	manager := service.NewSessionManager(store, store, nil)
	router := service.NewRouter(cfg, manager, fakeBackend{name: "test-backend"}, service.WithIntentPipeline(pipeline))

	return skillTestStack{cfg: cfg, registry: registry, router: router}
}

func writeSkillFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir skill path: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
}

func mustNormalizeMessage(t *testing.T, input chatiface.NormalizeInput) chatiface.Message {
	t.Helper()
	message, err := chatiface.NormalizeInboundMessage(input)
	if err != nil {
		t.Fatalf("normalize message: %v", err)
	}
	return message
}

type fakeBackend struct {
	name string
}

func (b fakeBackend) Name() string {
	if b.name == "" {
		return "fake"
	}
	return b.name
}

func (b fakeBackend) Execute(_ context.Context, request execution.Request) (execution.Result, error) {
	now := time.Now().UTC()
	return execution.Result{
		BackendSessionID: "backend-" + request.SessionID,
		Output:           request.Input,
		State:            execution.ResultSuccess,
		StartedAt:        now,
		CompletedAt:      now,
	}, nil
}

func (b fakeBackend) Cancel(_ context.Context, _ string) error {
	return nil
}

func (b fakeBackend) HealthCheck(_ context.Context) error {
	return nil
}
