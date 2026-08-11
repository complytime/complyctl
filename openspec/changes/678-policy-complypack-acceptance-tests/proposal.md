## Why

`complyctl get` implements OCI artifact signature verification via `sigstore-go`
(keyed and keyless modes). Unit tests cover parsing, state tracking, and resolver
logic, but no end-to-end acceptance test exercises the full verification flow
against a real OCI registry with real cosign signatures. Without acceptance
coverage, regressions in the verification pipeline (pre-copy timing, state
persistence, CLI surfaces) would go undetected until manual testing.

## What Changes

> **Note**: the bullets below record the original proposal. The shipped
> implementation dropped the witness service, added a real CTFE CT log
> (Fulcio `--ca=fileca` with committed PKI), and un-skipped the keyless
> tests. See the IMPLEMENTATION RECONCILIATION note at the top of
> `tasks.md` and Decision #2 in `design.md` for the full delta.

- Extend `tests/acceptance/compose.yaml` with a `verification` profile that
  stands up Sigstore infrastructure (Dex, Fulcio, Rekor, witness) alongside
  the existing zot registry.
- Add a `sign-seed` init container that pushes Gemara testdata to zot under
  three repository paths with different signing treatments: cosign keypair,
  Fulcio+Rekor keyless, and unsigned.
- Add a `verify-sut` container that runs verification acceptance tests against
  the seeded registry.
- Pre-generate all test PKI material in `tests/acceptance/testdata/sigstore/`
  for deterministic container startup.
- Implement 8 test scenarios across keyed, keyless, and configuration behavior
  groups. Keyless tests are skip-gated pending `trusted_root` support.
- Add `make test-acceptance-verify` and `make test-acceptance-all` targets.
- Add `.github/workflows/acceptance_verify_test.yml` CI workflow, separate
  from existing acceptance tests to isolate Sigstore infrastructure failures.

## Capabilities

### New Capabilities
- `verification-acceptance-tests`: End-to-end acceptance tests for OCI
  artifact signature verification covering keyed mode, keyless mode
  (skip-gated), skip-verify flag, per-entry overrides, and unsigned
  artifact rejection against a real OCI registry with real cosign
  signatures.

### Modified Capabilities
- `acceptance-tests`: Extended compose stack gains `verification` profile
  with Sigstore services; existing `lifecycle` profile unchanged.

## Impact

- **New files**: `Dockerfile.sign-seed`, `Dockerfile.verify-sut`,
  `verification_test.go`, `verification_helpers_test.go`, `sign-seed.sh`,
  `testdata/sigstore/*` (9 PKI/config files), CI workflow
- **Modified files**: `compose.yaml` (new profile + services), `Makefile`
  (new targets), `AGENTS.md` (CI table, test commands, project structure,
  recent changes)
- **Production code**: `internal/cache/verify.go`, `sync.go`, and
  `complypack_sync.go` gain plain-HTTP (`http://`) registry scheme
  support and keyed/keyless bundle fixes surfaced by the live stack
  (see the tasks.md IMPLEMENTATION RECONCILIATION note). The
  `VerifyFunc` signature gains an `insecure bool` parameter, threaded
  through all call sites.
- **Dependencies**: Sigstore container images (dex, fulcio, rekor-tiles,
  omniwitness) digest-pinned in compose.yaml; cosign CLI in sign-seed
  container

## Constitution Alignment

Assessed against the Unbound Force org constitution.

### I. Autonomous Collaboration

**Assessment**: PASS

Acceptance tests are self-contained in the compose stack. All
artifacts (PKI, configs, test binaries) are committed and
self-describing. No external service dependencies at runtime.

### II. Composability First

**Assessment**: PASS

Verification tests use a separate compose profile (`verification`)
and a separate Makefile target (`test-acceptance-verify`). The
existing `lifecycle` profile and `test-acceptance` target are
unchanged. Each profile can run independently.

### III. Observable Quality

**Assessment**: PASS

Test output follows Go test conventions with machine-parseable
results. Container logs are captured on failure for debugging.
State files are inspected programmatically for verification
assertions.

### IV. Testability

**Assessment**: PASS

This change is itself a test infrastructure addition. All
components are testable in isolation via compose profiles.
