## Context

complyctl and complytime-providers were a single repository until spec 004 split the
providers into their own repo. The split preserved the SDK contract (`pkg/provider`) and
the gRPC wire protocol (`api/plugin/plugin.proto`) as the integration boundary, but left
no automated mechanism to verify that the two repositories remain compatible after
independent changes. All existing tests in both repositories are self-contained: complyctl
uses a purpose-built `test-provider` binary stub; complytime-providers uses interface-based
mocks and injectable command runners. Neither exercises the real binary integration.

The `reusable_compliance.yml` workflow in org-infra is the only existing workflow that
wires the two together, but it was written before the provider split and now references
stale paths (`bin/ampel-plugin`, `cmd/ampel-plugin/`). Fixing that workflow is a
follow-up; this change establishes the correct pattern for PR-time cross-repo testing.

## Goals / Non-Goals

**Goals:**

- Validate on every PR that the `complyctl` binary and the real `complyctl-provider-ampel`
  binary can discover each other, complete the gRPC handshake, exchange typed data
  through Describe / Generate / Scan RPCs, and produce correctly formatted scan output.
- Validate that the Gemara policy layer (OCI fetch, policy resolution, assessment
  configuration extraction) correctly drives the Ampel provider through the full pipeline.
- Keep test content minimal (1 control, 1 assessment plan, 1 granular policy) so the
  test is fast, deterministic, and easy to understand.
- Design for easy extension: adding a second Ampel rule or a future OpenSCAP provider
  should require only additive changes (new files, new test functions).

**Non-Goals:**

- Testing the Ampel provider's internal logic (CEL evaluation, attestation parsing) —
  that is covered by complytime-providers unit tests.
- Testing all 5 branch-protection rules from the existing policy — 1 rule is sufficient
  to validate the integration boundary.
- Adding the mirrored CI workflow to `complytime-providers` — tracked as a separate
  change in that repository, to be implemented once this change is merged.
- Updating `org-infra/reusable_compliance.yml` — noted as a follow-up.
- OpenSCAP provider integration — deferred to a future change.
- RPM-level integration testing — deferred per spec 005 (FR-027).

## Decisions

### D1: complyctl owns the test script (Option C)

The integration test script and all test fixture content live in the complyctl
repository. The complytime-providers CI workflow checks out complyctl and runs the
script from there.

**Rationale**: complyctl defines the integration contract — the provider SDK, the gRPC
protocol, and the Gemara policy format. Owning the test harness here means the contract
owner also owns the test that validates it. The providers repo only needs to build its
binary and delegate. This avoids duplicating test logic across repositories and keeps
the test script in sync with the codebase it primarily exercises.

**Alternative rejected**: Each repo maintains its own workflow with duplicated test
logic (Option A). Rejected because duplication creates drift — the two scripts would
inevitably diverge.

**Alternative rejected**: Centralize the test script in org-infra (Option B). Rejected
because the test content (mock registry testdata, assertion logic) is tightly coupled
to complyctl internals and would be harder to maintain in a separate repo.

### D2: PR branch vs main for each repo

When a PR is filed in complyctl, the workflow builds complyctl from the PR branch and
checks out `complytime-providers@main` to build the stable provider binary. When the
mirrored workflow is later added to `complytime-providers`, it will do the inverse:
build the provider from the PR branch and check out `complyctl@main`.

**Rationale**: The goal is to catch regressions introduced by the PR under review. Using
`main` for the stable side is the least surprising convention. Building from source
(not release artifacts) is required because PRs have not been released yet.

**Note on spec 004 guidance**: Spec 004 recommended using release artifacts for the
`ci_compliance.yml` workflow to avoid coupling CI to source. That guidance applies to
compliance scanning workflows (which run against stable releases), not to PR-time
integration tests (which by definition need the unreleased change). The distinction is:
release-time integration uses artifacts; PR-time integration builds from source.

### D3: Real tools (snappy + ampel), real scan target

The integration test uses the real `snappy` and `ampel` CLI binaries installed via
`carabiner-dev/actions/install/snappy` and `carabiner-dev/actions/install/ampel`.
The scan target is `https://github.com/complytime/complyctl`, a public repository with
branch protection enabled. The standard `GITHUB_TOKEN` (automatically available in
Actions) provides read access to the GitHub branch rules API.

**Rationale**: The user's requirement is functional integration ("ensure the binaries
can read expected content format and produce correct outcomes"). Mock shims would not
validate that snappy correctly collects branch protection data or that ampel correctly
evaluates the CEL policy. Using the real tools and a real public target provides
meaningful coverage of the full pipeline without requiring secrets beyond the
standard token.

**Risk**: If `complytime/complyctl` branch protection rules change, the test outcome
(PASS/FAIL) may change. Mitigation: the test asserts that scan completes and produces
output (attestation files exist, results contain the expected requirement ID), not
that a specific PASS/FAIL result is returned. A failed policy check is a valid scan
outcome, not a test failure.

### D4: Minimal test-specific Gemara content, new mock registry entry

Rather than reusing the existing 5-rule `ampel-branch-protection` policy (which would
require providing all 5 granular policy files or silently skipping 4 of them), a
dedicated minimal policy is added to the mock registry under
`policies/test-branch-protection`. It contains exactly 1 control (`force-push-protection`)
and 1 assessment plan (`block-force-push`) with executor `ampel`.

**Rationale**: Using a dedicated test policy makes the test self-contained and its
intent obvious. The ID chain (catalog → policy → granular policy → complytime.yaml) uses
descriptive names (`block-force-push`) rather than opaque framework codes (`BP-3.01`),
making the test easier to read and extend.

### D5: Granular policy file lives in complyctl test fixtures

The `block-force-push.json` granular policy file is stored in
`tests/cross-repo/testdata/granular-policies/` within the complyctl repository (not in
complytime-providers). The CI workflow copies it into the workspace before running the
test.

**Rationale**: Granular policies are provider-side artifacts that a user (or CI) must
supply; they are not distributed via OCI. For the integration test, having the fixture
in complyctl keeps the test self-contained. If the granular policy format changes in the
future (e.g., new required fields), the test will fail and the fixture will need
updating — which is the desired behavior, as it signals a breaking change.

### D6: Scan outcome assertion strategy

The test asserts:
1. `complyctl get` succeeds and the OCI layout is written to disk.
2. `complyctl generate` succeeds and the policy bundle JSON is written.
3. `complyctl scan` exits 0 (tool-level success, even if policy controls evaluate FAIL).
4. Snappy attestation file exists in `.complytime/ampel/results/`.
5. Ampel result attestation file exists in `.complytime/ampel/results/`.
6. The ampel result attestation contains the `block-force-push` requirement ID.

**Exit-code semantics**: `complyctl scan` exits 0 when the pipeline completed (snappy
collected data, ampel evaluated the policy, and attestations were written), regardless
of whether individual controls passed or failed. A non-zero exit indicates a tool-level
failure (binary not found, gRPC handshake failure, OCI fetch error, etc.). The test
treats exit 0 as success and any non-zero exit as a test failure.

The test does NOT assert a specific PASS or FAIL result for the scanned repository,
because branch protection configuration can change. It asserts that the pipeline ran
to completion and produced structured output.

## Risks / Trade-offs

**[Network dependency]** → The test calls the live GitHub API via snappy. Transient
API failures or rate limiting could cause flaky CI. Mitigation: the standard
`GITHUB_TOKEN` has generous rate limits for branch rules API calls. The test scans a
single branch of a single public repository, requiring at most 1 API call per run.

**[Branch protection configuration drift]** → If `complytime/complyctl` branch
protection rules change, the scan result (PASS/FAIL) changes. Mitigation: assertions
check output structure, not specific pass/fail status (see D6).

**[snappy/ampel version pinning]** → The `carabiner-dev/actions/install/` actions
install a specific version of snappy and ampel. If those versions become incompatible
with the provider's expected attestation format, the test will fail.
Mitigation: the action versions are pinned by SHA in the workflow.

**[Build time]** → Building both repositories from source in CI adds time compared to
downloading release artifacts. With Go module caching this is acceptable for PR
validation. The test is scoped to one provider and one rule to keep total runtime low.

**[complytime-providers CI coupling]** → Once the mirrored workflow is added to
`complytime-providers`, it will pin `complyctl@main`. If complyctl's main branch is
broken, providers PRs will also fail the cross-repo test. Mitigation: complyctl's own
CI must pass before merging to main, so main should be stable.

**[complytime-providers@main mutable ref]** → The CI workflow checks out
`complytime/complytime-providers@main` by branch name, not by SHA. If a breaking
change is merged to providers main without a corresponding complyctl change, the
cross-repo test will fail on complyctl PRs until the breakage is resolved. Accepted
risk: SHA-pinning the providers checkout would require a manual update process for
every providers merge, adding maintenance burden that outweighs the stability benefit
for a non-release workflow. The expected failure mode (test fails, team investigates)
is acceptable for PR-time integration testing.

## Open Questions

- Should the cross-repo integration test block PR merge (required status check) or run
  as an informational check? Recommended: required, to enforce the integration contract.
- When should the mirrored workflow be added to `complytime-providers`? It should follow
  immediately after this change is merged so the integration boundary is validated from
  both sides.
- When should the `org-infra/reusable_compliance.yml` stale-path issue be addressed?
  This change exposes the gap but does not fix it. A follow-up OpenSpec change should
  update the reusable workflow to check out complytime-providers for the provider binary
  and granular policies.
