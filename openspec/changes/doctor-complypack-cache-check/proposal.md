## Why

`ComplypackSync.SyncComplypack()` checked freshness by comparing `state.json` digest against remote but did not verify the cache directory still existed on disk. If the local cache directory was deleted but `state.json` still recorded a matching digest, sync was permanently skipped. `LookupByEvaluatorID` then returned an empty path, the provider got no complypack content, and scan could produce incomplete results without error.

There was no user-facing detection or self-healing for this state. `complyctl doctor` checked whether evaluator-ids were present in complypack config but did not verify file integrity.

This is tracked in [#649](https://github.com/complytime/complyctl/issues/649).

## What Changes

- `SyncComplypack` gains a cache-existence check (mirroring the policy sync pattern) so `complyctl get` self-heals when cache is deleted.
- The existing `CheckComplypacks` in doctor already detects missing caches via `LookupByEvaluatorID` — no new doctor check needed.

## Capabilities

### New Capabilities
- `complypack-cache-self-heal`: `SyncComplypack` detects missing cache directories and re-fetches despite matching digest, mirroring the policy sync `PolicyStoreExists()` pattern.

### Modified Capabilities
- `get`: Complypack sync gains cache-existence check alongside digest comparison for incremental skip decisions.

### Removed Capabilities
- `doctor-complypack-integrity`: Evaluated and removed — the existing `CheckComplypacks` already detects missing caches via `LookupByEvaluatorID`.

## Impact

- `internal/cache/complypack_sync.go` — root-cause fix: `cacheExistsForState()` + skip guard update
- `internal/cache/complypack_sync_test.go` — cache-missing re-fetch test
- No doctor changes (existing `CheckComplypacks` already covers this case)
- No CLI flag changes

## Constitution Alignment

### I. Autonomous Collaboration

**Assessment**: PASS

Doctor proactively detects and reports the inconsistency with actionable remediation advice.

### II. Composability First

**Assessment**: PASS

The check is additive — existing checks are unaffected. The new check is independent and scoped to complypack cache integrity only.

### III. Observable Quality

**Assessment**: PASS

Clear diagnostic output identifying which complypack entries have missing cache directories, with suggested fix commands.

### IV. Testability

**Assessment**: PASS

Testable by pre-seeding `state.json` with complypack entries and verifying the check detects missing/present directories.
