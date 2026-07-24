## 1. Update BuildLookupRef signature and tests (TDD: test first)

- [ ] 1.1 Update `TestBuildLookupRef` table in `internal/cache/sync_test.go`: add `registryHost` field to test struct, keep existing cases with empty host for backward compatibility, add new cases with non-empty host (tests will fail to compile)
- [ ] 1.2 Add `registryHost string` as first parameter to `BuildLookupRef` in `internal/cache/sync.go`; prepend `registryHost + "/"` when non-empty, preserve existing output when empty (tests pass)

## 2. Update Sync struct and constructor (TDD: test first)

- [ ] 2.1 Write regression tests in `internal/cache/sync_test.go` (alongside existing verification tests): (a) construct `Sync` with `registryHost` = `registry.example.com`, call `SyncPolicy` with a capturing `VerifyFunc` closure that records the `registryRef` argument, assert `registryRef` has `registry.example.com/` prefix and does NOT have `index.docker.io/` prefix; (b) construct `Sync` with empty `registryHost` and non-nil `VerifyFunc`, assert `SyncPolicy` returns error containing "registry host" (tests fail to compile)
- [ ] 2.2 Update all existing `NewSync` calls in `internal/cache/sync_test.go` and `internal/policy/loader_test.go` to include `registryHost` argument (tests still fail to compile)
- [ ] 2.3 Add `registryHost string` field to `Sync` struct and `registryHost string` parameter to `NewSync` in `internal/cache/sync.go`
- [ ] 2.4 Update both `BuildLookupRef` calls in `SyncPolicy` (lines ~96 and ~119) to pass `s.registryHost`
- [ ] 2.5 Add fail-closed validation in `SyncPolicy`: if `s.verifier` is non-nil and `s.registryHost` is empty, return error containing "registry host" before calling `VerifyFunc` (tests pass)

## 3. Update ComplypackSync struct and constructor (TDD: test first)

- [ ] 3.1 Write regression tests in `internal/cache/complypack_sync_test.go` (alongside existing verification tests): (a) construct `ComplypackSync` with `registryHost` = `registry.example.com`, call `SyncComplypack` with a capturing `VerifyFunc` closure, verify all three call sites (pre-copy and cache re-verification) receive host-qualified `registryRef` with `registry.example.com/` prefix; (b) construct `ComplypackSync` with empty `registryHost` and non-nil `VerifyFunc`, assert `SyncComplypack` returns error containing "registry host" (tests fail to compile)
- [ ] 3.2 Update all existing `NewComplypackSync` calls in `internal/cache/complypack_sync_test.go` and `internal/cache/complypack_pipeline_test.go` to include `registryHost` argument (tests still fail to compile)
- [ ] 3.3 Add `registryHost string` field to `ComplypackSync` struct and `registryHost string` parameter to `NewComplypackSync` in `internal/cache/complypack_sync.go`
- [ ] 3.4 Update all three `BuildLookupRef` calls in `SyncComplypack` (lines ~72, ~142, ~221) to pass `s.registryHost`
- [ ] 3.5 Add fail-closed validation in `SyncComplypack`: if `s.verifier` is non-nil and `s.registryHost` is empty, return error containing "registry host" before calling `VerifyFunc` (tests pass)

## 4. Update construction site in get.go

- [ ] 4.1 Pass `ref.Registry` to `NewSync` call in `cmd/complyctl/cli/get.go` (line ~382)
- [ ] 4.2 Pass `ref.Registry` to `NewComplypackSync` call in `cmd/complyctl/cli/get.go` (line ~577)

## 5. Verification

- [ ] 5.1 Run `make test-unit` and confirm all tests pass with no regressions
- [ ] 5.2 Run `make lint` and confirm no linting violations
- [ ] 5.3 Verify CRAP scores remain below threshold with `make crapload-check`

## 6. Documentation and governance

- [ ] 6.1 Add entry to `AGENTS.md` Recent Changes section documenting the security fix and BREAKING behavior change
- [ ] 6.2 Assess whether `docs/QUICK_START.md` verification guidance needs updating for fail-closed semantics
- [ ] 6.3 Update `governance/threats/complytime-threats.yaml` THR02: refine MIT02 description to note host-qualified verification, add MIT04 for fail-closed behavior when registry host is empty with verifier configured
