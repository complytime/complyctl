<!--
  [P] marks tasks eligible for parallel execution.
-->

## 1. Add ValidateDigest function

- [ ] 1.1 Add `ValidateDigest(s string) error` to `internal/cache/state.go`
  using `digest.Parse()` from `opencontainers/go-digest`

## 2. Add validation at write path

- [ ] 2.1 Change `UpdatePolicyStateWithVerification` to return `error`;
  call `ValidateDigest(digest)` and return error if invalid
- [ ] 2.2 Change `UpdateComplypackStateWithVerification` to return `error`;
  call `ValidateDigest(digest)` and return error if invalid
- [ ] 2.3 Update caller in `internal/cache/sync.go` to handle returned error
- [ ] 2.4 Update caller in `internal/cache/complypack_sync.go` to handle
  returned error

## 3. Add validation at read path

- [ ] 3.1 Add post-unmarshal validation loop in `LoadState` that iterates
  `Policies` and `Complypacks` maps, calls `ValidateDigest` on each
  `Digest` field, logs warning and removes entries with malformed digests

## 4. Update test fixtures to use valid-format digests

- [ ] 4.1 [P] Update `internal/cache/state_test.go`
- [ ] 4.2 [P] Update `internal/cache/sync_test.go`
- [ ] 4.3 [P] Update `internal/cache/complypack_sync_test.go`
- [ ] 4.4 [P] Update `internal/cache/complypack_test.go`
- [ ] 4.5 [P] Update `internal/cache/verify_test.go`
- [ ] 4.6 [P] Update `internal/cache/complypack_source_test.go`
- [ ] 4.7 [P] Update `cmd/complyctl/cli/cli_test.go`
- [ ] 4.8 [P] Update mock helpers in `internal/cache/cachetest/` if needed

## 5. Add dedicated validation tests

- [ ] 5.1 [P] Test `ValidateDigest` with valid sha256 digest
- [ ] 5.2 [P] Test `ValidateDigest` with valid sha384/sha512 digests
- [ ] 5.3 [P] Test `ValidateDigest` with empty string
- [ ] 5.4 [P] Test `ValidateDigest` with missing colon
- [ ] 5.5 [P] Test `ValidateDigest` with wrong hex length
- [ ] 5.6 [P] Test `ValidateDigest` with unsupported algorithm
- [ ] 5.7 Test `UpdatePolicyStateWithVerification` rejects malformed digest
- [ ] 5.8 Test `UpdateComplypackStateWithVerification` rejects malformed digest
- [ ] 5.9 Test `LoadState` warns and excludes entries with malformed digests
- [ ] 5.10 Test `LoadState` preserves entries with valid digests

## 6. Verification

- [ ] 6.1 Run `make test-unit` -- all tests pass
- [ ] 6.2 Run `make lint` -- zero lint issues
- [ ] 6.3 Run `make vet` -- passes
- [ ] 6.4 Run `make crapload-check` -- no CRAP regressions
