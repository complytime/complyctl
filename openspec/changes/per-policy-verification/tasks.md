## 1. Config Schema

- [x] 1.1 Add `Verification *VerificationConfig` and `SkipVerify bool`
  fields to `PolicyEntry` in `internal/complytime/config.go`
- [x] 1.2 Extend `validateEntries()` to validate per-entry verification
  configs (same rules as workspace-level) with entry-identifying
  error messages
- [x] 1.3 Add validation that `skip_verify` and `verification` on the
  same entry is an error
- [x] 1.4 Add unit tests for per-entry config: `LoadFrom` YAML
  roundtrip tests for new fields (deserialization of `verification:`
  and `skip_verify:` on entries), `Validate` tests for mutual
  exclusivity, per-entry verification validation rules, and backward
  compatibility (existing `PolicyEntry` structs without new fields
  pass validation unchanged)

## 2. Verifier Resolution and Caching

- [x] 2.1 Introduce `resolveVerifier()` function in `get.go` that
  resolves per-entry verification config with workspace-level
  fallback and `skip_verify` handling
- [x] 2.2 Add verifier cache (`map[VerificationConfig]VerifyFunc`)
  to avoid redundant TUF fetches for entries sharing the same config
- [x] 2.3 Refactor `syncSinglePolicy()` and `syncSingleComplypack()`
  to receive per-entry `syncOpts` built from resolved verification
  config instead of a shared slice
- [x] 2.4 Add unit tests for resolution priority chain:
  `--skip-verify` > `skip_verify` > entry config > workspace config
  > no verification. Include negative/boundary cases: `skip_verify:
  false` (explicit) behaves identically to omitted; `verification:
  {}` (empty struct) falls through to workspace config; verifier
  cache hit/miss with identical and different configs

## 3. Error Collection

- [x] 3.1 Refactor `syncAllPolicies()` to collect errors from all
  entries and return via `errors.Join()` instead of fail-fast
- [x] 3.2 Refactor `syncAllComplypacks()` with the same error
  collection pattern
- [x] 3.3 Add unit tests for error collection: partial success,
  all fail, all succeed scenarios. Verify error messages include
  effective policy IDs and individual errors are unwrappable

## 4. Integration and E2E

E2E tests inject mock `VerifyFunc` instances via the existing
`SyncOption`/`WithVerifier()` functional options pattern to test
the resolution and error collection logic without requiring real
sigstore infrastructure or signed artifacts.

- [x] 4.1 Add E2E test for mixed verification configs (keyless +
  keyed entries in the same workspace) using mock verifiers
- [x] 4.2 Add E2E test for `skip_verify` entry alongside verified
  entries
- [x] 4.3 Add E2E test for error collection (one entry fails, others
  succeed)
- [x] 4.4 Verify backward compatibility: existing configs without
  per-entry fields produce identical behavior

## 5. Documentation

- [x] 5.1 [P] Update AGENTS.md Recent Changes with
  per-entry-verification summary
- [x] 5.2 [P] Update CHANGELOG.md with the new feature entry
- [x] 5.3 ~~File website issue~~ Skipped: complytime org does not
  use unbound-force/website for documentation
<!-- spec-review: passed -->
<!-- code-review: passed -->
