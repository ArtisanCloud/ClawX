#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

SPEC_PARITY="specs/004-channels/openclaw-channel-parity.md"
SPEC_SPEC="specs/004-channels/spec.md"
SPEC_PLAN="specs/004-channels/plan.md"
SPEC_TASKS="specs/004-channels/tasks.md"
GUIDE_GOV="docs/guides/phase_4/phase_4_spec_governance_checklist.md"
GUIDE_VALIDATION="docs/guides/phase_4/phase_4_channel_parity_validation.md"

required_files=(
  "$SPEC_PARITY"
  "$SPEC_SPEC"
  "$SPEC_PLAN"
  "$SPEC_TASKS"
  "$GUIDE_GOV"
  "$GUIDE_VALIDATION"
)

for file in "${required_files[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "[FAIL] missing required file: $file"
    exit 1
  fi
done

range="${SPEC_GOVERNANCE_DIFF_RANGE:-}"
changed_files=""
range_applied="false"

if [[ -n "$range" ]]; then
  base_ref="${range%%..*}"
  head_ref="${range##*..}"
  if git rev-parse --verify "$base_ref" >/dev/null 2>&1 && git rev-parse --verify "$head_ref" >/dev/null 2>&1; then
    changed_files="$(git diff --name-only "$range" || true)"
    range_applied="true"
  fi
fi

if [[ "$range_applied" != "true" ]]; then
  changed_files="$( { git diff --name-only; git diff --cached --name-only; } | sort -u || true )"
fi

if [[ -n "$changed_files" ]]; then
  mapfile -t changed_spec_files < <(printf '%s\n' "$changed_files" | awk 'NF>0' | grep -E "^specs/004-channels/(openclaw-channel-parity\.md|spec\.md|plan\.md|tasks\.md)$" || true)

  if [[ "${#changed_spec_files[@]}" -gt 0 ]]; then
    declare -A changed_map=()
    for item in "${changed_spec_files[@]}"; do
      changed_map["$item"]=1
    done

    missing=()
    for item in "$SPEC_PARITY" "$SPEC_SPEC" "$SPEC_PLAN" "$SPEC_TASKS"; do
      if [[ -z "${changed_map[$item]:-}" ]]; then
        missing+=("$item")
      fi
    done

    if [[ "${#missing[@]}" -gt 0 ]]; then
      echo "[FAIL] FR-026 gate failed: spec chain changed but not fully synchronized."
      echo "Changed files:"
      printf '  - %s\n' "${changed_spec_files[@]}"
      echo "Missing files:"
      printf '  - %s\n' "${missing[@]}"
      exit 1
    fi
  fi
fi

if ! rg -q "FR-021" "$GUIDE_GOV"; then
  echo "[FAIL] governance checklist missing FR-021 section"
  exit 1
fi
if ! rg -q "FR-022" "$GUIDE_GOV"; then
  echo "[FAIL] governance checklist missing FR-022 section"
  exit 1
fi
if ! rg -q "FR-026" "$GUIDE_GOV"; then
  echo "[FAIL] governance checklist missing FR-026 section"
  exit 1
fi

if ! rg -q "Wave 2" "$GUIDE_VALIDATION"; then
  echo "[FAIL] parity validation doc missing Wave 2 section"
  exit 1
fi
if ! rg -q "Wave 3" "$GUIDE_VALIDATION"; then
  echo "[FAIL] parity validation doc missing Wave 3 section"
  exit 1
fi
if ! rg -q "Wave 4" "$GUIDE_VALIDATION"; then
  echo "[FAIL] parity validation doc missing Wave 4 section"
  exit 1
fi

echo "[PASS] spec governance checks passed"
