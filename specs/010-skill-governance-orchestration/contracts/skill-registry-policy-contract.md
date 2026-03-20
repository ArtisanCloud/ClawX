# Skill Registry & Policy Contract (010)

## Scope
- Skill metadata registry (`register/list/get`)
- Install / upgrade / disable governance
- Policy enforcement (`source`, `version`, `disabled`, `capability`)

## Inputs
- Command path:
  - `/skill install <skill_id> --source <builtin|clawhub|local|git> --version <vX.Y.Z> [--agent <agent_id>]`
  - `/skill upgrade <skill_id> --version <vX.Y.Z>`
  - `/skill disable <skill_id>`
- NL path:
  - Natural language is routed to `SkillAction` with same governance checks.

## Core Guarantees
1. Metadata must be valid (`skill_id`, `version`, `source`, risk enum).
2. Install must pass policy source allowlist.
3. Install/upgrade must pass version strategy:
   - `latest|compatible`: accept any non-empty version.
   - `pin`: only pinned version per skill.
4. Disabled skills are rejected immediately in policy/execution path.
5. User-facing errors are normalized:
   - policy reject -> `策略拒绝`
   - disabled -> `技能已禁用`
   - missing params -> `参数不完整`
   - not found -> `未找到对应技能或配置项`

## Deterministic Behavior
- Given same policy snapshot and same metadata, policy evaluation output is deterministic.
- Registry list/get reflect latest successful upsert.
- Disable is immediate for subsequent checks in same runtime.

## Validation Coverage
- Contract:
  - `tests/contract/skill_registry_metadata_contract_test.go`
  - `tests/contract/skill_policy_source_contract_test.go`
  - `tests/contract/skill_policy_version_contract_test.go`
  - `tests/contract/skill_disable_contract_test.go`
- Integration:
  - `tests/integration/skill_registry_policy_flow_test.go`
