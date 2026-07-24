## Why

`BuildLookupRef()` in `internal/cache/sync.go` constructs OCI references without a registry host, causing `go-containerregistry` to default to `index.docker.io`. Signature verification therefore checks against Docker Hub instead of the registry configured in `complytime.yaml`. This is a security defect: an attacker could publish a signed artifact on Docker Hub that passes verification while the actual policy was fetched from a different registry. Fixes [#767](https://github.com/complytime/complyctl/issues/767).

## What Changes

- **BREAKING**: `BuildLookupRef` gains a `registryHost` parameter prepended to the OCI reference when non-empty.
- `Sync` and `ComplypackSync` structs gain a `registryHost` field populated at construction time from `ref.Registry`.
- `NewSync` and `NewComplypackSync` constructors gain a `registryHost` parameter.
- All `BuildLookupRef` call sites updated to pass the registry host.
- **BREAKING**: Verification now fails closed -- if `registryHost` is empty and a verifier is configured, sync returns an error instead of silently verifying against Docker Hub.

## Capabilities

### Modified Capabilities
- `artifact-verification`: Fix registry host resolution so signature verification checks the configured registry instead of Docker Hub; add fail-closed semantics when registry host is unavailable with a verifier configured.

## Impact

- `internal/cache/sync.go` -- `BuildLookupRef` signature change, `Sync` struct and `NewSync` constructor updated
- `internal/cache/complypack_sync.go` -- `ComplypackSync` struct and `NewComplypackSync` constructor updated
- `cmd/complyctl/cli/get.go` -- passes `ref.Registry` to constructors
- `internal/cache/sync_test.go` -- all `BuildLookupRef` and `NewSync` test calls updated
- `internal/cache/complypack_sync_test.go` -- all `NewComplypackSync` test calls updated
- `internal/policy/loader_test.go` -- `NewSync` calls updated for new constructor signature
- `internal/cache/complypack_pipeline_test.go` -- `NewComplypackSync` calls updated for new constructor signature
- No interface changes, no mock interface updates, no `VerifyFunc` signature changes

## Constitution Alignment

Assessed against the Unbound Force org constitution.

### I. Autonomous Collaboration

**Assessment**: PASS

This change preserves artifact-based communication. Error messages from fail-closed behavior are self-describing and guide users toward correct configuration. All spec artifacts maintain self-describing outputs.

### II. Composability First

**Assessment**: N/A

No new mandatory dependencies introduced. The change modifies internal function signatures without altering external interfaces.

### III. Observable Quality

**Assessment**: PASS

Fail-closed error messages contain actionable guidance ("registry host" substring). Verification results continue to produce machine-parseable state in `PolicyState.Verified`/`SignerIdentity`/`Issuer` fields.

### IV. Testability

**Assessment**: PASS

All new behavior is testable in isolation via unit tests on `BuildLookupRef`, `SyncPolicy`, and `SyncComplypack`. Regression tests verify the core security property (registry host prefix in `VerifyFunc` arguments) using capturing closures.
