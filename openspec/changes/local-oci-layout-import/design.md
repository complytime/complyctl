# Design

See `proposal.md — Why` for motivation.

## Context

`complyctl get` constructs a `RegistrySource` / `RegistryComplypackSource` for
each configured entry, then passes it into the existing `Sync` /
`ComplypackSync` pipeline. Those pipelines call two interface methods:

- `DefinitionVersion(ctx, ref) (digest, version, error)` — resolves manifest
  metadata.
- `CopyPolicy(ctx, policyID, version, destStore) (desc, error)` — copies
  content into the destination OCI store.

Both `Sync` and `ComplypackSync` already accept these as interfaces
(`internal/cache/source.go:16-20`,
`internal/cache/complypack_source.go:19-32`). No changes to those pipelines are
required; only new local source implementations are needed.

The vendored `oras.land/oras-go/v2/content/oci` package provides
`oci.NewFromFS(ctx, fsys)` (`readonlyoci.go:37-64`), a read-only OCI store that
validates `oci-layout` and `index.json`, resolves digests, and exposes a `Fetch`
interface compatible with `oras.Copy`. This primitive is already in the vendor
tree; no new dependency is required.

Verification currently runs before copying via `s.verifier` in `SyncPolicy` and
`SyncComplypack`. The verifier receives a registry reference and retrieves
signature material from the registry (`internal/cache/verify.go:208-283`). No
local-layout path exists in the verifier today.

## Goals / Non-Goals

**Goals**
- Implement `LocalPolicySource` and `LocalComplypackSource` using the existing
  source interfaces.
- Add `--from-oci-layout <path>` to `complyctl get`.
- Preserve all existing cache, state, metadata, and incremental-skip behavior.
- Fail closed per entry when verification is configured without `--skip-verify`,
  before any network-capable verification or registry initialization.
- Record permitted imports as `Verified: false`.

**Non-Goals**
- Offline signature verification (follow-up issue, builds on #768).
- Whole-workspace export/import (`complyctl export`) — follow-on.
- Changes to the `Sync`, `ComplypackSync`, `PolicyLoader`, or `Resolver`
  packages.
- Changes to `complytime.yaml` configuration format.

## Decisions

### D1: Implement local sources as new types rather than modifying existing ones

`LocalPolicySource` and `LocalComplypackSource` implement the existing
`PolicySource` and `ComplypackSource` interfaces. The `get` CLI branches on the
flag to construct one or the other.

**Alternative considered**: A flag on `RegistrySource` to read locally. Rejected
because it would mix registry and local concerns in one type and require
threading the path through the registry client constructor.

### D2: Use `oci.NewFromFS` (read-only store) for the input layout

`oci.NewFromFS` validates the layout structure, resolves digests, and exposes
`Fetch` — exactly what `oras.Copy` needs as a source. Using the writable
`oci.New` would silently create missing layout files, which would hide user
errors.

### D3: Artifact selection by configured digest; no repository matching

In local mode, the configured URL MUST include an OCI digest. That digest is the
sole selector for an artifact in the local layout. The configured registry and
repository remain the cache/state identity, consistent with online behavior, but
MUST NOT participate in local artifact selection and MUST NOT be contacted. The
layout path has no enforced subdirectory structure in this per-entry model.

**Alternative considered**: Require the layout to mirror the XDG cache directory
structure (whole-workspace import). Deferred to the follow-on whole-workspace
import issue — one entry at a time is explicit and avoids inventing a directory
convention before user feedback.

### D4: Fail closed for configured verification per entry

When `s.verifier != nil` in the sync pipeline, the current design calls
`s.verifier(ctx, registryRef, insecure)` before copying. For a local source,
no registry reference exists. Rather than silently skipping verification, the
CLI inspects each entry's declarative verification configuration before
constructing a verifier. When `--skip-verify` is absent, it returns an error
for that entry before loading a trusted root, starting TUF, creating a registry
client, resolving credentials, constructing its source, or changing its cache
or state; it continues with remaining entries.

This preserves the invariant established by `WithVerifier`'s contract
(`sync.go:56-63`): failed verification leaves the cache unchanged.

**Alternative considered**: Pass a no-op verifier that returns `Verified: false`.
Rejected because it silently downgrades a user's explicit security configuration
without their knowledge.

### D5: `Verified: false` recorded for permitted local imports

State updates use the existing `UpdatePolicyStateWithVerification` and
`UpdateComplypackStateWithVerification` paths, passing `nil` for
`*VerificationResult`. This is already handled: a nil result writes
`Verified: false` with empty signer/issuer fields.

### D6: Select local sources before registry initialization

For permitted local entries, `get` MUST select `LocalPolicySource` or
`LocalComplypackSource` before it creates a registry client or invokes
credential resolution. The verification rejection precedes this selection.
Together, these ordering constraints make the no-network contract structural:
tests inject guards for verifier construction, trusted-root loading, TUF
startup, registry-client creation, and credential resolution, and assert that
all remain unused for a rejected entry.

**Alternative considered**: Create registry clients first and bypass them during
local copying. Rejected because client construction can resolve credentials and
violates the local-mode no-network contract.

### D7: Complypack unpack pipeline unchanged

`LocalComplypackSource` only needs to satisfy `DefinitionVersion` and
`CopyComplypack`. The existing `ComplypackSync.SyncComplypack` unpack-and-store
pipeline (`complypack_sync.go:173-220`) runs unmodified on the copied content.
Artifact media type validation (`complypack_source.go:50-98`) is preserved by
delegating to the existing validation helper.

## Risks / Trade-offs

**[Risk] Configured digest absent from the layout** — The portable layout may
lack an artifact required by the workspace.
→ Mitigation: the local source resolves only the configured digest and returns
an entry-local error before copying when that digest is absent.

**[Risk] Partial import leaves workspace in mixed state** — Some entries succeed
and some fail, consistent with existing `get` behavior.
→ Mitigation: This matches current behavior for partial registry failures.
`complyctl doctor` will surface incomplete cache entries. Document this
explicitly.

**[Risk] Users treat `Verified: false` as secure** — Imported content has no
cryptographic provenance.
→ Mitigation: `complyctl list` shows the VERIFIED column; `complyctl doctor`
surfaces unverified entries; documentation is explicit about this limitation.

## Migration Plan

- No configuration changes; existing `complytime.yaml` files are unchanged.
- Existing online `complyctl get` is unaffected.
- `docs/QUICK_START.md` offline-workflow section updated to document the new
  flag and supersede the manual `oras push` workaround for policy cache seeding.
  The manual workaround remains valid but is no longer the only documented path.
- No rollback complexity: the flag is additive. Removing it would restore prior
  behavior.

## Open Questions

- Should the command print a summary of imported, skipped (incremental), and
  failed entries, analogous to the post-sync summary in `bundle-metadata-cli`?
  Deferred — implementation can address it without changing the spec.
- Should `complyctl doctor` flag entries imported without verification when
  verification is configured? Likely yes, but deferred to a follow-up.
