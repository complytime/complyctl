<!--
  [P] marks tasks eligible for parallel execution.
  Add [P] when a task: (a) touches different files from
  other [P] tasks in the group, (b) has no dependency
  on prior tasks in the group, (c) can safely execute
  without ordering constraints.
  Do NOT add [P] when tasks modify the same file --
  parallel workers will cause merge conflicts.
  Tasks without [P] run sequentially first, then [P]
  tasks run in parallel.
-->

## 1. Add ValidateDigest function

- [x] 1.1 Add `ValidateDigest(s string) error` to `internal/cache/state.go`
  using `digest.Parse()` from `opencontainers/go-digest`; empty strings
  bypass validation and return nil

## 2. Add validation at write path

- [x] 2.1 Change `UpdatePolicyStateWithVerification` to return `error`;
  call `ValidateDigest(digest)` and return error if invalid
- [x] 2.2 Change `UpdateComplypackStateWithVerification` to return `error`;
  call `ValidateDigest(digest)` and return error if invalid
- [x] 2.3 Update caller in `internal/cache/sync.go` (line ~137) to handle
  returned error with context wrapping
- [x] 2.4 Update callers in `internal/cache/complypack_sync.go` (lines ~190,
  ~235) to handle returned error with context wrapping

## 3. Add validation at read path

- [x] 3.1 Add post-unmarshal validation loop in `LoadState` that iterates
  `Policies` and `Complypacks` maps, calls `ValidateDigest` on each
  `Digest` field, logs warning via `charmbracelet/log.Warn` with entry
  key and remediation hint, and removes entries with malformed digests;
  empty digest fields are preserved; returns `(state, nil)` even when
  entries are excluded

## 4. Update test fixtures to use valid-format digests

- [x] 4.1 [P] Update `internal/cache/state_test.go`
- [x] 4.2 [P] Update `internal/cache/sync_test.go`
- [x] 4.3 [P] Update `internal/cache/complypack_sync_test.go`
- [x] 4.4 [P] Update `internal/cache/complypack_test.go`
- [x] 4.5 [P] Update `internal/cache/verify_test.go`
- [x] 4.6 [P] Update `internal/cache/complypack_source_test.go`
- [x] 4.7 [P] Update `cmd/complyctl/cli/cli_test.go`
- [x] 4.8 [P] Update mock helpers in `internal/cache/cachetest/`
- [x] 4.9 [P] Update `internal/doctor/doctor_test.go`
- [x] 4.10 [P] Update `internal/policy/generation_state_test.go`
- [x] 4.11 [P] Update `internal/output/evaluator_test.go`
- [x] 4.12 [P] Update `internal/output/step_identity_test.go`

## 5. Add dedicated validation tests

- [x] 5.1 Add table-driven `TestValidateDigest` covering valid digests
  (sha256, sha384, sha512), empty string, missing colon, wrong hex
  length, and unsupported algorithm (D4)
- [x] 5.2 Test `UpdatePolicyStateWithVerification` rejects malformed digest
  and returns error (D2)
- [x] 5.3 Test `UpdateComplypackStateWithVerification` rejects malformed
  digest and returns error (D2)
- [x] 5.4 Test `LoadState` warns and excludes entries with malformed digests
  while preserving valid and empty-digest entries (D1)

## 6. Verification

- [x] 6.1 Run `make test-unit` -- all tests pass
- [x] 6.2 Run `make lint` -- zero lint issues
- [x] 6.3 Run `make vet` -- passes
- [x] 6.4 Run `make sanity` -- vendor + format + vet + git diff check
- [x] 6.5 Run `make crapload-check` -- no CRAP regressions

## 7. Documentation

- [x] 7.1 [P] Update `CHANGELOG.md` with digest validation entry
- [x] 7.2 [P] Update `AGENTS.md` Recent Changes with
  digest-format-validation summary
<!-- spec-review: passed -->
