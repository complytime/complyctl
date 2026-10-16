# Proposal

## Why

`complyctl get` requires a live OCI registry connection, making initial cache
population impossible in air-gapped or network-restricted environments without
standing up local registry infrastructure. Organizations operating in classified,
regulated, or disconnected environments need a first-class, supported mechanism
for populating the policy and complypack cache from portable artifacts.

## What Changes

- Add `--from-oci-layout <path>` flag to `complyctl get`.
- When the flag is supplied, resolve each configured policy and complypack from
  the provided local OCI Image Layout directory instead of a remote registry.
- In local mode, each configured `url` MUST include an OCI digest. That digest
  selects the artifact from the layout; the registry and repository identify
  only the cache and state entry. No registry client or credential lookup is
  performed.
- Content is imported into the normal XDG cache, and `state.json` is updated
  consistently with all existing sync metadata (digest, version, evaluator ID,
  policy metadata).
- Verification fails closed per entry: an entry with configured verification
  requires explicit `--skip-verify` and fails before verifier construction,
  trusted-root loading, TUF startup, registry-client creation, credential
  resolution, or cache/state changes. Other entries continue. Imports without
  configured verification, or with an explicit skip, are recorded as
  `Verified: false`.
- `--timeout` and `--skip-verify` flags remain applicable in local-layout mode.
- Existing online `complyctl get` behavior is unchanged.

## Capabilities

### New Capabilities

- `local-oci-layout-import`: Import policies and complypacks from a local OCI
  Image Layout directory, with consistent cache/state updates, content-integrity
  validation, and fail-closed verification behavior for air-gapped environments.

### Modified Capabilities

- `cache-sync`: The existing sync pipeline gains a local source implementation
  (`LocalPolicySource`, `LocalComplypackSource`) that satisfies the existing
  `PolicySource` and `ComplypackSource` interfaces. Behavior for digest
  comparison, incremental skip, and state updates is unchanged; only the source
  of artifact bytes differs.

## Impact

- **`cmd/complyctl/cli/get.go`**: New `--from-oci-layout` flag; select local
  sources before registry-client creation or credential resolution when the flag
  is set.
- **`internal/cache/source.go`**: New `LocalPolicySource` struct implementing
  `PolicySource`.
- **`internal/cache/complypack_source.go`**: New `LocalComplypackSource` struct
  implementing `ComplypackSource`.
- **`internal/cache/sync.go`** / **`internal/cache/complypack_sync.go`**: No
  interface changes required; existing `Sync` and `ComplypackSync` accept the
  new local source implementations.
- **`docs/QUICK_START.md`**: Update offline-workflow guidance with the new flag,
  partial-success behavior, failure reporting and exit status, identifying
  imported entries, and safe retry guidance.
- **Vendored dependency**: Uses existing `oras.land/oras-go/v2/content/oci`
  `ReadOnlyStore` already vendored; no new dependencies.
- **Follow-on issues**: Fully offline signature verification and whole-workspace
  export/import are explicitly out of scope.
