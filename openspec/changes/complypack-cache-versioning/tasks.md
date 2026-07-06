<!--
  [P] marks tasks eligible for parallel execution.
  Add [P] when a task: (a) touches different files from
  other [P] tasks in the group, (b) has no dependency
  on prior tasks in the group, (c) can safely execute
  without ordering constraints.
  Do NOT add [P] when tasks modify the same file —
  parallel workers will cause merge conflicts.
  Tasks without [P] run sequentially first, then [P]
  tasks run in parallel.
-->

## 1. Configuration and constants

- [ ] 1.1 [P] Add `CacheVersionsEnvVar` constant
  (`"COMPLYTIME_CACHE_VERSIONS"`) to
  `internal/complytime/consts.go`
- [ ] 1.2 [P] Add `CacheRetentionCount()` function to
  `internal/cache/` that reads
  `os.Getenv(complytime.CacheVersionsEnvVar)`, parses
  as int, clamps to minimum 1, warns on invalid values
  (including empty string), defaults to 1

## 2. State injection and retention-aware eviction

- [ ] 2.1 Modify `NewComplypackCache()` in
  `internal/cache/complypack.go` to accept an optional
  `*State` parameter and store it on the struct
- [ ] 2.2 Modify `evictOldVersions()` to accept a
  retention count and use the struct's `*State` for
  timestamp ordering. Evict orphaned directories first,
  then oldest by `LastUpdated`, keeping up to N versions.
  When state is nil, fall back to current behavior
  (remove all except target version)
- [ ] 2.3 Update `Store()` to pass retention count
  (from `CacheRetentionCount()`) to `evictOldVersions()`
- [ ] 2.4 Update `NewComplypackCache` call sites to pass
  state where available (nil where not):
  `cmd/complyctl/cli/scan.go`,
  `cmd/complyctl/cli/get.go`,
  `cmd/complyctl/cli/providers.go`,
  `internal/doctor/doctor.go`
- [ ] 2.5 Update test file call sites to pass nil state:
  `internal/cache/complypack_test.go`,
  `internal/cache/complypack_pipeline_test.go`,
  `internal/cache/complypack_sync_test.go`,
  `cmd/complyctl/cli/cli_test.go`

## 3. State-driven lookup

- [ ] 3.1 Add `EvaluatorIDToVersion(evaluatorID)` helper
  on `*State` that iterates `Complypacks` entries to find
  the active version for a given evaluator-id (reverse
  lookup). Returns `("", false)` when evaluator-id is
  not found or Complypacks map is empty
- [ ] 3.2 Modify `LookupByEvaluatorID()` to use the
  struct's `*State` when non-nil: resolve version via
  `EvaluatorIDToVersion()` and delegate to `Lookup()`.
  Fall back to directory scan when state is nil or
  lookup fails. No signature change.

## 4. Local cache hit in sync

- [ ] 4.1 In `SyncComplypack()` in
  `internal/cache/complypack_sync.go`, after the remote
  digest probe returns a mismatch: resolve evaluator-id
  from existing state entry for the repository. If no
  state entry exists (first-time sync), skip the local
  cache check. If state entry exists, check
  `complypackCache.Lookup(evaluatorID, version)`. If
  cache hit: re-verify via registry API when verifier
  is configured (reject cache hit on verification
  failure), update state, and return `(true, nil)` to
  trigger generation invalidation

## 5. Doctor cache health reporting

- [ ] 5.1 [P] Add `walkCacheSize()` helper in
  `internal/doctor/doctor.go` that sums file sizes
  under the complypacks cache directory
- [ ] 5.2 [P] Add `findOrphanedVersions()` helper that
  compares on-disk version directories against state.json
  complypack entries. When state.json is absent or empty,
  report versions as "untracked" instead of "orphaned"
- [ ] 5.3 Extend `CheckComplypacks()` to call both
  helpers and emit cache size info and orphan warnings.
  Load state internally via `LoadState()` (no signature
  change to `CheckComplypacks`)

## 6. Unit tests

- [ ] 6.1 [P] Test `CacheRetentionCount()`: default 1,
  env var override, empty string fallback, invalid value
  fallback with warning, value < 1 clamped, integer
  overflow fallback
- [ ] 6.2 [P] Test `evictOldVersions()` with N=2:
  pre-seed state with 3 versions having explicit
  `LastUpdated` timestamps (v1 oldest, v3 newest). Store
  v4.0.0. Assert v4.0.0 and v3.0.0 remain, v1.0.0 and
  v2.0.0 are removed
- [ ] 6.3 [P] Test `evictOldVersions()` with N=1:
  preserves current behavior (single version)
- [ ] 6.4 [P] Test `evictOldVersions()` orphaned
  directories evicted before tracked versions: given N=2,
  1 tracked version (v2.0.0 in state) and 2 orphaned
  directories (v0.1.0, v0.2.0 not in state), after
  Store(v3.0.0): v3.0.0 and v2.0.0 remain, v0.1.0 and
  v0.2.0 are removed
- [ ] 6.5 [P] Test `evictOldVersions()` with nil state:
  falls back to current behavior (remove all except
  target version)
- [ ] 6.6 [P] Test `LookupByEvaluatorID()` with state
  injected via `NewComplypackCache`: resolves active
  version from state
- [ ] 6.7 [P] Test `LookupByEvaluatorID()` with nil
  state: falls back to directory scan (current behavior)
- [ ] 6.8 [P] Test `SyncComplypack()` local cache hit:
  assert `(true, nil)` return AND state.json updated to
  reflect switched version AND no OCI content fetch
- [ ] 6.9 [P] Test `SyncComplypack()` local cache miss
  when version directory exists but content files are
  missing or corrupted: assert sync falls through to
  remote fetch
- [ ] 6.10 [P] Test `SyncComplypack()` local cache hit
  with verifier configured: assert re-verification
  occurs before accepting cache hit
- [ ] 6.11 [P] Test `EvaluatorIDToVersion()`: reverse
  lookup returns correct version; returns empty when
  evaluator-id not found; handles empty Complypacks map
- [ ] 6.12 [P] Test cross-evaluator isolation: eviction
  scoped to target evaluator-id
- [ ] 6.13 [P] Test doctor orphan detection: seed
  state.json with v2.0.0 only, create v1.0.0 and v2.0.0
  directories, assert warning identifies v1.0.0 as
  orphaned
- [ ] 6.14 [P] Test doctor cache size reporting: seed
  cache with known-size files, assert output includes
  human-readable size
- [ ] 6.15 [P] Test doctor no orphans: all on-disk
  versions tracked in state, assert no orphan warnings
- [ ] 6.16 [P] Test doctor untracked vs orphaned: seed
  version directories on disk without state.json, assert
  output uses "untracked" terminology (not "orphaned")
  and includes `complyctl get` suggestion (FR-006)

## 7. Verification

- [ ] 7.1 Run `go test -race ./internal/cache/...` and
  confirm all tests pass
- [ ] 7.2 Run `go test -race ./internal/doctor/...` and
  confirm all tests pass
- [ ] 7.3 Run `go vet ./...`
- [ ] 7.4 Run `make lint`
- [ ] 7.5 E2E: `complyctl get` with version switch and
  `COMPLYTIME_CACHE_VERSIONS=2` — verify both versions
  remain on disk and no re-download on switch-back

## 8. Documentation

- [ ] 8.1 [P] Update `CHANGELOG.md` with cache versioning
  entry under Added: `COMPLYTIME_CACHE_VERSIONS` env var
  and doctor cache health reporting
- [ ] 8.2 [P] Update `AGENTS.md` Recent Changes section
  with complypack-cache-versioning summary
- [ ] 8.3 [P] File `unbound-force/website` issue for
  cache versioning documentation
  (`COMPLYTIME_CACHE_VERSIONS` env var, doctor cache
  health reporting, `complyctl get` local cache hit)
<!-- spec-review: passed -->
