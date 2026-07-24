## Context

`BuildLookupRef()` constructs OCI references using only `repository`, `tag`, and `digest` -- omitting the registry host. When these references reach `name.ParseReference()` in the verification path, `go-containerregistry` defaults the host to `index.docker.io`. Signature verification therefore checks Docker Hub instead of the registry configured in `complytime.yaml`.

The registry host is available at the construction site in `cmd/complyctl/cli/get.go` (via `ref.Registry` from `ParsePolicyRef`) but is never threaded through to the `Sync`/`ComplypackSync` structs where `BuildLookupRef` is called.

Three verification call sites are affected:
1. `internal/cache/sync.go:119` -- policy pre-copy verification
2. `internal/cache/complypack_sync.go:142` -- complypack pre-copy verification
3. `internal/cache/complypack_sync.go:221` -- complypack local cache re-verification

Two additional `BuildLookupRef` call sites (`sync.go:96`, `complypack_sync.go:72`) feed `DefinitionVersion()` which resolves its own host internally. These are not affected by the bug but MUST be updated because the `BuildLookupRef` signature change is not backward-compatible.

## Goals / Non-Goals

**Goals:**
- Ensure all OCI signature verification resolves against the configured registry host, not Docker Hub
- Fail closed when registry host is unavailable and verification is configured (security-critical)
- Maintain all existing test coverage with CRAP scores below 30

**Non-Goals:**
- Changing the `VerifyFunc` type signature
- Modifying `PolicySource`/`ComplypackSource` interfaces or mock implementations in `cachetest/`
- Changing the `registry.Client` API surface
- Altering `SyncPolicy`/`SyncComplypack` method signatures

## Decisions

### D1: Add `registryHost` field to sync structs

**Choice:** Add a `registryHost string` field to both `Sync` and `ComplypackSync` structs, populated at construction time via updated `NewSync`/`NewComplypackSync` constructors. Update `BuildLookupRef` to accept the host as its first parameter and prepend it.

**Rationale:** This is the smallest correct change. Each struct has exactly one construction site (`get.go`), so the redundancy risk of storing host separately from the `source` is eliminated by the single-constructor constraint. `VerifyFunc`, `SyncPolicy`/`SyncComplypack` method signatures, and all interfaces remain unchanged.

**Alternatives considered:**
- *Thread `registryHost` through `SyncPolicy`/`SyncComplypack` method parameters*: Higher churn, touches method signatures that are part of the package API, and the host is a property of the sync session not individual calls.
- *Embed full registry reference in `PolicySource`*: Would require interface changes and mock updates in `cachetest/`, much larger blast radius for no additional correctness gain.

### D2: Fail-closed when `registryHost` is empty with verifier configured

**Choice:** If `verifier` is non-nil and `registryHost` is empty, `SyncPolicy`/`SyncComplypack` MUST return an error immediately.

**Rationale:** Verifying against an unknown registry is a security defect, not a graceful degradation. The previous behavior (defaulting to Docker Hub) meant an attacker could publish a signed artifact on Docker Hub that passes verification while the actual policy was fetched from a different registry. This is a deliberate fail-closed security decision.

### D3: Backward-compatible `BuildLookupRef` behavior when host is empty

**Choice:** When `registryHost` is empty, `BuildLookupRef` output MUST match pre-change behavior (no host prefix, no leading `/`).

**Rationale:** Non-verification call sites (`DefinitionVersion()`) may rely on the current format. Preserving backward compatibility for empty host eliminates risk of breaking those paths.

## Risks / Trade-offs

- **[Breaking change for misconfigured users]** Users with `verification:` configured but bare policy IDs (no registry host in URL) will see an error instead of silent verification against Docker Hub. **Mitigation:** This is intentional -- the previous behavior provided false security guarantees. Error message will clearly indicate the registry host is required.
- **[Single-constructor invariant]** The `registryHost` field MUST match the registry host used by `PolicySource`/`ComplypackSource`. Both derive from `ref.Registry` at the single construction site in `get.go`. **Mitigation:** Document the invariant. Future refactors adding construction sites MUST preserve this correspondence.
