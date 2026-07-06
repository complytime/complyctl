## Context

PR #661 solved complypack cache non-determinism by
enforcing a single-version-per-evaluator invariant:
`Store()` calls `evictOldVersions()` to remove all
sibling version directories before writing the new one.
This made `LookupByEvaluatorID()` deterministic but
prevents retaining previously cached versions.

`SyncComplypack` currently has one skip path: remote
digest matches local state AND cache exists on disk.
There is no path for "remote digest differs but the
target version already exists locally." This forces a
re-download on every version switch.

The state structure (`state.json`) tracks one entry per
complypack repository with version, digest, and
evaluator-id. This is sufficient to identify the active
version without directory scanning.

**Cross-spec dependency**: The `fix-complypack-cache-
invalidation` OpenSpec established that when
`SyncComplypack` returns `fetched == true`, callers
eagerly invalidate workspace generation state for the
affected evaluator-id. Any new code path that changes
the active complypack version MUST return `(true, nil)`
to trigger this invalidation, or stale generation
artifacts will persist.

## Goals / Non-Goals

### Goals
- Retain up to N complypack versions per evaluator-id
  on disk (configurable, default 1)
- Serve locally cached versions without network
  round-trip when the target version exists on disk
- Make `LookupByEvaluatorID` state-driven rather than
  directory-scan-driven
- Surface orphaned versions and cache size in doctor
- Preserve generation invalidation semantics when the
  active complypack version changes via local cache hit

### Non-Goals
- Size-based or time-based cache eviction policies
  (may be added later if count-based proves insufficient)
- New CLI subcommand (`complyctl cache clean` or similar)
- Per-workspace cache isolation (cache remains global)
- Content-level deduplication across versions with
  identical functional content but different manifests

## Decisions

### D1: Keep-N retention model with default 1

**Decision**: `evictOldVersions()` retains the N most
recently synced versions per evaluator-id, where N is
read from `COMPLYTIME_CACHE_VERSIONS` (default 1).

**Rationale**: Count-based retention is simple,
predictable, and covers all four use cases in #676.
Complypacks are OCI artifacts of roughly uniform size,
making count a reasonable proxy for disk usage. Default
1 preserves the current single-version behavior (zero
regression), while power users opt in by setting the
env var.

**Alternatives considered**: Time-based TTL (requires
tracking last-access timestamps, harder to reason
about), size-based cap (complex implementation,
unpredictable which versions survive), hybrid
(unnecessary complexity for initial implementation).

### D2: Environment variable for configuration

**Decision**: The retention count is configured via
`COMPLYTIME_CACHE_VERSIONS` environment variable.

**Rationale**: The complypack cache is global
(`~/.complytime/complypacks/`), shared across all
workspaces. A `complytime.yaml` setting would create
scope ambiguity — which workspace's value wins when
multiple workspaces share the cache. An env var matches
the global scope, requires zero config schema changes,
and follows the existing pattern (`COMPLYTIME_WORKSPACE`,
`COMPLYTIME_SHOW_PASSING`).

**Alternatives considered**: `complytime.yaml` `cache:`
section (scope mismatch with global cache), global
config file `~/.complytime/config.yaml` (new config
concept doesn't exist yet), CLI flag `--cache-versions`
(tedious for repeated use, no persistence).

### D3: Local cache check before remote fetch

**Decision**: In `SyncComplypack`, after the remote
digest probe returns a mismatch, check whether the
target version already exists on disk via
`complypackCache.Lookup(evaluatorID, version)`. The
evaluator-id is resolved from the existing state entry
for the repository. If no state entry exists (first-time
sync), the local cache check is skipped and the standard
fetch path is taken. If the version exists locally,
update state.json and return `(true, nil)` to trigger
downstream generation invalidation.

**Rationale**: This eliminates re-downloads when
switching between previously cached versions. The
`Lookup(evaluatorID, version)` method already exists
and validates both coordinates (file existence + JSON
parse of `config.json`). Returning `(true, nil)` rather
than `(false, nil)` ensures callers that gate generation
invalidation on `fetched == true` (per
`fix-complypack-cache-invalidation`) correctly
invalidate stale artifacts when the active version
changes. The local cache check is only reachable when
a previous sync has established the evaluator-id in
state — first-time syncs always fetch.

**Alternatives considered**: Return `(false, nil)` for
cache hits (breaks generation invalidation contract),
only check after a failed fetch (defeats the purpose),
probe on-disk directories when state has no entry
(complex, unnecessary for the version-switching use
case).

### D4: State injection at construction time

**Decision**: `NewComplypackCache(cacheDir, state)`
accepts an optional `*State`. The struct stores it as
a field. `LookupByEvaluatorID` and `evictOldVersions`
use the struct's state internally — no signature
changes on public methods.

**Rationale**: Both `LookupByEvaluatorID` (for version
resolution) and `Store` (for eviction ordering) need
state access. Injecting at construction avoids changing
two method signatures and rippling through 5+ call
sites. All `NewComplypackCache` callers already have
state loaded or can trivially load it. When `state` is
nil (e.g., in tests), both methods fall back to current
behavior: directory scan for lookup, filesystem-only
eviction for store.

`ComplypackCache` lives in `internal/cache/`, so no
downstream repos can import it — the API change is
strictly internal to complyctl.

**Alternatives considered**: Add `*State` parameter to
`LookupByEvaluatorID` and `Store` individually (ripples
to all call sites, two separate parameter additions),
add a `SetState()` setter (mutable state on struct is
error-prone, temporal coupling).

### D5: Eviction ordering by state timestamp

**Decision**: When `evictOldVersions()` must reduce
the version count below N, it evicts the versions
with the oldest `LastUpdated` timestamp in state.json.
Versions not tracked in state (orphaned directories)
are evicted first. When state is nil or empty,
eviction falls back to filesystem-only behavior
(remove all except the target version, matching
current behavior).

**Rationale**: `LastUpdated` reflects when the version
was last synced, which is the most relevant ordering
for retention. Orphaned directories (present on disk
but absent from state) are least valuable and should
be evicted preferentially. The state only tracks the
active version per repository, but during the eviction
window (before the new active version is recorded),
the struct's state provides the previous active
version's timestamp. Combined with the newly stored
version, this gives sufficient ordering for N=2.
For N>2, previously evicted versions are no longer
on disk, so the effective retention is bounded by
what was kept during prior Store calls.

**Alternatives considered**: Filesystem mtime (less
reliable, depends on OS behavior during atomic
rename), alphabetical version sorting (not meaningful
— `v1.0.0` is not always older than `v2.0.0` in
practice).

### D6: Re-verification on local cache hit

**Decision**: When a local cache hit occurs and a
`VerifyFunc` is configured on the sync, the cached
artifact MUST be re-verified via the registry API
before accepting the cache hit. If re-verification
fails, the cache hit is rejected and the standard
fetch path is taken.

**Rationale**: A version may have been originally
cached before signature verification was configured.
Serving it from cache without re-verification would
bypass the sigstore verification pipeline established
by the `sigstore-verification` feature. Re-verifying
via registry API (not local content) ensures the
artifact's signature is validated against the current
verification config. If the registry is unavailable
for re-verification, the cache hit is still rejected
(conservative — prefer security over convenience).

**Alternatives considered**: Skip verification for
cache hits (security gap), store per-version
verification metadata (adds state complexity beyond
the scope of this change), always re-fetch when
verifier is configured (defeats the cache hit
optimization for verified artifacts).

## Risks / Trade-offs

- **[State dependency in eviction]** `evictOldVersions`
  currently has no state dependency — it purely operates
  on the filesystem. Adding state awareness couples it
  to state.json correctness. Mitigation: orphaned
  directories (not in state) are evicted first, so
  corrupted state at worst causes premature eviction of
  valid versions. When state is nil or absent, eviction
  falls back to filesystem-only behavior (current).
- **[Evaluator-id reverse lookup]** State is keyed by
  repository, not evaluator-id. A reverse lookup
  requires iterating all complypack state entries.
  The number of entries is small (typically 1-3), so
  this is not a performance concern.
- **[Default 1 limits discoverability]** Users must
  know about `COMPLYTIME_CACHE_VERSIONS` to enable
  multi-version caching. Mitigation: doctor reports
  cache health, and documentation covers the env var.
- **[Concurrent Store calls]** Two concurrent `Store()`
  calls for the same evaluator-id with different
  versions could race on eviction. Mitigated by
  sequential sync in `syncAllComplypacks()` — no
  parallelism exists today. A future concurrent sync
  would need file locking.
- **[Cross-spec: generation invalidation]** The local
  cache hit path changes the active complypack version.
  The `fix-complypack-cache-invalidation` spec requires
  generation state invalidation when the active version
  changes. Returning `(true, nil)` from the cache hit
  path reuses the existing invalidation trigger.
- **[Cross-spec: sigstore verification]** The local
  cache hit path must re-verify when a verifier is
  configured, to avoid serving artifacts that were
  cached before verification was enabled. This adds
  a registry API call but preserves security guarantees.
