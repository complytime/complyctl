## Why

The `verification:` configuration introduced in PR #670 applies a single
verification identity to all policies and complypacks. Organizations that
consume artifacts from multiple publishers with different signing
identities cannot verify all artifacts in a single `complyctl get`
invocation. They must choose between verifying one publisher's artifacts,
using `--skip-verify` (disabling all verification), or running separate
invocations -- none of which provides a robust experience for
production compliance workflows.

## What Changes

- Add optional `verification:` field to `PolicyEntry`, allowing per-entry
  verification configuration that overrides the workspace-level default
- Add optional `skip_verify:` field to `PolicyEntry`, allowing explicit
  opt-out of verification for individual entries
- Change sync error handling from fail-fast to error collection: all
  entries are attempted and errors are reported together at the end
- Cache verifier instances by config value to avoid redundant TUF
  trusted root fetches when multiple entries share the same verification
  config

Resolution priority (highest wins):

1. `--skip-verify` CLI flag: skip all verification
2. `entry.skip_verify: true`: skip this entry
3. `entry.verification:` config: use entry-level verifier
4. workspace-level `verification:` config: use workspace verifier
5. nothing configured: no verification

Validation rules:

- `skip_verify: true` and `verification:` on the same entry is an error
- Each `verification:` block (entry or workspace) is standalone; no
  field inheritance or merging between levels

## Capabilities

### New Capabilities

- `per-entry-verification`: Per-policy and per-complypack verification
  configuration with workspace-level default fallback, entry-level
  skip, verifier caching, and error collection

### Modified Capabilities

None.

## Impact

- `internal/complytime/config.go`: `PolicyEntry` struct gains two
  fields; `Validate()` / `validateEntries()` extended for per-entry
  verification validation
- `cmd/complyctl/cli/get.go`: `syncAllPolicies()` and
  `syncAllComplypacks()` change from fail-fast to error collection;
  `syncSinglePolicy()` and `syncSingleComplypack()` receive per-entry
  `syncOpts` instead of shared slice; new verifier resolution and
  caching logic replaces single `buildVerifierOpts()` call
- `internal/cache/verify.go`: No changes (VerifyFunc contract unchanged)
- `internal/cache/sync.go`, `internal/cache/complypack_sync.go`: No
  changes (SyncOption/WithVerifier contract unchanged)
- No new dependencies (uses stdlib `errors.Join` from Go 1.20+)
- No config version bump required
- Fully backward compatible: existing configs without per-entry fields
  behave identically
