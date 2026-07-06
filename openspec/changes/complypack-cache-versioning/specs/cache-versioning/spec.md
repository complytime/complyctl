## ADDED Requirements

### FR-001: Retention-count-aware eviction

`Store()` MUST retain up to N versions per evaluator-id,
where N is the configured retention count. Versions
exceeding N MUST be evicted in order: orphaned directories
first (not tracked in state), then oldest by
`LastUpdated` timestamp. When state is nil or absent,
eviction MUST fall back to current behavior (remove all
except the target version).

**Given** an evaluator-id with 3 cached versions
(v1.0.0 synced first, v2.0.0 synced second, v3.0.0
synced third) and retention count N=2
**When** `Store()` writes v4.0.0
**Then** v4.0.0 and v3.0.0 remain on disk
**And** v1.0.0 and v2.0.0 are removed

### FR-002: Default retention preserves current behavior

The default retention count MUST be 1, matching the
current single-version-per-evaluator invariant.

**Given** `COMPLYTIME_CACHE_VERSIONS` is not set
**When** `Store()` writes v2.0.0 for an evaluator-id
that has v1.0.0 cached
**Then** only v2.0.0 remains on disk
**And** v1.0.0 is removed

### FR-003: Environment variable configuration

The retention count MUST be configurable via the
`COMPLYTIME_CACHE_VERSIONS` environment variable. Values
less than 1 MUST be treated as 1. Non-integer values
MUST be ignored with a warning logged to stderr, falling
back to the default.

**Given** `COMPLYTIME_CACHE_VERSIONS=3`
**When** `Store()` writes versions for the same
evaluator-id
**Then** up to 3 versions are retained per evaluator-id

### FR-004: Local cache hit avoids re-download

`SyncComplypack` MUST check whether the target version
exists in the local cache before fetching from the
registry. The local cache check is only reachable when
state has an existing entry for the repository (previous
sync established the evaluator-id). First-time syncs
always fetch. If `Lookup(evaluatorID, version)` returns
a non-error result, state MUST be updated to reflect the
version switch and `SyncComplypack` MUST return
`(true, nil)` to trigger downstream generation
invalidation. No OCI content fetch MUST occur.

**Given** evaluator-id `io.complytime.opa` has v1.0.0
and v2.0.0 cached locally
**And** state.json records v2.0.0 as active
**And** `complytime.yaml` is changed to reference the
tag for v1.0.0
**When** `complyctl get` runs
**Then** state.json is updated to record v1.0.0 as
active
**And** no OCI content fetch occurs
**And** `SyncComplypack` returns `(true, nil)`
**And** generation state is invalidated for the
affected evaluator-id

### FR-005: State-driven lookup

`LookupByEvaluatorID` MUST resolve the active version
from state.json rather than scanning the filesystem.
When state is unavailable or the lookup fails, it MUST
fall back to the current directory-scan behavior for
backward compatibility.

**Given** evaluator-id `io.complytime.opa` has v1.0.0
and v2.0.0 on disk
**And** state.json records v2.0.0 as the active version
**When** `LookupByEvaluatorID("io.complytime.opa")` is
called with state
**Then** it returns the content path for v2.0.0

### FR-006: Orphaned version detection in doctor

`CheckComplypacks` MUST report complypack version
directories that exist on disk but are not tracked in
state.json as orphaned. Orphaned versions MUST produce
a non-blocking warning. When state.json does not exist
or is empty, version directories SHOULD be reported as
"untracked" (not "orphaned") with a message suggesting
`complyctl get` to rebuild state.

**Given** evaluator-id directory contains v1.0.0/ and
v2.0.0/
**And** state.json only tracks v2.0.0
**When** `complyctl doctor` runs
**Then** a warning is emitted identifying v1.0.0 as
orphaned

### FR-007: Cache size reporting in doctor

`CheckComplypacks` MUST report the total disk usage of
the complypack cache directory
(`~/.complytime/complypacks/`). The size MUST be
reported in human-readable format (e.g., "12.3 MB").

**Given** the complypack cache contains artifacts
totaling 15 MB
**When** `complyctl doctor` runs
**Then** the output includes cache size information

### FR-008: Cross-evaluator isolation

Eviction MUST be scoped to a single evaluator-id.
Versions cached for other evaluator-ids MUST NOT be
affected.

**Given** `io.complytime.opa` has v1.0.0 and v2.0.0
**And** `io.complytime.ampel` has v1.0.0
**And** retention count N=1
**When** `Store()` writes v3.0.0 for `io.complytime.opa`
**Then** `io.complytime.ampel/v1.0.0` remains untouched

### FR-009: Generation invalidation on version switch

When a local cache hit changes the active complypack
version (state is updated), `SyncComplypack` MUST
return `(true, nil)` so that callers trigger generation
state invalidation for the affected evaluator-id. This
ensures the scan pipeline regenerates artifacts against
the correct complypack content, consistent with the
`fix-complypack-cache-invalidation` contract.

**Given** evaluator-id `io.complytime.opa` has v1.0.0
and v2.0.0 cached locally
**And** state.json records v2.0.0 as active with
generation state referencing v2.0.0 artifacts
**And** user switches `complytime.yaml` to v1.0.0
**When** `complyctl get` runs (local cache hit)
**Then** `SyncComplypack` returns `(true, nil)`
**And** `get.go` invalidates generation state for
`io.complytime.opa`
**And** the next `complyctl scan` regenerates artifacts
using v1.0.0 complypack content

### FR-010: Re-verification on local cache hit

When a `VerifyFunc` is configured and a local cache hit
occurs, `SyncComplypack` MUST re-verify the artifact's
signature via the registry API before accepting the
cache hit. If re-verification fails, the cache hit MUST
be rejected and the standard fetch path MUST be taken.

**Given** a verifier is configured for keyless
verification
**And** evaluator-id `io.complytime.opa` has v1.0.0
cached locally (originally fetched without verification)
**And** user switches `complytime.yaml` to v1.0.0
**When** `complyctl get` runs
**Then** the artifact's signature is verified via the
registry API before accepting the cache hit
**And** if verification fails, a full OCI fetch occurs

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
