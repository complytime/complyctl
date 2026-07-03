## Context

The sigstore-go verification feature (PR #670) introduced a single
workspace-level `verification:` block in `complytime.yaml`. The
`buildVerifierOpts()` function in `get.go` constructs one verifier
and passes it to every sync call via a shared `[]cache.SyncOption`
slice. This design assumed a single signing identity across all
artifacts.

Organizations consuming policies from multiple publishers need
per-entry verification configuration. The cache layer
(`Sync`/`ComplypackSync`) already accepts a `VerifyFunc` via the
functional-options pattern and is unaware of config structure --
only the command layer (`get.go`) needs changes.

## Goals / Non-Goals

**Goals:**

- Allow per-policy and per-complypack verification configuration
  that overrides the workspace-level default
- Allow explicit opt-out of verification for individual entries
  via `skip_verify`
- Collect all sync errors and report them together instead of
  aborting on the first failure
- Cache verifier instances to avoid redundant TUF trusted root
  fetches when entries share the same verification config
- Maintain full backward compatibility with existing configs

**Non-Goals:**

- Field-level inheritance between workspace and entry verification
  configs (each block is standalone)
- Changes to the cache layer (`internal/cache/`) contracts
- Config version bump
- New doctor diagnostic checks
- Per-entry verification for targets (targets reference policies,
  they do not fetch artifacts)

## Decisions

### D1: Standalone verification blocks, no field merging

Each `verification:` block (entry-level or workspace-level) is a
complete, standalone configuration. When an entry has its own
`verification:`, it replaces the workspace default entirely -- no
field-level merging.

**Alternatives considered:**

- *Partial override*: Entry specifies only `key:`, inherits
  `issuer:` from workspace. Rejected because `VerificationConfig`
  has two mutually exclusive modes (keyless vs keyed), making
  partial merging semantically ambiguous and error-prone.
- *Deep merge with explicit null*: YAML null to clear inherited
  fields. Rejected as overly complex for three fields.

**Rationale:** Simple to reason about -- "what verification does
this entry use?" is answered by looking at exactly one place. No
merge logic, no "which field came from where?" debugging.

### D2: `skip_verify` as a separate boolean field

`PolicyEntry` gains a `SkipVerify bool` field separate from the
`Verification *VerificationConfig` pointer. Setting both on the
same entry is a validation error.

**Alternatives considered:**

- *Overload empty verification block*: `verification: {}` means
  "skip." Rejected because nil (omitted) already means "inherit
  workspace," making the distinction between nil and empty subtle
  and error-prone in YAML.
- *Sentinel inside VerificationConfig*: `verification: { skip:
  true }`. Rejected because it pollutes the verification struct
  with a field that means "ignore all other fields."

**Rationale:** `skip_verify: true` is a clear intent declaration.
It reads naturally and doesn't overload the verification config
semantics.

### D3: Resolution in the command layer, not the cache layer

Per-entry verification resolution happens in `get.go`. The cache
layer continues to receive a pre-built `VerifyFunc` via
`WithVerifier()` and remains config-unaware.

**Rationale:** The command layer already knows about config
structure (`WorkspaceConfig`, `PolicyEntry`). The cache layer's
contract (`SyncOption`/`VerifyFunc`) is clean and should not gain
config awareness. This preserves the existing separation of
concerns.

### D4: Verifier caching by config value

A `map[VerificationConfig]VerifyFunc` cache avoids constructing
duplicate verifiers when multiple entries share the same config.
`VerificationConfig` is a struct with three string fields, so it
is directly comparable and works as a map key.

**Rationale:** `NewKeylessVerifier` fetches the TUF trusted root
(~30s network call). Without caching, N entries with the same
workspace-level config would trigger N fetches. The cache is
scoped to a single `complyctl get` invocation (no persistence).

### D5: Error collection with `errors.Join`

`syncAllPolicies` and `syncAllComplypacks` collect all errors and
return them via `errors.Join()` (stdlib since Go 1.20). All
entries are attempted regardless of individual failures.

**Alternatives considered:**

- *Fail-fast (current behavior)*: Stop on first error. Rejected
  because in multi-publisher scenarios, one registry being
  unreachable should not prevent fetching from others.
- *Custom multi-error type*: Rejected as unnecessary when stdlib
  `errors.Join` exists and the codebase targets Go 1.25.

**Rationale:** Users get the full picture in one invocation.
Partially successful fetches are still useful -- cached policies
from previous runs remain available for scanning.

### D6: No config version bump

The new fields are additive. Old complyctl versions silently
ignore unknown YAML fields (`goccy/go-yaml` default behavior).
The failure mode is conservative: old complyctl applies
workspace-level verification to all entries, which fails closed
if the entry needs a different verifier.

**Rationale:** Bumping `CurrentWorkspaceVersion` to 2 would
reject all existing `version: 1` configs on new complyctl (the
validation uses `!=` not `>`). That is a breaking change. The
fail-closed behavior of old complyctl with new config fields is
acceptable and safe.

## Risks / Trade-offs

- **[Silent field drop on old complyctl]** Old complyctl ignores
  per-entry verification fields, applying workspace-level
  verification instead. This fails closed (verification error,
  not silent bypass). Mitigation: the error message from the sync
  layer identifies the policy and failure reason.

- **[Verifier cache unbounded]** The cache grows with distinct
  `VerificationConfig` values. In practice this is bounded by
  the number of entries in `complytime.yaml` (typically < 10).
  Mitigation: none needed; the cache is per-invocation and
  garbage-collected on exit.

- **[Error collection changes UX]** Users accustomed to fail-fast
  behavior will now see multiple errors at once. Mitigation: this
  is strictly more informative. The exit code remains non-zero
  when any error occurs.
