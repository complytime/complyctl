# complyctl Development Guidelines

## Overview

complyctl is a lightweight compliance runtime CLI that pulls
[Gemara](https://gemara.openssf.org/) policies from an OCI
registry and executes scans via providers, producing compliance
reports in multiple formats (EvaluationLog, OSCAL, SARIF,
Markdown).

- **Type**: CLI tool (Cobra-based)
- **Language**: Go 1.25 (`github.com/complytime/complyctl`)
- **License**: Apache-2.0
- **Mission**: Automate compliance policy evaluation through
  a provider-extensible, OCI-native workflow

## Build & Test Commands

### Build

```bash
make build                # compile complyctl to ./bin/
make build-test-provider  # build test provider for E2E tests
make clean                # remove build artifacts
make vendor               # go mod tidy + verify + vendor
```

### Test

```bash
# Unit tests (race detector + coverage)
make test-unit
# → go test -race -v -coverprofile=coverage.out ./...

# E2E tests (requires build + test provider)
make test-e2e
# → go test -tags=e2e -mod=vendor ./tests/e2e/... -v -count=1 -timeout 120s

# Integration tests (shell-based, requires build + test provider)
make test-integration
# → ./tests/integration_test.sh

# Cross-repo integration tests (requires PROVIDERS_BIN_DIR and GITHUB_TOKEN)
make test-cross-repo PROVIDERS_BIN_DIR=/path/to/providers/bin
# → timeout 120 ./tests/cross-repo/cross_repo_integration_test.sh

# Behavioral assessment (EvaluationLog + SARIF reports)
make test-behavioral

# Devcontainer smoke test (verifies Containerfile builds)
make test-devcontainer
# → podman build -t complyctl-devcontainer-test .devcontainer/

# Schema validation (validates EvaluationLog YAML against Gemara CUE schema, requires cue)
make test-schema-validation
# → ./tests/schema-validation/validate.sh .complytime/scan

# Acceptance tests (container-based, requires podman-compose or docker compose)
make test-acceptance
# → compose up --profile lifecycle (zot + seed + SUT containers)
```

### Lint & Format

```bash
make lint       # golangci-lint run ./... + goimports check
make format     # go fmt ./...
make vet        # go vet ./...
make sanity     # vendor + format + vet + git diff --exit-code
make proto      # buf generate (protobuf codegen)
```

### CRAP Load Monitoring

```bash
make crapload           # run CRAP/GazeCRAP analysis (human-readable)
make crapload-baseline  # generate baseline thresholds in .gaze/baseline.json
make crapload-check     # check for CRAP regressions against baseline
```

### CI Workflow Structure

| Workflow | File | Purpose |
| ---------- | ------ | --------- |
| CI | `ci_checks.yml` | Standardized CI via org-infra reusable workflow |
| Unit Test | `unit_test.yml` | Unit tests + buf lint |
| E2E Test | `e2e_test.yml` | End-to-end tests with mock registry |
| Integration Test | `integration_test.yml` | Shell-based integration tests |
| Cross-Repo Integration | `ci_cross_repo_integration.yml` | Cross-repo integration tests with complytime-providers + EvaluationLog schema validation |
| CRAP Load | `ci_crapload.yml` | CRAP analysis on PRs (reusable from org-infra) |
| Security | `ci_security.yml` | Security scanning |
| Compliance | `ci_compliance.yml` | Compliance checks |
| Dependencies | `ci_dependencies.yml` | Dependency management |
| SonarCloud | `ci_sonarcloud.yml` | Code quality analysis |
| Behavioral | `behavioral_assessment.yml` | Behavioral assessment reports |
| Scheduled | `ci_scheduled.yml` | Daily OSV-Scanner and Scorecards |
| Acceptance Test | `acceptance_test.yml` | Container-based acceptance tests with real OCI registry |
| Release | `release.yml` | Release automation (reusable preflight + GoReleaser from org-infra) |

## Project Structure

```text
.devcontainer/       # devcontainer config for testing environment
├── Containerfile    # Fedora base image definition
├── devcontainer.json # devcontainer standard configuration
└── scripts/
    └── post-create.sh # setup automation script
api/                 # protobuf definitions (provider gRPC API)
cmd/
├── complyctl/       # CLI entrypoint (main.go)
├── behavioral-report/ # behavioral assessment report generator
├── mock-oci-registry/ # mock OCI registry for testing
└── test-provider/   # test provider for E2E tests
docs/                # user documentation (install, quick start, style guide)
governance/          # compliance governance artifacts
├── capabilities/    # capability definitions
├── controls/        # control catalogs (complytime-controls.yaml)
├── policies/        # policy definitions
└── threats/         # threat models
internal/
├── cache/           # OCI layout cache management
├── complytime/      # workspace config and export logic
├── doctor/          # pre-flight diagnostics
├── output/          # report formatters (OSCAL, SARIF, Markdown)
├── policy/          # policy resolution and assessment
├── registry/        # OCI registry client
├── terminal/        # TUI components (spinner, bubbles)
└── version/         # version info (injected via ldflags)
openspec/            # OpenSpec tactical specification schemas
pkg/
├── log/             # structured logging
└── provider/        # provider discovery and gRPC lifecycle
plans/               # feature planning artifacts
scripts/             # maintenance scripts (SPDX checks, workflow setup)
specs/               # Speckit strategic specifications (NNN-*/  format)
tests/
├── acceptance/      # Container-based acceptance tests (compose + zot)
├── behavioral/      # behavioral test scenarios
├── cross-repo/      # cross-repo integration tests (complyctl + ampel + opa providers)
├── e2e/             # E2E tests (build-tag gated: -tags=e2e)
├── schema-validation/  # CUE schema validation of EvaluationLog output (validate.sh)
└── integration_test.sh  # shell-based integration test
vendor/              # vendored dependencies
```

## Coding Conventions

### Go Standards

- **Formatting**: `gofmt` and `goimports` enforced via
  golangci-lint (`formatters.enable: [goimports]`)
- **Linters**: golangci-lint v2 with `default: standard` plus
  `gosec` for security checks (see `.golangci.yml`)
- **Import grouping**: stdlib, then external, then internal
  (enforced by goimports)
- **Error handling**: Always check and handle errors; return
  to caller when unresolvable. Wrap with `fmt.Errorf("context: %w", err)`
- **File naming**: lowercase with underscores (`my_file.go`)
- **Package names**: short, concise, lowercase, no underscores
- **File headers**: Every `.go` file MUST start with
  `// SPDX-License-Identifier: Apache-2.0`
- **Line length**: 99 characters max unless exceeding improves
  readability

### Container Image Conventions

- **Base images**: Red Hat UBI (`ubi10/ubi-minimal`) MUST
  NOT be pinned by digest hash. Use `:latest` to track
  current errata and security patches. Dismiss Scorecard
  `Pinned-Dependencies` alerts for UBI images — this is
  an intentional tradeoff favoring automatic errata
  coverage over reproducibility.
- **Trivy suppressions**: Use `# trivy:ignore:DSXXX`
  comments inline above the `FROM` line for accepted
  findings on test/ephemeral containers.

### Spec Writing Conventions

- Use RFC 2119 language (MUST/SHOULD/MAY) for requirements
- Given/When/Then format for scenarios
- FR-NNN numbering for functional requirements
- Line length < 72 for spec prose

## Testing Conventions

- **Framework**: stdlib `testing` + `testify` (assert/require)
- **Test naming**: `TestFunctionName_Description` for unit tests
- **Assertion style**: `require` for fatal preconditions,
  `assert` for non-fatal checks
- **Mocking**: Interface-based mock structs defined alongside
  tests (no codegen mocking framework)
- **Filesystem isolation**: `t.TempDir()` for tests that touch
  the filesystem
- **E2E gating**: E2E tests use build tag `//go:build e2e` and
  run via `make test-e2e` (not included in `make test-unit`)
- **Coverage**: Generated by `make test-unit` into `coverage.out`
- **CRAP monitoring**: Functions tracked via gaze; new functions
  MUST NOT exceed CRAP threshold of 30

## Behavioral Rules

These rules are non-negotiable. Violations are CRITICAL severity.

- **Gatekeeping**: MUST NOT modify quality/governance gates
  (coverage thresholds, CRAP scores, severity definitions,
  CI flags, agent settings, constitution MUST rules, review
  limits, workflow markers). Stop and report instead.
- **Phase boundaries**: MUST NOT cross workflow phase boundaries.
  Spec phases: spec artifacts only. Implement: source code.
  Review: fixes only. Violation = process error, stop immediately.
- **CI parity**: MUST replicate CI checks locally before marking
  tasks complete. Derive commands from `.github/workflows/`.
- **Review council**: MUST run `/review-council` before PR
  submission. Resolve all REQUEST CHANGES. No code changes
  between APPROVE and PR. Exempt: constitution amendments,
  docs-only, emergency hotfixes.
- **Branch protection**: MUST NOT commit directly to `main`.
  All changes via feature branches and PRs.
- **Documentation gate**: Before marking a task complete,
  assess documentation impact: `CHANGELOG.md` for change
  entries, `AGENTS.md` for structural updates (project
  structure, conventions, build commands), `README.md` for
  description changes.
- **Website gate**: MUST file `unbound-force/website` issue
  for user-facing changes before PR merge. Exempt: internal
  refactoring, test-only, CI-only, spec artifacts.
- **Zero-waste**: No orphaned specs, unused standards, or
  aspirational documents that do not map to actionable work.

### PR Review Commands

| Command           | When             | Scope                     |
|-------------------|------------------|---------------------------|
| `/review-council` | Pre-PR (local)   | 5+ Divisor agents         |
| `/review-pr [N]`  | Post-PR (GitHub) | Single agent, CI analysis |

## Specification Workflow

All non-trivial changes MUST be preceded by a spec workflow.

| Tier       | Tool     | When                      | Artifacts              |
|------------|----------|---------------------------|------------------------|
| Strategic  | Speckit  | >= 3 stories, cross-repo  | `specs/NNN-*/`         |
| Tactical   | OpenSpec | < 3 stories, single-repo  | `openspec/changes/*/`  |

Pipeline: `constitution → specify → clarify → plan → tasks →
analyze → checklist → implement`

**Ordering**: Constitution before specs. Spec before plan. Plan
before tasks. Tasks before implementation. Spec artifacts MUST
be committed/pushed before implementation begins.

**Branches**: Speckit: `NNN-<name>`. OpenSpec: `opsx/<name>`.

**Task bookkeeping**: Mark checkboxes `[x]` immediately on
completion. `[P]` marks parallel-eligible tasks.

**When in doubt**: Start with OpenSpec. Escalate to Speckit if
scope grows beyond 3 stories or crosses repo boundaries.

**What requires a spec**: New features, refactoring that changes
signatures, test additions across multiple functions, agent
changes, CI changes, data model changes.

**Exempt**: Constitution amendments, typo fixes, emergency
hotfixes (retroactively documented).

## Convention Packs

This repository uses convention packs scaffolded by
unbound-force. Agents MUST read the applicable pack(s)
before writing or reviewing code.

- `.opencode/uf/packs/default.md`
- `.opencode/uf/packs/default-custom.md`
- `.opencode/uf/packs/severity.md`
- `.opencode/uf/packs/content.md`
- `.opencode/uf/packs/content-custom.md`
- `.opencode/uf/packs/go.md`
- `.opencode/uf/packs/go-custom.md`

## Architecture

complyctl follows a **Cobra CLI delegation** pattern where each
subcommand (`init`, `get`, `scan`, etc.) is a standalone Cobra
command wired in `cmd/complyctl/`. Core logic lives in `internal/`
packages organized by domain responsibility.

- **Provider model**: Providers are standalone executables
  discovered by naming convention (`complyctl-provider-*`) and
  accessed via **HashiCorp go-plugin** over **gRPC** (protobuf
  definitions in `api/plugin/`). Providers implement `Describe`,
  `Generate`, and `Scan` RPCs.
- **OCI-native caching**: Policies are fetched from OCI registries
  using `oras-go` and stored as local OCI Layouts under
  `~/.cache/complytime/policies/` (XDG cache) with digest-based
  incremental sync. Persistent data (`state.json`, providers) lives
  under `~/.local/share/complytime/` (XDG data).
- **Policy resolution**: The `internal/policy/` package resolves
  Gemara policy dependency graphs, extracts assessment configs,
  and applies parameter overrides from `complytime.yaml`.
- **Output formatters**: `internal/output/` produces EvaluationLog,
  OSCAL assessment-results, SARIF, and Markdown reports via
  strategy-pattern formatters.
- **TUI**: Interactive elements use `charmbracelet/bubbletea`
  and `lipgloss` for terminal rendering.

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->

## Recent Changes
- custom-trusted-root: `VerificationConfig` gains `TrustedRoot string` field with `yaml:"trusted_root,omitempty"` tag in `internal/complytime/config.go`; `ValidateVerificationConfig()` enforces mutual exclusivity with `key` (error: "trusted_root cannot be used with key-based verification"), requires `issuer` + `identity` (error: "trusted_root requires issuer and identity for keyless verification"), and validates file existence via `filepath.Clean` + `os.Stat`; `NewKeylessVerifier()` in `internal/cache/verify.go` gains `trustedRootPath string` parameter — when non-empty, calls `root.NewTrustedRootFromPath(trustedRootPath)` instead of TUF fetch, wrapping errors with `"failed to load trusted root from %s: %w"`; `buildVerifierFromConfig()` in `cmd/complyctl/cli/get.go` passes `cfg.TrustedRoot` to `NewKeylessVerifier()`; THR02.MIT05 added to threat model for user-supplied trust anchor without TUF integrity protection; fixes #768
- fix-doctor-target-variable-oneofs: `DescribeResponse` proto gains `optional_target_variable_groups` field 7 (pipe-delimited one-of groups, e.g. `"url|input_path"`); `ProviderHealth` struct gains `OptionalTargetVariableGroups []string`; `CheckVariables()` validates optional groups as at-least-one-present per target instead of requiring all; resolves contradiction where `doctor` required both `url` and `input_path` while `scan` rejected configs with both set; OPA provider update tracked in complytime-providers#145; fixes #759
- registry-host-verification-fix: **BREAKING** -- `BuildLookupRef()` gains `registryHost` parameter prepended to OCI references for signature verification; `Sync`/`ComplypackSync` structs gain `registryHost` field populated from `ref.Registry` at construction; `NewSync`/`NewComplypackSync` constructors gain `registryHost string` parameter; all `VerifyFunc` invocations now receive host-qualified `registryRef` resolving against the configured registry instead of Docker Hub; fail-closed when `registryHost` is empty with verifier configured (returns error containing "registry host"); defense-in-depth with `ValidateOCIRef` config-time check; THR02 mitigations updated (MIT02 refined, MIT04 added for fail-closed); fixes #767
- digest-format-validation: `ValidateDigest()` in `internal/cache/state.go` validates OCI digest format (`algorithm:hex`) using `opencontainers/go-digest`; `UpdatePolicyStateWithVerification` and `UpdateComplypackStateWithVerification` now return `error`, rejecting malformed digests at write time; `LoadState` warns and excludes entries with malformed digests while preserving valid and empty-digest entries; `internal/cache/cachetest/digests.go` provides canonical test digest constants; defense-in-depth per NIST 800-53 SI-10 (#677)
- xdg-base-directory: **BREAKING** — User-scoped paths now follow the XDG Base Directory Specification; cache (policies, complypacks) moves from `~/.complytime/` to `~/.cache/complytime/` (`$XDG_CACHE_HOME`); data (providers, `state.json`) moves to `~/.local/share/complytime/` (`$XDG_DATA_HOME`); `ResolveCacheDir()`/`ResolveDataDir()`/`ResolveProviderDir()` in `internal/complytime/consts.go` provide cross-platform resolution with env var overrides; `CheckLegacyDir()` in `internal/complytime/legacy.go` prints deprecation warning with manual migration guidance when legacy `~/.complytime/` exists; workspace-local `.complytime/` unchanged; implements ADR-0016 and aligns with complypack#127 (#734)
- adopt-org-infra-release-workflows: `release.yml` replaced inline preflight + GoReleaser jobs (~184 lines) with two org-infra reusable workflow calls (`reusable_release_preflight.yml` + `reusable_release_goreleaser.yml`); smart re-run detection replaces tag uniqueness check that blocked re-runs; semver-aware Python comparator replaces `sort -V` pre-release ordering inversion; file-based CI check discovery via `ci_checks` override (`["unit-test", "e2e-test", "integration-test"]`); `docs/RELEASE_PROCESS.md` restructured to reference org-wide release process and document automated tag creation; fixes #654 (re-run blocking + sort -V), #655 (docs, hardening, recovery follow-ups) (#699)
- doctor-grouped-output: `complyctl doctor` output redesigned with grouped sections and nested sub-results; `CheckResult` gains `Children []CheckResult` field, `Label string` field, and `CheckGroup` type with `GroupOrder()` function for section ordering; `Run()` assembles result tree via `attachByEvaluatorID()` and `attachByPolicyID()` helpers; `CheckPolicyVersions` emits per-policy `policy/<eid>` warnings for unreachable registries instead of `registry/<reg>` with silent skip; `CheckComplypacks` lists each complypack individually by evaluator-id; `printDiagnostics` in `cmd/complyctl/cli/doctor.go` rewritten as nested tree walk with summary counts at depth 0–1 (excludes verbose detail grandchildren); string-parsing helpers (`partitionVariables`, `buildChildMap`, `filterParents`, `extractEvaluatorID`) removed (#692)
- doctor-machine-readable-output: `complyctl doctor` gains `--format text|json` flag; `resolveFormat()` auto-selects `text` when `NO_COLOR` is set; three renderers: `printDiagnosticsHuman` (emoji), `printDiagnosticsText` (plain label), `printDiagnosticsJSON` (structured JSON with `checks`, `summary`, `blocking_failure`); `statusLabel()` maps `CheckStatus` to grep-stable `[PASS]`/`[FAIL]`/`[WARN]`; `OutputFormatText`/`OutputFormatJSON` constants in `internal/complytime/consts.go` (#611)
- doctor-human-format-refactor: `complyctl doctor --format` gains explicit `human` option (three-value set: `human|text|json`); `OutputFormatHuman` constant in `internal/complytime/consts.go`; `summarizeResults()` extracts shared counting logic across renderers into `resultSummary` struct; `statusEmoji()` centralises emoji mapping; `printDiagnosticsTo()` rejects unrecognised format values instead of falling back to human; flag help text and shell completion updated (#744)
- ci-step-summary-report: Emoji status indicators added to `--format pretty` markdown report; `resultEmoji()` helper maps `gemara.Result` to `complytime.Status*` constants; `writeControlsTable()` prefixes Result column with emoji for control and requirement rows; `writeSummary()` prefixes counts table headers with emoji; `writeFindings()` prefixes group headers with emoji; report now directly usable as GitHub Actions Step Summary via `cat report-*.md >> $GITHUB_STEP_SUMMARY` (#697)
- complypack-cache-versioning: `COMPLYTIME_CACHE_VERSIONS` env var configures retention count (default 1) for complypack cache versions per evaluator-id; `NewComplypackCache()` gains `*State` parameter for state-driven lookup and timestamp-based eviction ordering; `evictOldVersions()` becomes retention-count-aware (orphaned dirs first, then oldest by `LastUpdated`); `LookupByEvaluatorID()` resolves from state.json with directory-scan fallback; `SyncComplypack()` checks local cache before remote fetch with re-verification when verifier is configured, returns `(true, nil)` for cache hits to trigger generation invalidation; `EvaluatorIDToVersion()` reverse lookup on `*State`; `CacheRetentionCount()` in `internal/cache/retention.go`; `CheckComplypacks()` extended with `walkCacheSize()` and `findOrphanedVersions()` for cache health reporting; `complyctl doctor` reports cache size and orphaned/untracked versions (#676)
- bundle-metadata-cli: `complyctl list` gains EVALUATOR and CONTROLS columns sourced from `PolicyState` metadata in `state.json`; `complyctl get` prints post-sync summary to stderr after fresh fetch showing policy title, evaluator, control count, and assessment count; `PolicyState` gains `PolicyTitle`/`PolicyEvaluator`/`ControlCount`/`AssessmentCount` fields populated at sync time via `SetPolicyMetadata()`; `policy.Resolver.ExtractPolicyMetadata()` extracts display metadata without building full `DependencyGraph`; `policyLayerResult.Title` added to `parsePolicyLayer()` output; upgrade backfill for pre-existing caches without metadata; `formatPolicySummary()` helper for testable stderr output (#506)
- per-entry-verification: Per-entry `verification:` and `skip_verify:` fields on `PolicyEntry` in `internal/complytime/config.go` for policy-level signature verification overrides; `resolveVerifier()` in `cmd/complyctl/cli/get.go` with verifier cache and resolution priority chain (entry → workspace → none); error collection in `syncAllPolicies()`/`syncAllComplypacks()` via `errors.Join()` for partial-failure resilience; cross-group error collection in `syncAll()` so policy failures do not block complypack sync; per-entry WARNING to stderr on sync failure for real-time feedback; `validateEntries()` extended for per-entry verification validation and mutual exclusivity of `verification:` and `skip_verify:` (#680)
- acceptance-tests: Container-based acceptance test stack (`tests/acceptance/`, `make test-acceptance`, `acceptance_test.yml`); zot + seed + sut compose architecture validates real OCI registry interop; `build-acceptance-test` target compiles acceptance binary; `test-acceptance-clean` tears down containers and volumes; `SaveState` atomic write refactor in `internal/cache/state.go`
- debug-visible-output: `--debug` flag now tees all log messages to stderr via `teeWriter` in `pkg/log/log.go`; `enableDebug()` in `cmd/complyctl/cli/root.go` reconstructs logger with `NewTeeWriter(stderr, logFile)` and forces `termenv.ANSI256` color profile when stderr is a TTY (respects `NO_COLOR`); `Debug log: <path>` hint printed to stderr after workspace resolution; flag description updated to `"output debug logs to stderr and log file"`; `SetColorProfile()` method added to `CharmHclog` adapter (#614)
- sigstore-verification: `complyctl get` verifies OCI artifact signatures via `sigstore-go` when `verification:` configured in `complytime.yaml`; keyless (OIDC issuer + identity) and keyed (public key) modes; pre-copy verification via registry API before `oras.Copy()`; `--skip-verify` flag; `VerificationConfig` in `WorkspaceConfig`; `VerifyFunc`/`NewKeylessVerifier`/`NewKeyedVerifier` in `internal/cache/verify.go`; `PolicyState` gains `Verified`/`SignerIdentity`/`Issuer`/`VerifiedAt` fields; `SyncOption`/`WithVerifier()` functional options on `Sync`/`ComplypackSync`; `complyctl list` VERIFIED column; `complyctl doctor` `CheckVerification` diagnostic; `sigstore-go` v1.2.1 + `go-containerregistry` dependencies added
- markdown-report-redesign: `--format pretty` markdown report redesigned with summary metadata table, pass rate, grouped controls table (control + requirement rows with message column), findings section grouped by result type with recommendation and collapsible evidence per finding, evaluation log in collapsible `<details>`; `Evidence` message and `recommendation` field added to proto `AssessmentLog`; `provider.Evidence` type added to `pkg/provider/client.go`; evaluator populates `gemara.Evidence` and `Recommendation` on assessment logs; `internal/output/markdown.go` rewritten with `writeSummary`/`writeControlsTable`/`writeFindings`/`writeEvaluationLog` methods
- scan-show-passing: `complyctl scan --show-passing` flag (default `true`) and `COMPLYTIME_SHOW_PASSING` env var to include passing controls in terminal summary table; `FormatScanSummary()` gains `showPassing bool` parameter; `nonPassingEntry` renamed to `summaryEntry`; `nonPassingSortPriority()` renamed to `sortPriority()` with `gemara.Passed` at priority 6; supersedes FR-037 non-passing-only principle per user feedback (#438)
- workspace-configuration: `--workspace` flag and `COMPLYTIME_WORKSPACE` env var for workspace directory resolution; config file moved to `.complytime/complytime.yaml` with legacy fallback; `NewWorkspace(baseDir string)` signature change; all output paths relative to resolved workspace
- fix-resolve-plan-ids: `complyctl scan` resolves assessment plan IDs to requirement and control IDs in scan reports via `extractPlanToReqMap()`/`resolveAssessmentIDs()` in `cmd/complyctl/cli/scan.go`
- complypack-pull: `complyctl get` fetches complypack OCI artifacts when `complypacks:` configured in `complytime.yaml`; `complyctl providers` gains COMPLYPACK column; `complyctl doctor` gains complypack cache check; `GenerateRequest.complypack_content_path` proto field added; `internal/cache/complypack*.go` cache/sync/source modules; `internal/cache/state.go` extended with complypack state tracking
- scan-target-arg: `complyctl scan [target]` positional argument for scoping scans to a single target with automatic policy-id inference
- cross-repo-integration-tests: Cross-repo integration test infrastructure (`tests/cross-repo/`, `make test-cross-repo`, `ci_cross_repo_integration.yml`) validating complyctl + Ampel provider pipeline
- opa-devcontainer-content: Added OPA provider test content to devcontainer; OPA Gemara testdata (catalog + policy with `executor.id: opa`) seeded in mock registry; Rego policies + `complytime-mapping.json` for complypack; `test-opa-bp` policy-id and `test-k8s-deployment` target in `complytime.yaml`; `docs/TESTING_ENVIRONMENT.md` OPA command reference
- devcontainer-bundle-cache: Mock OCI registry gains `seedFromDirectory()` to serve mounted Gemara YAML policies from `/bundles/` (or `COMPLYCTL_BUNDLES_DIR`); post-create script adds policy entries to `complytime.yaml`; `docs/TESTING_ENVIRONMENT.md` Private Bundles section added
- dev-testing-environment: Added `.devcontainer/` with Fedora-based devcontainer for interactive CLI testing; `docs/TESTING_ENVIRONMENT.md` documentation; `make test-devcontainer` CI smoke target; post-create script with GITHUB_TOKEN least-privilege handling
- scan-error-exit-codes: `complyctl scan` exits non-zero on operational errors; `ScanResponse.errors` proto field added; `ScanResult`/`RouteScanResult()` in `pkg/provider/manager.go`; `FormatOperationalWarnings` in `internal/output/scan_summary.go`; `processScanOutput`/`checkOperationalErrors`/`reportOperationalWarnings` in `cmd/complyctl/cli/scan.go`
- 005-bundle-resolver-alignment: Policy resolver supports both split-layer and Gemara bundle-format OCI artifacts; `internal/policy/loader.go` gained `LoadBundleFiles()`, `DetectManifestShape()`, `resolveManifest()`; `PolicyLoader` interface extended with bundle methods; `MockBundlePolicySource` added to `internal/cache/cachetest/`
- 005-rpm-packaging-ci: Added Go 1.25 + go-rpm-macros, Packit, Testing Farm (TMT/FMF)

- 004-providers-repository-split: Providers (openscap, ampel) migrated to `complytime-providers`; `pkg/plugin/` renamed to `pkg/provider/`; all "plugin" terminology updated to "provider"

- remove-exporter-infrastructure: **BREAKING** — Removed collector export infrastructure (`COMPLYTIME_EXPORT_ENABLED`, `collector:` config block, Export RPC). This was speculative infrastructure added before backend design was finalized. Export functionality will be redesigned and reintroduced when the backend shape is known. (#606)

- policy-verification-pipeline: `complyctl get` emits NOTE for unverified policies and complypacks on fresh fetches; `complyctl list` gains DIGEST column; `SyncPolicy` returns `(bool, error)` for fresh-fetch gating; THR02 mitigations MIT01 (warning) and MIT03 (digest visibility) added to threat model
