## Context

`complyctl get` implements OCI artifact signature verification via
`sigstore-go` with keyed (cosign keypair) and keyless (Fulcio+Rekor) modes.
Unit tests cover config parsing, state tracking, and resolver logic. The
existing acceptance test stack (`tests/acceptance/`) validates the lifecycle
profile (get/scan/report) against a real zot OCI registry but does not
exercise signature verification.

Issue #678 requires end-to-end acceptance tests that verify the full
verification pipeline against a real OCI registry with real cosign
signatures. This builds on PR #670 (sigstore-go verification) and
PR #667 (utility function tests).

## Goals / Non-Goals

> **Superseded (see IMPLEMENTATION RECONCILIATION in tasks.md):** the
> witness service was dropped, keyless tests are no longer skip-gated
> (they run against a committed-fileca Fulcio + Trillian CTFE log), and
> `trusted_root` was implemented and is exercised here. The Goals,
> Non-Goals, Container Image Sources table, Decision #6, Decision #7,
> and Risks below record the interim plan for historical context.

### Goals
- Acceptance-test keyed verification (cosign keypair) end-to-end
- Stand up Sigstore infrastructure (Fulcio, Rekor, Dex, witness) for
  keyless verification; keyless tests skip-gated pending `trusted_root`
- Test configuration behaviors: `--skip-verify`, per-entry `skip_verify`,
  per-entry verification override, unsigned artifact rejection
- Validate user-facing surfaces: `complyctl list` VERIFIED column,
  `complyctl doctor` verification diagnostics
- Reuse existing compose-based acceptance test pattern

### Non-Goals
- Re-testing sigstore-go's internal verification correctness
- Testing complypack verification (same code path as policies)
- Implementing `trusted_root` field (follow-on issue)
- TLS between containers (test stack uses plain HTTP)

## Decisions

### Container Image Sources

> **Superseded:** the shipped stack drops witness and uses
> `gcr.io/projectsigstore/*` (fulcio, rekor-server) plus
> `gcr.io/trillian-opensource-ci/*` (db_server, log_server, log_signer,
> ctfe) rather than the `ghcr.io/sigstore/*` images below, and adds a
> `ctfe` CT log + `createtree-init`. See the reconciliation note in
> tasks.md for the authoritative committed digests.

The verification profile uses the following container images (pin by
digest in compose.yaml, not floating tags):

| Service    | Image                                        | Healthcheck                              |
|------------|----------------------------------------------|------------------------------------------|
| dex-idp    | `ghcr.io/dexidp/dex`                         | OIDC discovery: `/.well-known/openid-configuration` |
| fulcio     | `ghcr.io/sigstore/fulcio`                    | `curl -f http://localhost:5555/healthz`  |
| rekor      | `ghcr.io/sigstore/rekor/rekor-server`          | `curl -f http://localhost:3000/api/v1/log` |
| witness    | `ghcr.io/transparency-dev/witness/omniwitness` | `curl -sf http://localhost:8100/healthz` or file check `test -s /data/witness.log` |

Resolve current digests at implementation time from each project's
releases page. Pin the exact `@sha256:` digest in compose.yaml.

### 1. Separate compose profile

**Decision**: Add a `verification` profile to the existing `compose.yaml`
rather than a separate compose file.

**Rationale**: The `zot` service is shared between profiles, avoiding
duplicate registry configuration. Docker Compose profile semantics allow
`test-acceptance` and `test-acceptance-verify` to run independently. A
separate compose file would duplicate the zot service definition and
network configuration. When both profiles are activated simultaneously
(e.g., `--profile lifecycle --profile verification`), the zot service
starts once and both seed containers push to separate repository paths
(`acceptance-test` vs `verify-*`), so no conflict occurs.

### 2. Committed test PKI (fileca + CTFE)

> **Reconciliation note**: an interim implementation used a Fulcio
> ephemeral in-memory CA with no committed PKI. That was superseded
> once keyless verification had to satisfy the production sigstore-go
> verifier's SCT requirement, which needs a real CT log sharing a
> committed root with Fulcio. See the IMPLEMENTATION RECONCILIATION
> note at the top of `tasks.md` for the full delta.

**Decision**: Check committed PKI material into
`tests/acceptance/testdata/sigstore/`: the Fulcio fileca root
(`root.pem`+`root.key`) and the CTFE signing key
(`privkey.pem`+`ctfe.pub`). Fulcio runs `--ca=fileca` and the CTFE
CT log signs SCTs with the CTFE key; both trust the committed root.
`sign-seed.sh` assembles `trusted_root.json` at runtime from these
committed keys plus the live Rekor public key. Dex/Fulcio config
live at `tests/acceptance/`.

**Rationale**: The keyless verifier requires an embedded SCT from a
CT log, and the CT log's `--roots_pem_file` must match the CA Fulcio
issues from, so Fulcio and the CT log must share a stable committed
root. Runtime key generation cannot satisfy that chicken-and-egg
ordering deterministically. The material is test-only with zero
security value.

### 3. sign-seed init container

**Decision**: Use a dedicated `sign-seed` container (UBI-minimal +
cosign + oras) to push testdata, sign artifacts, and generate
`trusted_root.json`, rather than signing at image build time.

**Rationale**: Signing requires a running Fulcio and Rekor for keyless
mode. The sign-seed container runs after all Sigstore services are
healthy, ensuring correct dependency ordering. Cosign keypair signing
(keyed mode) also happens here for consistency, even though it does not
require Fulcio/Rekor.

### 4. Three repository paths

**Decision**: Push identical Gemara testdata to three zot repository
paths with different signing treatments:
- `policies/verify-keyed` -- cosign keypair
- `policies/verify-keyless` -- Fulcio + Rekor keyless
- `policies/verify-unsigned` -- no signing

**Rationale**: Isolating signing modes by repository path allows tests
to use different `complytime.yaml` configurations per test case without
re-pushing or re-signing content.

### 5. Shared acceptance binary

**Decision**: Verification tests use the same `acceptance.test` binary
and `//go:build acceptance` tag as the existing lifecycle tests. The
`verify-sut` container filters with `-test.run=TestVerification`.

**Rationale**: Avoids a second compile target and keeps all acceptance
tests in one package. Test filtering via `-test.run` is the standard
Go mechanism for partitioning test execution. To ensure lifecycle
isolation, verification tests include a `skipIfNoVerificationEnv(t)`
guard that skips when `COSIGN_KEY_PATH` is not set, preventing them
from running in the lifecycle profile's `sut` container.

### 6. Keyless tests skip-gated

> **Superseded:** keyless tests are NOT skip-gated in the shipped
> implementation. `trusted_root` was added to `VerificationConfig` and
> the keyless scenarios run live against a committed-fileca Fulcio and a
> Trillian-backed CTFE log, with `trusted_root.json` assembled at
> runtime from the committed CA + CTFE keys plus the live Rekor key.

**Decision**: Keyless test scenarios (`TestVerification_Keyless_*`) use
`t.Skip("requires trusted_root support")` until the `trusted_root`
field is added to `VerificationConfig`.

**Rationale**: The current `NewKeylessVerifier` uses Sigstore's public
good TUF root, which does not trust the local Fulcio/Rekor instances.
A `trusted_root` field is needed to point verification at the local
infrastructure. Standing up the infrastructure now ensures the tests
can be unblocked with a single field addition rather than a large
infrastructure change.

### 7. Separate CI workflow

**Decision**: New `.github/workflows/acceptance_verify_test.yml`
separate from `acceptance_test.yml`.

**Rationale**: Sigstore infrastructure (5+ containers) is heavier and
more failure-prone than the lifecycle stack (zot + seed + sut). Keeping
workflows separate prevents Sigstore flakiness from blocking core
acceptance CI.

## Risks / Trade-offs

- **[Container image stability]** Rekor-tiles and omniwitness use
  rolling tags or `main` branches. Mitigated by digest-pinning in
  compose.yaml.

- **[Compose stack complexity]** The verification profile adds 6 new
  services (dex, fulcio, rekor, witness, sign-seed, verify-sut).
  Mitigated by profile isolation -- lifecycle tests never start these
  services.

- **[Keyless test coverage deferred]** Keyless tests skip until
  `trusted_root` lands. Mitigated by standing up full infrastructure
  now so the unblock is a single config field addition.

- **[CI resource usage]** Sigstore stack requires more memory/CPU than
  lifecycle tests. Estimated peak memory: ~4 GB for combined stack
  (7 containers). GitHub Actions `ubuntu-latest` provides 7 GB RAM /
  2 CPUs, which should be sufficient. If CI OOM-kills occur, consider
  adding `mem_limit` to compose services or switching to a larger
  runner. Mitigated by separate workflow with independent timeout
  (15 minutes).
