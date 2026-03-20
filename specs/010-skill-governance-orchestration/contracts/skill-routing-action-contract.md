# Skill Routing & Action Contract (010)

## Scope
- LLM-first natural language routing
- Command-to-action mapping
- Low-confidence clarify (no execution)
- Non-skill intent fallback to original task chain
- Route input digest contract (`message + context_digest + skill_catalog_digest`)

## Route Input Contract
- `message`: original user text
- `context_digest`: projected digest hash + compact entries
- `skill_catalog_digest`: deterministic hash from registry + binding snapshot

## Action Contract
- `intent`: required
- `skill_id`: required for executable intents
- `confidence`: `0.0 ~ 1.0`
- `risk_level`: enum `low|medium|high`
- `requires_confirmation`: mandatory for `high`
- `source`: `command|nl`
- `arguments`: map (includes digest hints in NL path)

## Routing Rules
1. Command (`/skill ...`) is parsed first, mapped to canonical `SkillAction`.
2. NL path uses router threshold (default `0.70`).
3. If confidence below threshold:
   - return clarify suggestion
   - do not execute
4. If no skill intent matched:
   - fallback to non-skill chain
5. Command and NL for equivalent intent should converge to equivalent `SkillAction` semantics.

## Validation Coverage
- Contract:
  - `tests/contract/skill_routing_action_contract_test.go`
  - `tests/contract/skill_command_action_mapping_contract_test.go`
  - `tests/contract/skill_low_confidence_contract_test.go`
  - `tests/contract/skill_non_intent_fallback_contract_test.go`
- Integration:
  - `tests/integration/skill_nl_routing_flow_test.go`
  - `tests/integration/skill_dual_entry_flow_test.go`
  - `tests/integration/skill_non_intent_fallback_flow_test.go`
  - `tests/integration/skill_routing_performance_test.go`
