## Why

Digest values stored in `state.json` (via `PolicyState.Digest`) are passed
through without format validation. While digests originate from OCI registry
responses and are validated upstream by `oras-go`, there is no defense-in-depth
check at the ingestion layer. If a malformed digest reaches `state.json`
(manual edit, file corruption, future bug), it propagates silently through
generation state freshness detection, OCI reference construction, and display.

Raised by @trevor-vaughan in
[#661](https://github.com/complytime/complyctl/pull/661#discussion_r3506967281)
citing NIST 800-53 SI-10 (Input Validation): cryptographic hashes should be
validated at every handoff point. Tracked in
[#677](https://github.com/complytime/complyctl/issues/677).

## What Changes

- Add `ValidateDigest(s string) error` function in `internal/cache/state.go`
  using `digest.Parse()` from the already-vendored `opencontainers/go-digest`.
- `UpdatePolicyStateWithVerification` and `UpdateComplypackStateWithVerification`
  gain an `error` return, rejecting malformed digests at the write path.
- `LoadState` gains post-unmarshal validation that warns and excludes entries
  with malformed digests, preserving backward compatibility with corrupted
  state files. Empty digest fields are preserved (not treated as malformed).
- Test fixtures across `internal/cache/*_test.go`,
  `cmd/complyctl/cli/cli_test.go`, `internal/doctor/doctor_test.go`,
  `internal/policy/generation_state_test.go`, and other files updated to
  use valid-format digests.

## Acceptance Criteria

- FR-001: Given a valid OCI digest (e.g., `sha256:<64 hex chars>`),
  When `UpdatePolicyStateWithVerification` is called,
  Then no error is returned and the entry is stored in state.
- FR-002: Given a malformed digest (wrong hex length, missing colon,
  unsupported algorithm), When `UpdatePolicyStateWithVerification` is
  called, Then an error is returned and state is not modified.
- FR-003: Given a `state.json` with a policy entry whose digest is
  malformed, When `LoadState` is called, Then the entry is excluded
  from the returned State and a warning is logged containing the
  entry key and the validation error.
- FR-004: Given a `state.json` with entries having valid digests,
  When `LoadState` is called, Then all valid entries are preserved.
- FR-005: Given a `state.json` with entries having empty digest fields,
  When `LoadState` is called, Then those entries are preserved (empty
  digests are not treated as malformed).
- FR-006: Given a `state.json` with a mix of valid, malformed, and
  empty digest entries, When `LoadState` is called, Then valid and
  empty entries are preserved, malformed entries are excluded with
  warnings, and `LoadState` returns `(state, nil)`.

## Capabilities

### New Capabilities
- `digest-format-validation`: Validates OCI digest format (`algorithm:hex`)
  at both ingestion (write) and loading (read) boundaries using
  `opencontainers/go-digest`.

### Modified Capabilities
- `UpdatePolicyStateWithVerification`: Returns `error` on malformed digest.
- `UpdateComplypackStateWithVerification`: Returns `error` on malformed digest.
- `LoadState`: Warns and excludes entries with malformed digests instead of
  loading them silently. Empty digest fields are preserved.

### Removed Capabilities
- None.

## Impact

- `internal/cache/state.go` -- validation function, signature changes,
  post-load validation loop
- `internal/cache/sync.go` -- handle new error return from
  `UpdatePolicyStateWithVerification` (1 call site, line ~137)
- `internal/cache/complypack_sync.go` -- handle new error return from
  `UpdateComplypackStateWithVerification` (2 call sites, lines ~190, ~235)
- `internal/cache/*_test.go` -- ~30 test fixtures with short invalid digests
- `cmd/complyctl/cli/cli_test.go` -- ~30 test fixtures with short invalid
  digests
- `internal/doctor/doctor_test.go` -- 1 fixture calling
  `UpdatePolicyStateWithVerification` with short digest
- `internal/policy/generation_state_test.go` -- ~22 fixtures with short
  invalid digests
- `internal/cache/cachetest/` -- mock helpers using short digests
- No new dependencies (uses already-vendored `opencontainers/go-digest`)

### Documentation Impact

- **`CHANGELOG.md`**: Add entry describing digest validation behavior
  (warn+exclude on load, error on write).
- **`AGENTS.md`**: Add `digest-format-validation` to Recent Changes.
- **`README.md`**: No update needed (no CLI interface changes).
- **Website**: No update needed -- internal hardening, exempt per
  AGENTS.md website gate.

## Constitution Alignment

### I. Single Source of Truth (Centralized Constants)

**Assessment**: PASS

`ValidateDigest` centralizes digest format validation in a single function.
All validation call sites delegate to this function rather than
implementing inline checks.

### II. Simplicity & Isolation

**Assessment**: PASS

`ValidateDigest` follows SRP -- a small, focused function that validates
one thing. Each validation boundary (write, read) is independently
testable.

### III. Incremental Improvement

**Assessment**: PASS

Focused on a single concern (digest validation). No unrelated changes
included. Test fixture updates are mechanical consequences of the
validation change, not scope creep.

### V. Do Not Reinvent the Wheel

**Assessment**: PASS

Uses `digest.Parse()` from `opencontainers/go-digest`, already vendored
and imported in `internal/cache/sync.go`. No custom regex.

### VI. Composability (The Unix Philosophy)

**Assessment**: PASS

`ValidateDigest` is a standalone function usable at any boundary.
No coupling to specific callers.
