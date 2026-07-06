## Why

`complypackDigestsByEvaluator()` in `scan.go` builds a `map[string]string` (evaluator-id → digest) by iterating `state.Complypacks`. If two complypack entries in `complytime.yaml` resolve to the same evaluator-id (different repository URLs, same evaluator-id in their config), the map holds only one digest — last-writer-wins from Go map iteration, which is non-deterministic.

This causes generation state to record an arbitrary digest. On subsequent scans, `IsFresh()` may trigger unnecessary regeneration or skip when it shouldn't.

This is tracked in [#647](https://github.com/complytime/complyctl/issues/647).

## What Changes

- Add config validation in `complyctl get` (after all complypacks are fetched) that detects when multiple complypack entries resolve to the same evaluator-id.
- When detected, emit a clear error identifying the conflicting entries and refuse to proceed with sync.
- Alternatively: detect at `complyctl scan` time and error early.

## Capabilities

### New Capabilities
- `config-evaluator-id-validation`: Detects and rejects complypack configurations where multiple entries resolve to the same evaluator-id.

### Modified Capabilities
- `get`: Validates evaluator-id uniqueness after complypack sync completes.

### Removed Capabilities
- None.

## Impact

- `cmd/complyctl/cli/get.go` or `internal/complytime/config.go` — validation logic
- Config validation tests
- No breaking changes to complytime.yaml schema (just stricter validation)

## Constitution Alignment

### I. Autonomous Collaboration

**Assessment**: PASS

Clear error message identifies the conflicting entries with their repository URLs and shared evaluator-id.

### II. Composability First

**Assessment**: PASS

Validation is additive. Existing valid configs are unaffected.

### III. Observable Quality

**Assessment**: PASS

Fail-fast with actionable error rather than silent non-deterministic behavior.

### IV. Testability

**Assessment**: PASS

Testable with mock state containing duplicate evaluator-ids.
