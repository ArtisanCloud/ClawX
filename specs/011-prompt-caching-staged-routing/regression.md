# 011 Regression Record

- date: 2026-03-22
- command: `go test ./... -count=1`
- result: PASS

## Scope Summary
- `clawx/internal/infrastructure/logging`: PASS
- `clawx/tests/integration`: PASS
- `clawx/tests/contract`: PASS
- `clawx/tests/unit`: PASS

## Notes
- Includes Phase 4 additions:
  - prompt cache metrics aggregator and report renderer
  - staged routing performance baseline gate
  - operations documentation updates for `cached_tokens` observability and thresholds

## Additional Record

- date: 2026-03-22
- command: `go test ./... -count=1`
- result: PASS
- scope: includes Phase 6 trace CLI (`clawx trace cache-report`) and related tests/docs.
