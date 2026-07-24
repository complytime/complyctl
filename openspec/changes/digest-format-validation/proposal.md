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
  state files.
- Test fixtures across `internal/cache/*_test.go` and
  `cmd/complyctl/cli/cli_test.go` updated to use valid-format digests.

## Capabilities

### New Capabilities
- `digest-format-validation`: Validates OCI digest format (`algorithm:hex`)
  at both ingestion (write) and loading (read) boundaries using
  `opencontainers/go-digest`.

### Modified Capabilities
- `UpdatePolicyStateWithVerification`: Returns `error` on malformed digest.
- `UpdateComplypackStateWithVerification`: Returns `error` on malformed digest.
- `LoadState`: Warns and excludes entries with malformed digests instead of
  loading them silently.

### Removed Capabilities
- None.

## Impact

- `internal/cache/state.go` -- validation function, signature changes,
  post-load validation loop
- `internal/cache/sync.go` -- handle new error return from
  `UpdatePolicyStateWithVerification`
- `internal/cache/complypack_sync.go` -- handle new error return from
  `UpdateComplypackStateWithVerification`
- `internal/cache/*_test.go` -- ~20 test fixtures with short invalid digests
- `cmd/complyctl/cli/cli_test.go` -- ~20 test fixtures with short invalid
  digests
- `internal/cache/cachetest/` -- mock helpers using short digests
- No new dependencies (uses already-vendored `opencontainers/go-digest`)

## Constitution Alignment

### I. Autonomous Collaboration

**Assessment**: PASS

Warning messages on malformed digests are self-describing, identifying the
specific entry and the validation failure.

### II. Composability First

**Assessment**: PASS

Validation is additive. The `ValidateDigest` function is standalone and
reusable. No new dependencies introduced.

### III. Observable Quality

**Assessment**: PASS

Malformed digests are now observable via warnings at load time and errors at
write time, rather than propagating silently.

### IV. Testability

**Assessment**: PASS

`ValidateDigest` is independently testable. Each validation boundary
(write, read) is testable in isolation.
