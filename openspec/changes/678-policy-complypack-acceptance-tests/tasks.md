<!-- spec-review: passed -->
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

<!--
  IMPLEMENTATION RECONCILIATION (2026-08-05)

  The tasks below record the ORIGINAL plan. The shipped
  implementation diverged in several ways once the real
  Sigstore trust chain was wired end-to-end. This note
  reconciles task text with what was actually delivered;
  the checkboxes are left as the historical record.

  - PKI (section 1). NOTE: an interim design used a Fulcio
    ephemeral in-memory CA with no committed PKI; that was
    SUPERSEDED once keyless verification had to satisfy the
    production sigstore-go verifier's SCT requirement. The
    shipped design provisions a real RFC6962 CT log so keyless
    signatures carry a genuine SCT, which requires Fulcio and
    the CT log to share a committed root. Committed test PKI
    now lives at `tests/acceptance/testdata/sigstore/`:
    `root.pem`+`root.key` (Fulcio fileca CA, `--ca=fileca`
    `--fileca-key-passwd=fulcio`), `privkey.pem`+`ctfe.pub`
    (CTFE signing key, PEM legacy-encrypted, passphrase
    `ctfe`). `dex-config.yaml` and `fulcio-config.yaml` live
    at `tests/acceptance/`. `sign-seed.sh` still builds
    `trusted_root.json` at RUNTIME via `cosign trusted-root
    create`, now from the committed CA + CTFE keys plus the
    live Rekor key. Rekor/witness keys were NOT needed
    (see below); the keys have zero security value.

  - Witness dropped entirely (tasks 1.3, 1.6, 2.1). complyctl's
    verify path (`NewKeylessVerifier`) checks a Fulcio cert +
    SCT + Rekor transparency-log entry only; cosign keyless
    signing contacts Fulcio and Rekor exclusively. No code
    path contacts a witness, so a witness service would test
    infrastructure nothing exercises (zero-waste).

  - Rekor requires Trillian + MySQL (section 2). Rekor has no
    in-memory storage mode, so the verification profile adds
    `mysql`, `trillian-log-server`, and `trillian-log-signer`
    alongside `rekor-server`, mirroring Sigstore's own
    reference docker-compose. The CT log (`ctfe`,
    `gcr.io/trillian-opensource-ci/ctfe`) reuses that same
    mysql+trillian backend; a `createtree-init` service
    (`Dockerfile.createtree`, `createtree@v1.6.1`) provisions
    the Trillian tree and renders `ct_server.cfg` before
    `ctfe` starts. Fulcio's `--ct-log-url` points at
    `http://ctfe:6962/testlog`.

  - Image registries: `gcr.io/projectsigstore/*` (fulcio,
    rekor-server), `gcr.io/trillian-opensource-ci/*` (db_server,
    log_server, log_signer), `ghcr.io/dexidp/dex` -- not the
    `ghcr.io/sigstore/*` / transparency-dev paths named in
    task 2.1. All are digest-pinned.

  - `Dockerfile.sign-seed` runs as ROOT, not `USER nobody`
    (task 3.1). It writes to the root-owned `/shared` named
    volume and generates keys under `/tmp`; private keys never
    leave the container. It also installs `curl` + `jq` for
    the OIDC token exchange and trust-root assembly.

  - Keyless signing is REQUIRED, not skip-on-failure (task 4.3).
    With the trust chain present, `sign-seed.sh` fails closed
    (`set -euo pipefail`, empty/null token guard) rather than
    silently skipping keyless signing.

  - Keyless tests are no longer skip-gated (section 7). Both
    `TestVerification_Keyless_HappyPath` and
    `TestVerification_Keyless_WrongIdentity` are un-skipped and
    thread a `trusted_root` path (from `TRUSTED_ROOT_PATH`,
    default `/shared/trusted_root.json`) into
    `withKeylessVerification(issuer, identity, trustedRootPath)`.
    `WrongIdentity` asserts the stable wrap prefix
    `"signature verification failed"` plus an `"identity"`
    substring to distinguish an identity mismatch from a
    wrong-key failure.

  - `WrongKey`/`UnsignedWithVerification` cache-absence
    assertions now resolve the policy cache via
    `complytime.ResolveCacheDir()` (XDG) instead of the legacy
    `~/.complytime/...` path.

  - Makefile `test-acceptance-verify` dumps `ps -a` and logs
    for ALL verification-profile services (via
    `$(VERIFICATION_COMPOSE) logs`) on both the up-failure
    branch and the post-wait failure path before teardown,
    satisfying task 9.1's all-services log-capture intent
    (the witness service itself was dropped).

  - Verification code fixes surfaced by the live stack
    (`internal/cache/verify.go`): honor the `http://`
    plain-HTTP registry scheme for cosign-signature
    resolution; emit a PublicKey verification material for
    keyed (no-cert) bundles; hex-decode the Rekor bundle
    logID (cosign writes it as hex); use the signature-layer
    (simplesigning payload) digest as both the
    message-signature digest and the verifier artifact
    digest. Every verification test clears the global policy
    cache so the incremental-digest skip cannot mask a
    re-verification; the keyed happy-path assertion is
    decoupled from the `doctor` providers check.
-->

## 1. Pre-Generated Test PKI

- [x] 1.1 [P] Generate Fulcio root CA certificate and private key (`fulcio-root.pem`, `fulcio-root.key`) using `openssl ecparam -genkey -name prime256v1` (EC P-256), self-signed with CA:TRUE basic constraint, write to `tests/acceptance/testdata/sigstore/`
- [x] 1.2 [P] Generate Rekor Ed25519 signing keypair (`rekor-signing-key.pem`, `rekor-signing-pub.pem`) using `openssl genpkey -algorithm ed25519`, write to `tests/acceptance/testdata/sigstore/`
- [x] 1.3 [P] Generate witness private key (`witness-private.key`) using `openssl genpkey -algorithm ed25519`, write to `tests/acceptance/testdata/sigstore/`
- [x] 1.4 [P] Create Dex OIDC configuration (`dex-config.yaml`) with `mockCallback` connector, issuer `http://dex-idp:5556/dex`, `web: { http: "0.0.0.0:5556" }`, static client `complyctl-test` with secret `test-secret` (test-only mock credential, matches task 4.3), test user `admin@example.com` with password `password` (test-only); write to `tests/acceptance/testdata/sigstore/`
- [x] 1.5 [P] Create Fulcio identity configuration (`fulcio-identity.yaml`) trusting local Dex issuer, write to `tests/acceptance/testdata/sigstore/`
- [x] 1.6 [P] Create witness policy configuration (`witness-policy.yaml`) and witness log configuration (`witness-config.yaml`), write to `tests/acceptance/testdata/sigstore/`
- [x] 1.7 [P] Create `tests/acceptance/testdata/sigstore/README.md` with header `# TEST-ONLY PKI -- DO NOT REUSE` explaining these keys have zero security value and MUST NOT be used outside the acceptance test stack

## 2. Compose Stack

- [x] 2.1 Extend `tests/acceptance/compose.yaml` with `verification` profile: update the existing `zot` service profiles from `[lifecycle]` to `[lifecycle, verification]` to share the registry across both profiles; add `dex-idp` (`ghcr.io/dexidp/dex`, healthcheck: `curl -sf http://localhost:5556/dex/.well-known/openid-configuration`), `fulcio` (`ghcr.io/sigstore/fulcio`, healthcheck: `curl -f http://localhost:5555/healthz`), `rekor` (`ghcr.io/sigstore/rekor/rekor-server`, healthcheck: `curl -f http://localhost:3000/api/v1/log`), `witness` (`ghcr.io/transparency-dev/witness/omniwitness`, healthcheck: `curl -sf http://localhost:8100/healthz || test -s /data/witness.log`) services with digest-pinned images (pin specific sha256 digests, not floating tags), healthchecks, and correct dependency ordering
- [x] 2.2 Add `sign-seed` service to compose.yaml (verification profile): depends on dex-idp, fulcio, rekor, zot healthy; runs `sign-seed.sh`; mounts testdata and shared volume
- [x] 2.3 Add `verify-sut` service to compose.yaml (verification profile): depends on sign-seed completed; runs `acceptance.test -test.run=TestVerification`; mounts shared volume for cosign keys and trusted_root; environment variables: `REGISTRY_URL` (zot address), `COSIGN_KEY_PATH` (path to cosign.pub in shared volume), `COSIGN_WRONG_KEY_PATH` (path to wrong.pub), `TRUSTED_ROOT_PATH` (path to trusted_root.json), `VERIFY_KEYED_POLICY_ID`, `VERIFY_KEYLESS_POLICY_ID`, `VERIFY_UNSIGNED_POLICY_ID`

## 3. Container Images

- [x] 3.1 [P] Create `tests/acceptance/Dockerfile.sign-seed` (UBI-minimal base, install oras CLI version-pinned with SHA256 checksum matching `Dockerfile.seed` pattern, install cosign CLI version-pinned with SHA256 checksum -- e.g., `ARG COSIGN_VERSION=2.4.3` / `ARG COSIGN_SHA256=<checksum>` from cosign releases checksums.txt, copy sign-seed.sh, `USER nobody` matching `Dockerfile.seed` security pattern); only mount `cosign.pub` and `wrong.pub` into shared volume (private keys stay in sign-seed container)
- [x] 3.2 [P] Create `tests/acceptance/Dockerfile.verify-sut` (same pattern as existing `Dockerfile.sut`, copy acceptance binary and complyctl binary)

## 4. Signing Script

- [x] 4.1 Create `tests/acceptance/sign-seed.sh` with `set -euo pipefail` (matching `seed.sh` error handling pattern): push Gemara testdata (`catalog.yaml:application/vnd.gemara.catalog.v1+yaml` and `policy.yaml:application/vnd.gemara.policy.v1+yaml` from mounted `/testdata`, matching `seed.sh` media types) to three zot repository paths (`policies/verify-keyed`, `policies/verify-keyless`, `policies/verify-unsigned`); verify each push succeeded before proceeding
- [x] 4.2 In `sign-seed.sh`: set `COSIGN_PASSWORD=""` for non-interactive key generation (empty passphrase, acceptable for test-only keys), generate cosign keypair with `cosign generate-key-pair`, sign `policies/verify-keyed` with retry (up to 3 attempts with 2s backoff on transient failure) using `cosign sign --key --tlog-upload=false --allow-http-registry`
- [x] 4.3 In `sign-seed.sh`: obtain OIDC token from Dex via direct HTTP token exchange (`curl -s -d 'grant_type=password&username=admin@example.com&password=password&scope=openid email&client_id=complyctl-test&client_secret=test-secret' http://dex-idp:5556/dex/token` -- credentials MUST match those configured in `dex-config.yaml` from task 1.4; these are test-only mock credentials with no security value), sign `policies/verify-keyless` with retry (up to 3 attempts with 2s backoff on transient failure) using `cosign sign --fulcio-url --rekor-url --identity-token=$TOKEN --allow-http-registry`; if OIDC token acquisition fails, log warning and skip keyless signing (allow keyed tests to proceed independently)
- [x] 4.4 In `sign-seed.sh`: generate `trusted_root.json` from local Fulcio root cert and Rekor public key, write to shared volume
- [x] 4.5 In `sign-seed.sh`: generate a second cosign keypair ("wrong key") for negative test cases, write to shared volume

## 5. Verification Test Helpers

- [x] 5.1 Create `tests/acceptance/verification_helpers_test.go` with `//go:build acceptance` tag: `writeVerificationConfig(t *testing.T, dir string, registryURL string, opts ...verifyOption)` helper using functional options pattern -- options: `withKeyedVerification(keyPath string)`, `withKeylessVerification(issuer, identity string)`, `withSkipVerify()`, `withPolicyEntries(entries []policyEntryConfig)` for multi-entry scenarios; writes `complytime.yaml` with appropriate `verification:` and `policies:` blocks
- [x] 5.2 In `verification_helpers_test.go`: `readPolicyState()` helper that reads and unmarshals `state.json` for verification assertions
- [x] 5.3 In `verification_helpers_test.go`: `runComplyctlExpectError()` helper that runs complyctl and returns combined output + error without calling `t.Fatalf`, enabling assertion on stderr content and exit code for negative tests

- [x] 5.4 In `verification_test.go`: add a package-level `skipIfNoVerificationEnv(t)` guard that calls `t.Skip("verification env vars not set")` when `COSIGN_KEY_PATH` is not set; call this guard at the start of every `TestVerification_*` function (sections 6-8) to prevent verification tests from running in the lifecycle profile's `sut` container, which does not set these env vars

## 6. Verification Tests -- Keyed

- [x] 6.1 Implement `TestVerification_Keyed_HappyPath` in `tests/acceptance/verification_test.go`: configure keyed verification with correct cosign.pub, run `complyctl get`, assert `verified: true` in state, assert `complyctl list` shows VERIFIED=Yes, assert `complyctl doctor` reports verified
- [x] 6.2 Implement `TestVerification_Keyed_WrongKey` in `tests/acceptance/verification_test.go`: configure keyed verification with wrong.pub, run `complyctl get`, assert non-zero exit code, assert stderr contains `"signature verification failed"` (from `verify.go:171`), assert no policy content in cache

## 7. Verification Tests -- Keyless (Skip-Gated)

- [x] 7.1 Implement `TestVerification_Keyless_HappyPath` in `tests/acceptance/verification_test.go`: configure keyless verification with Dex issuer and identity, `t.Skip("requires trusted_root support")`, assert `verified: true` with signer_identity and issuer populated
- [x] 7.2 Implement `TestVerification_Keyless_WrongIdentity` in `tests/acceptance/verification_test.go`: configure keyless with wrong identity, `t.Skip("requires trusted_root support")`, assert failure with identity mismatch error

## 8. Verification Tests -- Configuration Behavior

- [x] 8.1 Implement `TestVerification_SkipVerifyFlag` in `tests/acceptance/verification_test.go`: configure keyed verification, run `complyctl get --skip-verify`, assert success with `verified: false`, assert stderr contains `"WARNING: signature verification skipped"` (from `get.go:107`) and `"NOTE: policy"` followed by `"was fetched without"` (from `get.go:402`)
- [x] 8.2 Implement `TestVerification_PerEntrySkipVerify` in `tests/acceptance/verification_test.go`: workspace-level keyed verification, two policy entries (verify-keyed inherits workspace verification, verify-unsigned with `skip_verify: true`); expected YAML structure: `verification: { key: <cosign.pub> }` at workspace level, `policies: [{ url: .../verify-keyed }, { url: .../verify-unsigned, skip_verify: true }]`; assert verify-keyed `verified=true` in state, verify-unsigned `verified=false` in state
- [x] 8.3 Implement `TestVerification_PerEntryOverride` in `tests/acceptance/verification_test.go`: workspace-level keyless (will not work without trusted_root), one entry overrides with per-entry keyed verification (`verification: { key: <cosign.pub> }`), second entry uses `skip_verify: true` to avoid keyless failure; assert keyed entry `verified=true`, skipped entry `verified=false`; this tests per-entry override precedence without requiring working keyless infrastructure
- [x] 8.4 Implement `TestVerification_UnsignedWithVerification` in `tests/acceptance/verification_test.go`: keyed verification configured, policy points to verify-unsigned, assert non-zero exit code, assert stderr contains `"no cosign signature found"` (from `verify.go:247`), assert `state.json` does not contain an entry for verify-unsigned policy, assert no OCI layout directory exists for verify-unsigned under the cache

## 9. Build and CI

- [x] 9.1 [P] Add `test-acceptance-verify`, `test-acceptance-verify-clean`, and `test-acceptance-all` targets to `Makefile`; add `VERIFICATION_COMPOSE` variable with `--profile verification` (matching `ACCEPTANCE_COMPOSE` pattern); `test-acceptance-verify` depends on `build build-test-provider build-acceptance-test` (matching `test-acceptance` prerequisite pattern); `test-acceptance-verify` MUST capture container logs from all verification-profile services (sign-seed, dex-idp, fulcio, rekor, witness, verify-sut) on failure before teardown, matching the existing log capture pattern in `test-acceptance`; `test-acceptance-verify-clean` tears down verification profile containers and volumes; `test-acceptance-all` depends on `test-acceptance` and `test-acceptance-verify`, running them sequentially (lifecycle profile first, then verification profile)
- [x] 9.2 [P] Create `.github/workflows/acceptance_verify_test.yml` (triggers: push to main, `pull_request: types: [opened, synchronize]` matching `acceptance_test.yml` pattern; runner: ubuntu-latest; timeout: 15m; steps: checkout, setup Go, `make test-acceptance-verify COMPOSE="docker compose"`)

## 10. Documentation

- [x] 10.1 Update `AGENTS.md` CI Workflow Structure table with `acceptance_verify_test.yml` entry
- [x] 10.2 Update `AGENTS.md` Test commands section with `make test-acceptance-verify` and `make test-acceptance-all`
- [x] 10.3 Update `AGENTS.md` Project Structure with `testdata/sigstore/` under `tests/acceptance/`
- [x] 10.4 Update `AGENTS.md` Recent Changes with verification acceptance tests entry
- [x] 10.5 Update `docs/TESTING_ENVIRONMENT.md` "Container-Based Acceptance Tests" section with: verification profile architecture (Sigstore services + sign-seed + verify-sut), `make test-acceptance-verify` and `make test-acceptance-all` commands, new CI workflow reference, and verification profile prerequisites

## 11. Regression Verification

- [x] 11.1 Run `make test-acceptance` to confirm existing lifecycle profile is unaffected by compose.yaml changes
- [x] 11.2 Run `make test-acceptance-verify` to confirm the new verification profile works end-to-end
