package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"clawx/internal/application/skillorchestrator"
	skilldomain "clawx/internal/domain/skill"
	"clawx/internal/infrastructure/persistence"
	chatiface "clawx/internal/interfaces/chat"
)

func TestHandleSkillChatCommandNaturalLanguageRoute(t *testing.T) {
	runtime := agentRuntime{
		agentID:      "bid-all",
		skillControl: newSkillRuntimeForTests(t).Runtime,
	}
	message := chatiface.Message{
		ConversationID: "conv-skill-nl",
		UserID:         "user-1",
		Text:           "请安装技能 bid.collect 到 bid-all",
	}
	handled, response, err := handleSkillChatCommand(message, runtime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatalf("expected natural language to fall through to LLM pipeline")
	}
	_ = response
}

func TestHandleSkillChatCommandCommandMapping(t *testing.T) {
	runtime := agentRuntime{
		agentID:      "bid-all",
		skillControl: newSkillRuntimeForTests(t).Runtime,
	}
	message := chatiface.Message{
		ConversationID: "conv-skill-cmd",
		UserID:         "user-1",
		Text:           "/skill install bid.collect --agent bid-all",
	}
	handled, response, err := handleSkillChatCommand(message, runtime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled")
	}
	if !strings.Contains(response, "技能安装成功：bid.collect") {
		t.Fatalf("unexpected response: %s", response)
	}
}

func TestHandleSkillChatCommandLowConfidenceClarify(t *testing.T) {
	runtime := agentRuntime{
		agentID:      "bid-all",
		skillControl: newSkillRuntimeForTests(t).Runtime,
	}
	message := chatiface.Message{
		ConversationID: "conv-skill-low",
		UserID:         "user-1",
		Text:           "技能怎么配置比较好？",
	}
	handled, response, err := handleSkillChatCommand(message, runtime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatalf("expected natural language to fall through to LLM pipeline")
	}
	_ = response
}

func TestHandleSkillChatCommandNonIntentFallback(t *testing.T) {
	runtime := agentRuntime{
		agentID:      "bid-all",
		skillControl: newSkillRuntimeForTests(t).Runtime,
	}
	message := chatiface.Message{
		ConversationID: "conv-skill-fallback",
		UserID:         "user-1",
		Text:           "请帮我跑一下 go test ./...",
	}
	handled, _, err := handleSkillChatCommand(message, runtime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatalf("expected not handled for non-skill intent")
	}
}

func TestHandleSkillChatCommandRegistryMetadataValidation(t *testing.T) {
	runtime := newSkillRuntimeForTests(t)
	message := chatiface.Message{
		ConversationID: "conv-skill-metadata",
		UserID:         "user-1",
		Text:           "/skill install ",
	}
	handled, _, err := handleSkillChatCommand(message, agentRuntime{
		agentID:      "bid-all",
		skillControl: runtime.Runtime,
	})
	if !handled {
		t.Fatalf("expected handled")
	}
	if err == nil {
		t.Fatalf("expected validation error for missing skill id")
	}
}

func TestHandleSkillChatCommandPolicySourceReject(t *testing.T) {
	runtime := newSkillRuntimeForTests(t)
	ctx := context.Background()
	policyStore := mustNewPolicyStoreForTest(t)
	if err := policyStore.Save(ctx, skilldomain.SkillPolicy{
		AllowedSources:  []skilldomain.RegistrySource{skilldomain.RegistrySourceBuiltin},
		VersionStrategy: skilldomain.VersionStrategyLatest,
	}); err != nil {
		t.Fatalf("save policy: %v", err)
	}
	runtime.PolicyEngine = skillorchestrator.NewPolicyEngine(policyStore)
	runtime.Installer = skillorchestrator.NewInstallService(runtime.Registry, runtime.PolicyEngine, skillorchestrator.NewMapAgentStateProvider(nil))

	message := chatiface.Message{
		ConversationID: "conv-skill-source-reject",
		UserID:         "user-1",
		Text:           "/skill install bid.collect --source clawhub --version v1.0.0",
	}
	handled, _, err := handleSkillChatCommand(message, agentRuntime{
		agentID:      "bid-all",
		skillControl: runtime.Runtime,
	})
	if !handled {
		t.Fatalf("expected handled")
	}
	if err == nil || !strings.Contains(err.Error(), "策略拒绝") {
		t.Fatalf("expected policy reject error, got: %v", err)
	}
}

func TestHandleSkillChatCommandPolicyVersionReject(t *testing.T) {
	runtime := newSkillRuntimeForTests(t)
	ctx := context.Background()
	policyStore := mustNewPolicyStoreForTest(t)
	if err := policyStore.Save(ctx, skilldomain.SkillPolicy{
		AllowedSources:  []skilldomain.RegistrySource{skilldomain.RegistrySourceBuiltin},
		VersionStrategy: skilldomain.VersionStrategyPin,
		PinnedVersions:  map[string]string{"bid.collect": "v1.0.0"},
	}); err != nil {
		t.Fatalf("save policy: %v", err)
	}
	runtime.PolicyEngine = skillorchestrator.NewPolicyEngine(policyStore)
	runtime.Installer = skillorchestrator.NewInstallService(runtime.Registry, runtime.PolicyEngine, skillorchestrator.NewMapAgentStateProvider(nil))

	message := chatiface.Message{
		ConversationID: "conv-skill-version-reject",
		UserID:         "user-1",
		Text:           "/skill install bid.collect --source builtin --version v2.0.0",
	}
	handled, _, err := handleSkillChatCommand(message, agentRuntime{
		agentID:      "bid-all",
		skillControl: runtime.Runtime,
	})
	if !handled {
		t.Fatalf("expected handled")
	}
	if err == nil || !strings.Contains(err.Error(), "策略拒绝") {
		t.Fatalf("expected version policy reject error, got: %v", err)
	}
}

func TestHandleSkillChatCommandDisableImmediateEffect(t *testing.T) {
	runtime := newSkillRuntimeForTests(t)
	ctx := context.Background()
	// Install once.
	_, _, err := handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-disable",
		UserID:         "user-1",
		Text:           "/skill install bid.collect --source builtin --version v1.0.0",
	}, agentRuntime{agentID: "bid-all", skillControl: runtime.Runtime})
	if err != nil {
		t.Fatalf("install skill: %v", err)
	}
	// Disable immediately.
	handled, response, err := handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-disable",
		UserID:         "user-1",
		Text:           "/skill disable bid.collect",
	}, agentRuntime{agentID: "bid-all", skillControl: runtime.Runtime})
	if err != nil || !handled {
		t.Fatalf("disable skill failed: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(response, "技能已禁用") {
		t.Fatalf("unexpected disable response: %s", response)
	}
	// Verify policy reflects disabled state.
	policy, err := runtime.TogglePolicyStore.Load(ctx)
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	if !policy.SkillDisabled("bid.collect") {
		t.Fatalf("skill should be disabled immediately")
	}
}

func TestHandleSkillChatCommandAgentStateProvider(t *testing.T) {
	runtime := newSkillRuntimeWithAgentProviderForTests(t)
	handled, response, err := handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-agent-state",
		UserID:         "user-1",
		Text:           "/skill install bid.collect --source builtin --version v1.0.0 --agent bid-all",
	}, agentRuntime{agentID: "bid-all", skillControl: runtime.Runtime})
	if err != nil || !handled {
		t.Fatalf("install with agent provider failed: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(response, "workspace=") || !strings.Contains(response, "profile=") {
		t.Fatalf("expected agent state in response, got: %s", response)
	}
}

func TestHandleSkillChatCommandBindScopeFallback(t *testing.T) {
	runtime := newSkillRuntimeForTests(t)
	// Install and bind global first.
	_, _, err := handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-bind-fallback",
		UserID:         "user-1",
		Text:           "/skill install bid.collect --source builtin --version v1.0.0",
	}, agentRuntime{agentID: "agent-b", skillControl: runtime.Runtime})
	if err != nil {
		t.Fatalf("install global candidate: %v", err)
	}
	handled, response, err := handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-bind-fallback",
		UserID:         "user-1",
		Text:           "/skill bind bid.collect --scope global --version v1.0.0",
	}, agentRuntime{agentID: "agent-b", skillControl: runtime.Runtime})
	if err != nil || !handled {
		t.Fatalf("bind global failed: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(response, "scope=global") {
		t.Fatalf("unexpected global bind response: %s", response)
	}
	// Bind higher-priority agent-local.
	handled, response, err = handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-bind-fallback",
		UserID:         "user-1",
		Text:           "/skill bind bid.collect --scope agent-local --agent agent-a",
	}, agentRuntime{agentID: "agent-a", skillControl: runtime.Runtime})
	if err != nil || !handled {
		t.Fatalf("bind agent-local failed: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(response, "scope=agent-local") {
		t.Fatalf("unexpected agent-local bind response: %s", response)
	}

	resolvedA, ok := runtime.Binder.Resolve(context.Background(), skillorchestrator.ResolveRequest{
		SkillID: "bid.collect", AgentID: "agent-a",
	})
	if !ok || resolvedA.Scope != skilldomain.ScopeAgentLocal {
		t.Fatalf("agent-a should resolve agent-local binding")
	}
	resolvedB, ok := runtime.Binder.Resolve(context.Background(), skillorchestrator.ResolveRequest{
		SkillID: "bid.collect", AgentID: "agent-b",
	})
	if !ok || resolvedB.Scope != skilldomain.ScopeGlobal {
		t.Fatalf("agent-b should resolve global binding")
	}
}

func TestHandleSkillChatCommandRiskConfirmRequired(t *testing.T) {
	runtime := newSkillRuntimeForTests(t)
	prepareRunnableSkill(t, runtime)
	handled, _, err := handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-risk-required",
		UserID:         "user-risk",
		Text:           "/skill run bid.collect --version v1.0.0 --risk high --agent bid-all",
	}, agentRuntime{agentID: "bid-all", skillControl: runtime.Runtime})
	if !handled {
		t.Fatalf("expected handled")
	}
	if err == nil || !strings.Contains(err.Error(), "需确认") || !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("expected confirmation required error, got: %v", err)
	}
}

func TestHandleSkillChatCommandConfirmRejectNoSideEffect(t *testing.T) {
	runtime := newSkillRuntimeForTests(t)
	prepareRunnableSkill(t, runtime)
	_, _, err := handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-risk-reject",
		UserID:         "user-risk",
		Text:           "/skill run bid.collect --version v1.0.0 --risk high --agent bid-all",
	}, agentRuntime{agentID: "bid-all", skillControl: runtime.Runtime})
	if err == nil {
		t.Fatalf("expected confirmation required")
	}
	confirmationID := extractConfirmationID(t, err.Error())
	handled, _, err := handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-risk-reject",
		UserID:         "user-risk",
		Text:           "/skill run bid.collect --version v1.0.0 --risk high --agent bid-all --reject " + confirmationID,
	}, agentRuntime{agentID: "bid-all", skillControl: runtime.Runtime})
	if !handled {
		t.Fatalf("expected handled reject")
	}
	if err == nil || !strings.Contains(err.Error(), "未执行任何副作用") {
		t.Fatalf("expected reject no-side-effect error, got: %v", err)
	}
}

func TestHandleSkillChatCommandAuditReplayConsistency(t *testing.T) {
	runtime := newSkillRuntimeForTests(t)
	prepareRunnableSkill(t, runtime)
	_, _, firstErr := handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-risk-approve",
		UserID:         "user-risk",
		Text:           "/skill run bid.collect --version v1.0.0 --risk high --agent bid-all",
	}, agentRuntime{agentID: "bid-all", skillControl: runtime.Runtime})
	if firstErr == nil {
		t.Fatalf("expected confirmation required")
	}
	confirmationID := extractConfirmationID(t, firstErr.Error())
	handled, response, err := handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-risk-approve",
		UserID:         "user-risk",
		Text:           "/skill run bid.collect --version v1.0.0 --risk high --agent bid-all --confirm " + confirmationID,
	}, agentRuntime{agentID: "bid-all", skillControl: runtime.Runtime})
	if err != nil || !handled {
		t.Fatalf("confirm run failed: handled=%v err=%v", handled, err)
	}
	traceID := extractTraceID(t, response)
	handled, replay, err := handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-risk-approve",
		UserID:         "user-risk",
		Text:           "/skill replay " + traceID,
	}, agentRuntime{agentID: "bid-all", skillControl: runtime.Runtime})
	if err != nil || !handled {
		t.Fatalf("replay failed: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(replay, "审计回放") || !strings.Contains(replay, "confirm_required") || !strings.Contains(replay, "success") {
		t.Fatalf("unexpected replay payload: %s", replay)
	}
}

func TestHandleSkillChatCommandMarketplaceInstallFromPackageArchive(t *testing.T) {
	installRoot := t.TempDir()
	t.Setenv("CLAWX_SKILL_INSTALL_ROOT", installRoot)
	archive := createSkillPackageArchive(t, `---
name: bid-market
description: marketplace package
aliases:
  - market
---
run`)
	runtime := newSkillRuntimeForTests(t)
	handled, response, err := handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-market-install",
		UserID:         "user-1",
		Text:           "/skill install bid.market --source clawhub --version v1.2.3 --package " + archive,
	}, agentRuntime{agentID: "bid-all", skillControl: runtime.Runtime})
	if err != nil || !handled {
		t.Fatalf("marketplace install failed: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(response, "安装目录:") {
		t.Fatalf("expected installed dir in response, got: %s", response)
	}
	manifest := filepath.Join(installRoot, "marketplace", "bid.market", "v1.2.3", "SKILL.md")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("installed manifest not found: %v", err)
	}
}

func TestInstallServiceRollbackOnRegistryFailure(t *testing.T) {
	installRoot := t.TempDir()
	t.Setenv("CLAWX_SKILL_INSTALL_ROOT", installRoot)
	archive := createSkillPackageArchive(t, `---
name: rollback-case
description: rollback package
---
rollback`)
	registry := skillorchestrator.NewRegistryService(failingRegistryRepository{})
	installer := skillorchestrator.NewInstallService(registry, nil, skillorchestrator.NewMapAgentStateProvider(nil))
	_, err := installer.Install(context.Background(), skilldomain.SkillMetadata{
		SkillID:     "rollback.case",
		Version:     "v1.0.0",
		Source:      skilldomain.RegistrySourceClawHub,
		InputSchema: map[string]any{"package_uri": archive},
		Enabled:     true,
	}, "")
	if err == nil {
		t.Fatalf("expected registry failure")
	}
	installDir := filepath.Join(installRoot, "marketplace", "rollback.case", "v1.0.0")
	if _, statErr := os.Stat(installDir); !os.IsNotExist(statErr) {
		t.Fatalf("install dir should be rolled back, stat err=%v", statErr)
	}
}

func TestHandleSkillChatCommandBuiltinGlobalAndAgentOverrideResolution(t *testing.T) {
	runtime := newSkillRuntimeForTests(t)
	_, _, err := handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-builtin-override",
		UserID:         "user-1",
		Text:           "/skill install web.fetch --source builtin --version v1.0.0",
	}, agentRuntime{agentID: "agent-b", skillControl: runtime.Runtime})
	if err != nil {
		t.Fatalf("install builtin: %v", err)
	}
	_, _, err = handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-builtin-override",
		UserID:         "user-1",
		Text:           "/skill bind web.fetch --scope global --version v1.0.0",
	}, agentRuntime{agentID: "agent-b", skillControl: runtime.Runtime})
	if err != nil {
		t.Fatalf("bind global: %v", err)
	}
	_, _, err = handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-builtin-override",
		UserID:         "user-1",
		Text:           "/skill bind web.fetch --scope agent-local --agent agent-a --version v2.0.0",
	}, agentRuntime{agentID: "agent-a", skillControl: runtime.Runtime})
	if err != nil {
		t.Fatalf("bind agent-local override: %v", err)
	}
	resolvedA, ok := runtime.Binder.Resolve(context.Background(), skillorchestrator.ResolveRequest{
		SkillID: "web.fetch", AgentID: "agent-a",
	})
	if !ok || resolvedA.Scope != skilldomain.ScopeAgentLocal || resolvedA.Version != "v2.0.0" {
		t.Fatalf("agent-a should resolve agent-local override, got ok=%v binding=%+v", ok, resolvedA)
	}
	resolvedB, ok := runtime.Binder.Resolve(context.Background(), skillorchestrator.ResolveRequest{
		SkillID: "web.fetch", AgentID: "agent-b",
	})
	if !ok || resolvedB.Scope != skilldomain.ScopeGlobal || resolvedB.Version != "v1.0.0" {
		t.Fatalf("agent-b should resolve global builtin binding, got ok=%v binding=%+v", ok, resolvedB)
	}
}

func prepareRunnableSkill(t *testing.T, runtime *skillRuntimeTestWrapper) {
	t.Helper()
	_, _, err := handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-prepare",
		UserID:         "user-risk",
		Text:           "/skill install bid.collect --source builtin --version v1.0.0 --agent bid-all",
	}, agentRuntime{agentID: "bid-all", skillControl: runtime.Runtime})
	if err != nil {
		t.Fatalf("install for runnable skill: %v", err)
	}
	_, _, err = handleSkillChatCommand(chatiface.Message{
		ConversationID: "conv-prepare",
		UserID:         "user-risk",
		Text:           "/skill bind bid.collect --scope agent-local --agent bid-all --version v1.0.0",
	}, agentRuntime{agentID: "bid-all", skillControl: runtime.Runtime})
	if err != nil {
		t.Fatalf("bind for runnable skill: %v", err)
	}
}

func extractConfirmationID(t *testing.T, text string) string {
	t.Helper()
	re := regexp.MustCompile(`--confirm\s+([A-Za-z0-9-]+)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) != 2 {
		t.Fatalf("confirmation id not found in text: %s", text)
	}
	return matches[1]
}

func extractTraceID(t *testing.T, text string) string {
	t.Helper()
	re := regexp.MustCompile(`trace=([A-Za-z0-9-]+)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) != 2 {
		t.Fatalf("trace id not found in text: %s", text)
	}
	return matches[1]
}

type skillRuntimeTestWrapper struct {
	*skillorchestrator.Runtime
	TogglePolicyStore *persistence.SkillPolicyFileStore
}

func newSkillRuntimeForTests(t *testing.T) *skillRuntimeTestWrapper {
	t.Helper()
	dir := t.TempDir()
	registryStore, err := persistence.NewSkillRegistryFileStore(filepath.Join(dir, "registry.json"))
	if err != nil {
		t.Fatalf("new registry store: %v", err)
	}
	policyStore, err := persistence.NewSkillPolicyFileStore(filepath.Join(dir, "policy.json"))
	if err != nil {
		t.Fatalf("new policy store: %v", err)
	}
	if err := policyStore.Save(context.Background(), skilldomain.SkillPolicy{
		AllowedSources:  []skilldomain.RegistrySource{skilldomain.RegistrySourceBuiltin, skilldomain.RegistrySourceClawHub},
		VersionStrategy: skilldomain.VersionStrategyLatest,
	}); err != nil {
		t.Fatalf("seed policy: %v", err)
	}
	bindingStore, err := persistence.NewSkillBindingFileStore(filepath.Join(dir, "bindings.json"))
	if err != nil {
		t.Fatalf("new binding store: %v", err)
	}
	runtime := skillorchestrator.NewRuntime(policyStore, bindingStore, registryStore, skillorchestrator.NewMapAgentStateProvider(nil))
	return &skillRuntimeTestWrapper{Runtime: runtime, TogglePolicyStore: policyStore}
}

func newSkillRuntimeWithAgentProviderForTests(t *testing.T) *skillRuntimeTestWrapper {
	t.Helper()
	base := newSkillRuntimeForTests(t)
	provider := skillorchestrator.NewMapAgentStateProvider(map[string]skillorchestrator.AgentState{
		"bid-all": {
			AgentID:   "bid-all",
			Workspace: "/tmp/bid-all",
			ProfileID: "codex",
			IsDefault: true,
		},
	})
	base.Runtime.Installer = skillorchestrator.NewInstallService(base.Runtime.Registry, base.Runtime.PolicyEngine, provider)
	return base
}

func mustNewPolicyStoreForTest(t *testing.T) *persistence.SkillPolicyFileStore {
	t.Helper()
	store, err := persistence.NewSkillPolicyFileStore(filepath.Join(t.TempDir(), "policy.json"))
	if err != nil {
		t.Fatalf("new policy store: %v", err)
	}
	return store
}

type failingRegistryRepository struct{}

func (failingRegistryRepository) Upsert(context.Context, skilldomain.SkillMetadata) error {
	return fmt.Errorf("forced upsert failure")
}

func (failingRegistryRepository) GetByID(context.Context, string) (skilldomain.SkillMetadata, error) {
	return skilldomain.SkillMetadata{}, skilldomain.ErrSkillMetadataNotFound
}

func (failingRegistryRepository) List(context.Context) ([]skilldomain.SkillMetadata, error) {
	return nil, nil
}

func (failingRegistryRepository) Delete(context.Context, string) error {
	return nil
}

func createSkillPackageArchive(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "skill-package.tgz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	defer file.Close()

	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	payload := []byte(body)
	header := &tar.Header{
		Name: "SKILL.md",
		Mode: 0o644,
		Size: int64(len(payload)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("write archive header: %v", err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatalf("write archive payload: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return path
}
