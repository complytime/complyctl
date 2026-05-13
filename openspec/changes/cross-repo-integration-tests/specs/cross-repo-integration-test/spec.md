## ADDED Requirements

### Requirement: Cross-repo integration test script

The complyctl repository SHALL provide a shell-based integration test script at
`tests/cross-repo/cross_repo_integration_test.sh` that accepts `PROVIDERS_BIN_DIR`
and `GITHUB_TOKEN` as environment variables and validates the full complyctl + Ampel
provider pipeline end-to-end using real binaries and real external tools. The script
SHALL derive the complyctl binary path from its own location (`REPO_ROOT/bin/complyctl`)
without requiring a separate environment variable.

#### Scenario: Script runs with required environment variables

- **WHEN** `PROVIDERS_BIN_DIR` points to a directory containing `complyctl-provider-ampel`
  and `GITHUB_TOKEN` is set
- **THEN** the script completes successfully and reports all tests passed

#### Scenario: Script fails when provider binary is missing

- **WHEN** `PROVIDERS_BIN_DIR` points to a directory that does not contain
  `complyctl-provider-ampel`
- **THEN** the script exits with a non-zero status and reports an error

### Requirement: Minimal test Gemara content in mock registry

The mock OCI registry SHALL serve a minimal test policy under
`policies/test-branch-protection` containing exactly one control
(`force-push-protection`) and one assessment plan (`block-force-push`) with executor
`ampel`. This policy SHALL be seeded at startup alongside existing policies.

#### Scenario: Policy is discoverable via the mock registry API

- **WHEN** the mock OCI registry is running
- **THEN** `GET /v2/policies/test-branch-protection/manifests/latest` returns a valid
  OCI manifest with a catalog layer and a policy layer

#### Scenario: Policy contains the block-force-push assessment plan

- **WHEN** complyctl fetches `policies/test-branch-protection` and resolves the policy
- **THEN** exactly one `AssessmentConfiguration` with `RequirementID: "block-force-push"`
  is produced for the Ampel provider

### Requirement: Minimal test granular policy fixture

The complyctl repository SHALL provide a minimal Ampel granular policy JSON file at
`tests/cross-repo/testdata/granular-policies/block-force-push.json` with `id:
"block-force-push"`, one tenet, and the GitHub branch-rules predicate type. This file
SHALL be used by the integration test to populate `.complytime/ampel/granular-policies/`
before running `complyctl generate`.

#### Scenario: Granular policy matches the assessment plan requirement ID

- **WHEN** the Ampel provider's Generate RPC receives `RequirementID: "block-force-push"`
- **THEN** the provider loads `block-force-push.json`, matches it by ID, and writes a
  policy bundle containing that single policy to
  `.complytime/ampel/policy/complytime-ampel-policy.json`

### Requirement: complyctl get fetches the test policy

The integration test SHALL run `complyctl get` against the mock registry and verify
that the policy is cached locally as an OCI layout.

#### Scenario: Policy is fetched and cached

- **WHEN** `complyctl get` is executed with `test-ampel-bp` configured in
  `complytime.yaml`
- **THEN** an OCI layout directory and `state.json` tracking file are written under
  `~/.complytime/policies/`

### Requirement: complyctl generate produces the Ampel policy bundle

The integration test SHALL run `complyctl generate --policy-id test-ampel-bp` and
verify that the provider creates the merged policy bundle.

#### Scenario: Generate produces the policy bundle

- **WHEN** `complyctl generate --policy-id test-ampel-bp` is executed with the granular
  policy file pre-populated
- **THEN** `.complytime/ampel/policy/complytime-ampel-policy.json` is created and
  contains the `block-force-push` policy

### Requirement: complyctl scan completes and produces attestation output

The integration test SHALL run `complyctl scan --policy-id test-ampel-bp` and verify
that snappy and ampel run to completion and produce attestation files, regardless of
the PASS/FAIL scan result. `complyctl scan` SHALL exit 0 when the scan pipeline ran
successfully, even if individual policy controls evaluated to FAIL. It SHALL exit
non-zero only on tool-level errors (binary not found, gRPC failure, OCI fetch error,
etc.). The test SHALL assert exit 0 and treat any non-zero exit as a test failure.

#### Scenario: Scan produces snappy and ampel attestation files

- **WHEN** `complyctl scan --policy-id test-ampel-bp` is executed with
  `GITHUB_TOKEN` set and `url: https://github.com/complytime/complyctl` as the target
- **THEN** at least one `*-snappy.intoto.json` file and one `*-ampel.intoto.json` file
  exist in `.complytime/ampel/results/`

#### Scenario: Ampel result attestation references the expected requirement ID

- **WHEN** the ampel result attestation is parsed
- **THEN** `predicate.results` contains an entry with `policy.id: "block-force-push"`

### Requirement: complyctl CI triggers cross-repo integration on PRs

The complyctl repository SHALL have a GitHub Actions workflow at
`.github/workflows/ci_cross_repo_integration.yml` that triggers on pull requests
targeting `main`, builds complyctl from the PR branch, checks out
`complytime-providers@main`, builds the Ampel provider binary, installs `snappy` and
`ampel`, and runs the integration test script.

#### Scenario: Workflow runs on PR to main

- **WHEN** a pull request targeting `main` is opened or synchronized in complyctl
- **THEN** the `ci_cross_repo_integration` workflow is triggered and runs the full
  integration test

#### Scenario: complyctl binary comes from the PR branch

- **WHEN** the workflow runs
- **THEN** the `complyctl` binary and `mock-oci-registry` binary are built from the
  PR branch source, not from a release artifact

#### Scenario: Ampel provider binary comes from complytime-providers main

- **WHEN** the workflow runs
- **THEN** `complyctl-provider-ampel` is built from `complytime-providers@main`

### Requirement: Makefile test-cross-repo target

The complyctl Makefile SHALL provide a `test-cross-repo` target that builds complyctl
and runs the cross-repo integration test script. `PROVIDERS_BIN_DIR` SHALL be a
required input with no default, so the target fails explicitly when the provider
binary location is not specified.

#### Scenario: Target runs the integration test

- **WHEN** `make test-cross-repo PROVIDERS_BIN_DIR=/path/to/providers/bin` is executed
  with a built provider binary present
- **THEN** the integration test script runs and exits with status 0 on success
