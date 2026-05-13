## Why

After the provider split (spec 004), `complyctl` and `complytime-providers` are
developed independently, but the two must interoperate correctly at runtime: complyctl
discovers, launches, and communicates with provider binaries over gRPC, while providers
consume the `pkg/provider` SDK and respond to Describe/Generate/Scan RPCs. Neither
repository currently has tests that verify this integration boundary, meaning a breaking
change in either repo can go undetected until a user encounters it. This change
establishes the complyctl side of the cross-repo integration test infrastructure: the
test script, test fixtures, mock registry content, and the CI workflow that validates
every complyctl PR against the Ampel provider from `complytime-providers@main`.

The corresponding CI workflow in `complytime-providers` is tracked separately and will
be implemented once this change is merged.

## What Changes

- **New test content**: Minimal Gemara catalog and policy (1 control, 1 assessment plan,
  executor `ampel`) added to `cmd/mock-oci-registry/testdata/` and seeded in the mock
  registry under `policies/test-branch-protection`.
- **New test fixture**: A single Ampel granular policy JSON file (`block-force-push.json`)
  with one tenet and GitHub-only CEL expression, placed in
  `tests/cross-repo/testdata/granular-policies/`.
- **New test workspace config**: `tests/cross-repo/testdata/complytime.yaml` pointing at
  the mock registry and the `complytime/complyctl` GitHub repository as scan target.
- **New integration test script**: `tests/cross-repo/cross_repo_integration_test.sh`
  that orchestrates the full `get` / `generate` / `scan` pipeline using the real
  `complyctl-provider-ampel` binary and real `snappy` / `ampel` CLI tools. This script
  is designed to be consumed by the complytime-providers CI workflow as well.
- **New Makefile target**: `test-cross-repo` that builds complyctl and runs the script.
- **New CI workflow in complyctl**: `.github/workflows/ci_cross_repo_integration.yml`
  triggers on PRs to main, builds complyctl from the PR branch, checks out
  `complytime-providers@main`, builds the real provider binary, installs snappy and
  ampel, and runs the integration test script.

## Capabilities

### New Capabilities

- `cross-repo-integration-test`: End-to-end functional integration test that validates
  complyctl and the Ampel provider binary interoperate correctly across repository
  boundaries, covering provider discovery, gRPC lifecycle, policy resolution, and scan
  output. The test script is designed to be reusable by `complytime-providers` CI.

### Modified Capabilities

## Impact

- **complyctl**: `cmd/mock-oci-registry/main.go` (new policy seed), new files under
  `tests/cross-repo/`, new Makefile target, new CI workflow.
- **complytime-providers**: not modified by this change. A follow-up change in that
  repository will add the mirrored CI workflow that checks out `complyctl@main` and
  runs this test script with the providers PR branch as the binary under test.
- **org-infra `reusable_compliance.yml`**: currently references stale paths
  (`bin/ampel-plugin`, `cmd/ampel-plugin/`) that were valid before the provider split.
  The new integration test design exposes this gap; a follow-up to update the reusable
  workflow is noted but out of scope for this change.
- **Dependencies**: `snappy` and `ampel` CLI tools installed via
  `carabiner-dev/actions/install/` composite actions; `GITHUB_TOKEN` (standard Actions
  token) for snappy to read branch protection rules from the public
  `complytime/complyctl` repository.

## Constitution Alignment

| Principle | Assessment |
|:----------|:-----------|
| I. Single Source of Truth | Test script, fixtures, and CI workflow live in one place (complyctl). No duplication with complytime-providers. |
| II. Simplicity & Isolation | Minimal test content (1 control, 1 rule). Each test function is isolated and independently verifiable. |
| III. Incremental Improvement | Scoped to complyctl side only. Providers side is a separate change. |
| IV. Readability First | Descriptive IDs (`block-force-push`, `force-push-protection`) throughout. Script follows existing `integration_test.sh` style. |
| V. Do Not Reinvent the Wheel | Uses established `carabiner-dev/actions` for tool installation; reuses existing mock registry infrastructure. |
| VI. Composability | Test script is reusable by `complytime-providers` CI. `make test-cross-repo` is a composable Makefile target. |
| VII. Convention Over Configuration | `PROVIDERS_BIN_DIR` is the only required input; all other paths derived from conventions. |
