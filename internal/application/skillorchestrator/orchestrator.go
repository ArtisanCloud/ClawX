package skillorchestrator

import (
	"context"
	"time"

	skilldomain "clawx/internal/domain/skill"
)

type Runtime struct {
	PolicyEngine *PolicyEngine
	Executor     *Executor
	Router       *Router
	Registry     *RegistryService
	Installer    *InstallService
	Upgrader     *UpgradeService
	Toggle       *ToggleService
	Binder       *BindingService
	Effective    *EffectiveViewService
	RiskGuard    *RiskGuard
	Audit        *AuditService
	Replay       *AuditReplayService
}

func NewRuntime(
	policyRepo skilldomain.PolicyRepository,
	bindingRepo skilldomain.BindingRepository,
	registryRepo skilldomain.RegistryRepository,
	agentStateProvider AgentStateProvider,
) *Runtime {
	engine := NewPolicyEngine(policyRepo)
	registry := NewRegistryService(registryRepo)
	installer := NewInstallService(registry, engine, agentStateProvider)
	upgrader := NewUpgradeService(registry, engine)
	toggle := NewToggleService(policyRepo)
	binder := NewBindingService(bindingRepo, registry)
	effective := NewEffectiveViewService(bindingRepo, registry)
	audit := NewAuditService()
	riskGuard := NewRiskGuard(10 * time.Minute)
	replay := NewAuditReplayService(audit)
	return &Runtime{
		PolicyEngine: engine,
		Executor:     NewExecutor(bindingRepo, engine, riskGuard, audit),
		Router:       NewRouterFromEnv(),
		Registry:     registry,
		Installer:    installer,
		Upgrader:     upgrader,
		Toggle:       toggle,
		Binder:       binder,
		Effective:    effective,
		RiskGuard:    riskGuard,
		Audit:        audit,
		Replay:       replay,
	}
}

func (r *Runtime) Preflight(ctx context.Context, metadata skilldomain.SkillMetadata) error {
	if r == nil || r.PolicyEngine == nil {
		return nil
	}
	return r.PolicyEngine.Evaluate(ctx, metadata)
}
