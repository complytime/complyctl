## Context

`complyctl doctor` runs pre-flight diagnostic checks. The existing `CheckComplypacks` verifies evaluator-id presence in config but not that cache files actually exist on disk.

## Goals

- Detect when `state.json` claims a complypack is cached but the files are missing
- Provide actionable remediation advice
- Non-blocking check (warning, not failure)

## Non-Goals

- Verifying content integrity (checksums/signatures)
- Adding a `--force` flag to `complyctl get` (evaluated and rejected — root-cause fix in SyncComplypack is preferred)

## Decisions

### D1: Fix the root cause in SyncComplypack

The original design comment in `complypack_sync.go` acknowledged that sync only checked state digest, not whether the cache directory existed. The policy sync (`sync.go:85`) already gates the skip with `PolicyStoreExists()`. We added the equivalent `cacheExistsForState()` check to `SyncComplypack`, using the evaluator-id recorded in state from a previous unpack. This makes `complyctl get` self-heal when cache is deleted — no `--force` flag needed.

A `--force` flag was evaluated and rejected because: (a) it bypasses digest comparison, which is a security-relevant integrity check; (b) it treats the symptom rather than the bug; (c) users should not need to know when to apply it.

### D2: No new doctor check needed

The existing `CheckComplypacks` already detects missing complypack caches via
`LookupByEvaluatorID` and emits a non-blocking warning. A separate
`CheckComplypackCacheIntegrity` was implemented and then removed during review
because it produced duplicate warnings for the same condition. With the
SyncComplypack root-cause fix, `complyctl get` self-heals — the existing doctor
check is sufficient for user visibility.

## Risks / Trade-offs

- The check requires loading `state.json` which doctor already does for other checks — no additional I/O cost.
- If #646 (version eviction) lands first, there's guaranteed to be at most one version dir, simplifying the "find content.tar.gz" logic.
