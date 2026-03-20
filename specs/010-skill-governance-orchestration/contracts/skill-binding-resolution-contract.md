# Skill Binding Resolution Contract (010)

## Scope
- Three-level binding model:
  - `global`
  - `project`
  - `agent-local`
- Deterministic resolution and fallback
- Effective skill view for `agent/project`

## Priority Rule
- Resolution priority is fixed:
  - `agent-local > project > global`

## Resolution Contract
1. If same skill exists in all scopes and agent-local matches current agent, pick agent-local.
2. If agent-local missing and project matches current project, pick project.
3. If both above missing, fallback to global.
4. If upper-level binding removed, next lower level becomes effective automatically.
5. Execution requires resolved binding; unresolved scope must reject.

## Effective View Contract
- Query by `agent/project` returns deterministic effective set.
- Each effective item includes:
  - `skill_id`
  - `version`
  - resolved `scope`
  - optional source/enable from registry
  - human-readable resolution reason

## Validation Coverage
- Contract:
  - `tests/contract/skill_binding_resolution_contract_test.go`
  - `tests/contract/skill_binding_fallback_contract_test.go`
  - `tests/contract/skill_effective_view_contract_test.go`
- Integration:
  - `tests/integration/skill_binding_scope_flow_test.go`
