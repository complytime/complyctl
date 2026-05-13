<!-- spec-review: passed -->

## 1. Test Gemara Content (mock registry testdata)

- [x] 1.1 Create `cmd/mock-oci-registry/testdata/test-branch-protection-catalog.yaml`
  with one control (`force-push-protection`) and assessment requirement
  (`block-force-push`)
- [x] 1.2 Create `cmd/mock-oci-registry/testdata/test-branch-protection-policy.yaml`
  with one assessment plan (`block-force-push`), executor `ampel`, frequency
  `on-demand`
- [x] 1.3 Add `policies/test-branch-protection` seed entry to `seedDefaults()` in
  `cmd/mock-oci-registry/main.go` using the two new testdata files

## 2. Test Fixtures (cross-repo testdata)

- [x] 2.1 Create `tests/cross-repo/testdata/granular-policies/block-force-push.json`
  with `id: "block-force-push"`, one tenet, GitHub branch-rules predicate type, and
  a simple force-push CEL expression
- [x] 2.2 Create `tests/cross-repo/testdata/complytime.yaml` pointing at
  `http://localhost:8765/policies/test-branch-protection` with policy ID `test-ampel-bp`
  and a target with `url: https://github.com/complytime/complyctl` and
  `specs: builtin:github/branch-rules.yaml` both under `targets[].variables`

## 3. Integration Test Script

- [x] 3.1 Create `tests/cross-repo/cross_repo_integration_test.sh` with setup and
  teardown functions: isolated temp `HOME` and `WORK_DIR`, provider binary install to
  `~/.complytime/providers/`, mock registry start with readiness poll, cleanup trap.
  Derive complyctl and mock-oci-registry binary paths from `REPO_ROOT` (script location),
  not from a separate env var.
- [x] 3.2 Implement `test_get` function: runs `complyctl get`, asserts OCI layout and
  `state.json` exist
- [x] 3.3 Implement `test_generate` function: runs `complyctl generate --policy-id
  test-ampel-bp`, asserts `complytime-ampel-policy.json` exists in
  `.complytime/ampel/policy/`
- [x] 3.4 Implement `test_scan` function: runs `complyctl scan --policy-id test-ampel-bp`,
  asserts snappy and ampel attestation files exist in `.complytime/ampel/results/`,
  asserts ampel result JSON contains `block-force-push` requirement ID
- [x] 3.5 Add assertion helpers (`assert_contains`, `assert_file_exists`,
  `assert_json_contains`) and pass/fail summary consistent with existing
  `tests/integration_test.sh` style
- [x] 3.6 Validate that `PROVIDERS_BIN_DIR` is set and `complyctl-provider-ampel`
  exists there; exit with a clear error message if not. Also validate that `GITHUB_TOKEN`
  is set and the complyctl and mock-oci-registry binaries exist at their derived paths.

## 4. Makefile Target

- [x] 4.1 Add `test-cross-repo` target to `Makefile` that depends on `build`, requires
  `PROVIDERS_BIN_DIR` to be set (error if empty), and runs
  `tests/cross-repo/cross_repo_integration_test.sh`

## 5. complyctl CI Workflow

- [x] 5.1 Create `.github/workflows/ci_cross_repo_integration.yml` that triggers on
  `pull_request` to `main` (opened and synchronize events) and on `push` to `main`
- [x] 5.2 Add step to check out complyctl (current ref) and build with `make build`
- [x] 5.3 Add step to check out `complytime/complytime-providers@main` into
  `_providers/` and build with `make build` in that directory
- [x] 5.4 Add steps to install `snappy` and `ampel` via
  `carabiner-dev/actions/install/snappy` and `carabiner-dev/actions/install/ampel`
  (pin to SHA)
- [x] 5.5 Add step to run `tests/cross-repo/cross_repo_integration_test.sh` with
  `PROVIDERS_BIN_DIR: ${{ github.workspace }}/_providers/bin` and
  `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}`
- [x] 5.6 Set minimum permissions: `contents: read` at workflow level;
  no additional permissions needed

## 6. Verification

- [x] 6.1 Run `make build` in complyctl and confirm the mock registry binary includes
  the new `policies/test-branch-protection` endpoint
- [x] 6.2 Run `make test-cross-repo PROVIDERS_BIN_DIR=<path>` locally with a built
  `complyctl-provider-ampel` binary and confirm all test functions pass
- [x] 6.3 Confirm `make test-unit` and `make test-integration` still pass (no
  regressions from mock registry change)
- [ ] 6.4 Open a draft PR in complyctl and confirm the `ci_cross_repo_integration`
  workflow triggers and passes
