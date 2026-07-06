## Why

The complypack cache (`~/.complytime/complypacks/{evaluator-id}/
{version}/`) enforces a single-version-per-evaluator invariant
via `evictOldVersions()` in `Store()`. This was the correct fix
for non-deterministic lookups ([#645]) and unbounded disk growth
([#646]), but it forces re-downloads when switching between
previously cached versions.

When a user changes the complypack tag in `complytime.yaml`
(e.g., `:v2.0.0` to `:v1.0.0`), `SyncComplypack` compares
the state.json digest against the remote, finds a mismatch,
and fetches from the registry — even if v1.0.0 was cached
moments ago. This affects productization workflows that
compare scan results across version combinations and
rollback scenarios when the registry is unavailable.

Tracked in [#676](https://github.com/complytime/complyctl/issues/676).

[#645]: https://github.com/complytime/complyctl/issues/645
[#646]: https://github.com/complytime/complyctl/issues/646

## What Changes

- `evictOldVersions()` becomes retention-count-aware: it
  keeps the N most recently synced versions per evaluator-id
  instead of evicting all but the current version.
- `SyncComplypack` checks local cache before fetching: if
  the target evaluator-id + version directory exists on disk,
  state is updated and `(true, nil)` is returned to trigger
  downstream generation invalidation, without a network
  round-trip.
- `LookupByEvaluatorID` resolves the active version from
  state.json instead of scanning directories, making it
  deterministic regardless of how many versions are cached.
- `CheckComplypacks` in doctor reports orphaned versions
  and total cache disk usage.

## Capabilities

### New Capabilities
- `complypack-local-cache-hit`: `SyncComplypack` serves
  locally cached versions without re-downloading when the
  target version directory exists on disk.
- `complypack-cache-health`: `complyctl doctor` reports
  orphaned complypack versions and total cache disk usage.

### Modified Capabilities
- `complypack-version-eviction`: Eviction retains up to N
  versions per evaluator-id (configurable via
  `COMPLYTIME_CACHE_VERSIONS`, default 1) instead of
  evicting all but the current version.
- `complypack-lookup`: `LookupByEvaluatorID` resolves
  version from state.json rather than directory scanning.

### Removed Capabilities
- None.

## Impact

- `internal/cache/complypack.go` — `NewComplypackCache()`
  gains `*State` parameter; `evictOldVersions()` becomes
  retention-count-aware using struct state;
  `LookupByEvaluatorID()` resolves from struct state
  (no signature change)
- `internal/cache/complypack_sync.go` — local cache check
  before remote fetch in `SyncComplypack()`; local cache
  hit returns `(true, nil)` to trigger generation
  invalidation
- `internal/cache/state.go` —
  `EvaluatorIDToVersion()` reverse lookup helper
- `internal/complytime/consts.go` — new
  `CacheVersionsEnvVar` constant
- `internal/doctor/doctor.go` — `CheckComplypacks()`
  extended with orphan detection and cache size reporting;
  `NewComplypackCache` call updated with state
- `cmd/complyctl/cli/scan.go` — `NewComplypackCache` call
  updated with state
- `cmd/complyctl/cli/providers.go` — `NewComplypackCache`
  call updated with state
- `cmd/complyctl/cli/get.go` — `NewComplypackCache` call
  updated with state
- Test files: `complypack_test.go`,
  `complypack_pipeline_test.go`, `complypack_sync_test.go`,
  `cli_test.go`, `doctor_test.go` — `NewComplypackCache`
  calls updated to pass `nil` state
- No CLI flag changes, no config schema changes,
  no public API changes (all code is under `internal/`)

## Constitution Alignment

### I. Single Source of Truth

**Assessment**: PASS

`CacheVersionsEnvVar` constant centralized in
`internal/complytime/consts.go`. Retention count read
from a single function `CacheRetentionCount()`.

### II. Simplicity & Isolation

**Assessment**: PASS

Changes scoped to the cache package. Eviction logic
remains in a single function. `LookupByEvaluatorID`
gains state awareness without signature change.

### III. Incremental Improvement

**Assessment**: PASS

Focused on cache versioning only. No unrelated
refactoring. Default 1 preserves current behavior.

### VI. Composability

**Assessment**: PASS

No signature changes to public methods consumed by
scan, doctor, or providers. `LookupByEvaluatorID`
returns the same types.

### VII. Convention Over Configuration

**Assessment**: PASS

Default retention count of 1 preserves current
single-version behavior. Users only configure when
deviating from the default.
