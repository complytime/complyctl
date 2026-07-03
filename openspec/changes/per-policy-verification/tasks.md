## 1. Config Schema

- [ ] 1.1 Add `Verification *VerificationConfig` and `SkipVerify bool`
  fields to `PolicyEntry` in `internal/complytime/config.go`
- [ ] 1.2 Extend `validateEntries()` to validate per-entry verification
  configs (same rules as workspace-level) with entry-identifying
  error messages
- [ ] 1.3 Add validation that `skip_verify` and `verification` on the
  same entry is an error
- [ ] 1.4 Add unit tests for per-entry config parsing, validation, and
  mutual exclusivity

## 2. Verifier Resolution and Caching

- [ ] 2.1 Introduce `resolveVerifier()` function in `get.go` that
  resolves per-entry verification config with workspace-level
  fallback and `skip_verify` handling
- [ ] 2.2 Add verifier cache (`map[VerificationConfig]VerifyFunc`)
  to avoid redundant TUF fetches for entries sharing the same config
- [ ] 2.3 Refactor `syncSinglePolicy()` and `syncSingleComplypack()`
  to receive per-entry `syncOpts` built from resolved verification
  config instead of a shared slice
- [ ] 2.4 Add unit tests for resolution priority chain:
  `--skip-verify` > `skip_verify` > entry config > workspace config
  > no verification

## 3. Error Collection

- [ ] 3.1 Refactor `syncAllPolicies()` to collect errors from all
  entries and return via `errors.Join()` instead of fail-fast
- [ ] 3.2 Refactor `syncAllComplypacks()` with the same error
  collection pattern
- [ ] 3.3 Add unit tests for error collection: partial success,
  all fail, all succeed scenarios

## 4. Integration and E2E

- [ ] 4.1 Add E2E test for mixed verification configs (keyless +
  keyed entries in the same workspace)
- [ ] 4.2 Add E2E test for `skip_verify` entry alongside verified
  entries
- [ ] 4.3 Add E2E test for error collection (one entry fails, others
  succeed)
- [ ] 4.4 Verify backward compatibility: existing configs without
  per-entry fields produce identical behavior

## 5. Documentation

- [ ] 5.1 Update AGENTS.md Recent Changes with per-policy-verification
  summary
- [ ] 5.2 Update CHANGELOG.md with the new feature entry
