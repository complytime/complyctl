## Context

Digest values in `state.json` flow through the system as plain `string`
fields with no type safety or format validation. The upstream source
(`oras-go` `Resolve()`) produces valid OCI digests, but the ingestion
layer (`UpdatePolicyStateWithVerification`, `UpdateComplypackStateWithVerification`)
and the loading layer (`LoadState`) accept any non-empty string. Test fixtures
use short invalid digests like `"sha256:abc123"` which pass silently.

## Goals

- Reject malformed digests at the write path (`Update*` methods) with
  a returned error
- Warn and exclude malformed digests at the read path (`LoadState`)
  for defense-in-depth against corrupted state files
- Use the already-vendored `opencontainers/go-digest` library (no new deps)
- Update all test fixtures to use valid-format digests

## Non-Goals

- Adding a typed `Digest` wrapper to `PolicyState` (struct change out of scope)
- Validating digests in `complypackDigestsByEvaluator()` (inputs are already
  validated upstream by the ingestion/loading layers)
- Migrating existing user `state.json` files (malformed entries are warned
  and excluded, not auto-fixed)

## Decisions

### D1: Behavior on malformed digest in `LoadState`

Warn and exclude. Log a warning per malformed entry, remove it from
the returned `State`. Do not return an error -- this preserves backward
compatibility for users whose `state.json` may have been hand-edited
or corrupted. The excluded entries can be re-fetched via `complyctl get`.

**Rationale**: Returning an error would block all CLI operations until the
user manually fixes `state.json`. Warning + exclusion is self-healing
(re-run `get` to restore valid entries).

### D2: `Update*` method signature change

Both `UpdatePolicyStateWithVerification` and
`UpdateComplypackStateWithVerification` gain an `error` return. Invalid
digests are rejected before storage. Callers in `sync.go` and
`complypack_sync.go` already have error-handling paths and will propagate
the error.

**Rationale**: This is the primary defense point. Invalid data should
never be written to `state.json` in the first place.

### D3: Validation scope

Validate at both write (`Update*`) and read (`LoadState`). The write path
prevents new bad data from entering. The read path catches pre-existing
bad data from manual edits or corruption. `complypackDigestsByEvaluator()`
does not need validation since its inputs come from `LoadState` which has
already filtered.

**Rationale**: Belt-and-suspenders approach satisfies the SI-10 principle
of validating at every handoff point.

### D4: Validation implementation

Use `digest.Parse()` from `opencontainers/go-digest` (already vendored
and imported in `internal/cache/sync.go`). This validates:
- `algorithm:hex` format
- Supported algorithms (sha256, sha384, sha512)
- Correct hex length for the algorithm (e.g., 64 hex chars for sha256)

**Rationale**: Constitution V (Do Not Reinvent the Wheel) -- the library
is already vendored, battle-tested, and used elsewhere in the codebase
(`classifyVersion` in `sync.go`).

## Risks / Trade-offs

- **Test fixture churn**: ~40 test digest strings across multiple files
  need valid-format replacements. High line count but mechanical and low
  risk. Each file can be updated independently.
- **Signature change on `Update*` methods**: All callers need to handle
  the new error return. There are few callers (2 in `sync.go`, 2 in
  `complypack_sync.go`, a few in tests) so impact is contained.
- **Existing corrupted state.json**: Users with malformed digests in
  their state file will see warnings and lose those entries. This is
  intentional -- `complyctl get` restores them with valid digests.
