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

# Behavioral assessment (EvaluationLog + SARIF reports)
make test-behavioral
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
|----------|------|---------|
| CI | `ci_checks.yml` | Standardized CI via org-infra reusable workflow |
| Unit Test | `unit_test.yml` | Unit tests + buf lint |
| E2E Test | `e2e_test.yml` | End-to-end tests with mock registry |
| Integration Test | `integration_test.yml` | Shell-based integration tests |
| CRAP Load | `ci_crapload.yml` | CRAP analysis on PRs (reusable from org-infra) |
| CRAP Main | `ci_crapload_main.yml` | CRAP baseline updates on main |
| Security | `ci_security.yml` | Security scanning |
| Compliance | `ci_compliance.yml` | Compliance checks |
| Dependencies | `ci_dependencies.yml` | Dependency management |
| SonarCloud | `ci_sonarcloud.yml` | Code quality analysis |
| Behavioral | `behavioral_assessment.yml` | Behavioral assessment reports |
| Scheduled | `ci_scheduled.yml` | Daily OSV-Scanner and Scorecards |
| Release | `release.yml` | Release automation |

## Project Structure

```text
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
├── behavioral/      # behavioral test scenarios
├── e2e/             # E2E tests (build-tag gated: -tags=e2e)
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

| Command | When | Scope |
|---------|------|-------|
| `/review-council` | Pre-PR (local) | 5+ Divisor agents |
| `/review-pr [N]` | Post-PR (GitHub) | Single agent, CI analysis |

## Specification Workflow

All non-trivial changes MUST be preceded by a spec workflow.

| Tier | Tool | When | Artifacts |
|------|------|------|-----------|
| Strategic | Speckit | >= 3 stories, cross-repo | `specs/NNN-*/` |
| Tactical | OpenSpec | < 3 stories, single-repo | `openspec/changes/*/` |

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
  `~/.complytime/policies/` with digest-based incremental sync.
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
- 005-rpm-packaging-ci: Added Go 1.25 + go-rpm-macros, Packit, Testing Farm (TMT/FMF)

- 004-providers-repository-split: Providers (openscap, ampel) migrated to `complytime-providers`; `pkg/plugin/` renamed to `pkg/provider/`; all "plugin" terminology updated to "provider"
