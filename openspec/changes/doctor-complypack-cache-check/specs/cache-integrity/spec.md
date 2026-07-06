## ADDED Requirements

### FR-001: Self-heal when cache directory is missing

**Given** `state.json` records a complypack entry with evaluator-id `E` and digest `D`
**And** the directory `~/.complytime/complypacks/E/` does not exist
**When** `complyctl get` runs and the remote digest matches `D`
**Then** `SyncComplypack` MUST re-fetch the artifact instead of skipping
**And** the cache MUST be restored after sync completes

### FR-002: Skip only when digest matches AND cache exists

**Given** `state.json` records a complypack entry with matching remote digest
**And** the cache directory and `content.tar.gz` exist on disk
**When** `complyctl get` runs
**Then** `SyncComplypack` MUST skip the fetch (incremental sync)

### FR-003: Doctor warns when cache is missing

**Given** `state.json` records a complypack entry with evaluator-id `E`
**And** `LookupByEvaluatorID(E)` returns an empty path
**When** `complyctl doctor` runs
**Then** the existing `CheckComplypacks` MUST emit a warning for evaluator `E`
**And** the warning MUST suggest running `complyctl get`

Note: A separate `CheckComplypackCacheIntegrity` was evaluated and removed
because `CheckComplypacks` already covers this case. Adding a second check
produced duplicate warnings for the same condition.

### FR-004: Handle empty state (no complypacks configured)

**Given** no complypack entries exist in the workspace config
**When** `complyctl get` or `complyctl doctor` runs
**Then** complypack checks MUST be skipped without error
