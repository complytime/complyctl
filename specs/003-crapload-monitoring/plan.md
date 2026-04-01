# Implementation Plan: CRAP Load Monitoring

**Branch**: `003-crapload-monitoring` | **Date**: 2026-03-10 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/003-crapload-monitoring/spec.md`

## Summary

Implement continuous CRAP (Change Risk Anti-Pattern) and GazeCRAP monitoring for the complyctl repository. This adds a reusable GitHub Actions workflow that analyses every PR for complexity/coverage regressions using the Gaze tool, posts a PR comment with results, and enforces baseline thresholds. An event-based workflow publishes metrics to GitHub Pages on merge for Grafana dashboard consumption. Baseline thresholds are established from the current codebase state and committed as a versioned JSON file.

## Technical Context

**Language/Version**: Go 1.24 (per go.mod)
**Primary Dependencies**: Gaze (`go install github.com/unbound-force/gaze/cmd/gaze@latest`) consumed directly from its repository
**Storage**: `.gaze/baseline.json` committed to repository; metrics published as JSON on `gh-pages` branch
**Testing**: `go test -coverprofile=coverage.out ./...` (existing pattern in Makefile `test-unit` target)
**Target Platform**: GitHub Actions (existing CI platform)
**Project Type**: CLI tool (Go)
**Performance Goals**: CI analysis completes within existing workflow time budget (no significant overhead)
**Constraints**: Reusable workflow must be flexible with defaulted inputs for future org-infra centralisation
**Scale/Scope**: Single repository initially, designed for org-wide adoption

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Single Source of Truth | PASS | Thresholds stored in single baseline file; CRAP threshold defaults defined once in reusable workflow inputs |
| II. Simplicity & Isolation | PASS | Separate workflows for PR analysis and event-based metrics publishing; reusable workflow is a single-purpose component |
| III. Incremental Improvement | PASS | Feature is self-contained; no unrelated changes |
| IV. Readability First | PASS | Workflow files will use descriptive step names; Makefile targets self-documenting |
| V. Do Not Reinvent the Wheel | PASS | Using Gaze (established tool) instead of custom CRAP computation |
| VI. Composability | PASS | Reusable workflow is modular; JSON output consumable by other tools |
| VII. Convention Over Configuration | PASS | All workflow inputs have reasonable defaults; users only configure when deviating |
| Makefile requirement | PASS | Adding `crapload`, `crapload-baseline`, `crapload-check` targets |
| Testing requirement | PASS | Workflow itself tested via PR; Makefile targets enable local verification |
| Conventional Commits | PASS | Commits will follow `ci:` or `feat:` prefix |
| Infrastructure centralisation | PASS | Designed as reusable workflow for future org-infra migration |

**Post-Phase 1 Re-check**: All gates still pass. No design decisions introduced violations.

## Project Structure

### Documentation (this feature)

```text
specs/003-crapload-monitoring/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
└── tasks.md           # Created by /speckit.tasks
```

### Source Code (repository root)

```text
.gaze/
└── baseline.json                          # Committed baseline thresholds

.github/workflows/
├── ci_crapload.yml                        # PR trigger: calls reusable workflow
├── ci_crapload_main.yml                   # Push to main trigger + workflow_dispatch + metrics publishing
└── reusable_crapload_analysis.yml         # Reusable workflow (workflow_call)

Makefile                                   # New targets: crapload, crapload-baseline, crapload-check
```

**Structure Decision**: No new source directories needed. This feature adds CI workflow files and a baseline configuration file. The Makefile is extended with new targets for local usage. All analysis logic is delegated to the Gaze tool.

## Reusable Workflow Design

### Inputs (with defaults)

| Input | Type | Default | Description |
|-------|------|---------|-------------|
| `go-version-file` | string | `'./go.mod'` | Path to go.mod for Go version detection |
| `gaze-version` | string | `'latest'` | Gaze version tag to install |
| `baseline-file` | string | `'.gaze/baseline.json'` | Path to baseline thresholds file |
| `packages` | string | `'./...'` | Go packages to analyse |
| `coverprofile` | string | `'coverage.out'` | Path to coverage profile (auto-generated if absent) |
| `new-function-threshold` | number | `30` | CRAP score ceiling for new functions with no baseline (industry-standard "danger" threshold) |

> **Note**: Per-function CRAP/GazeCRAP thresholds (both default to 15) are managed internally by gaze. The workflow uses `gaze report --format=json` which runs CRAP, Quality, Classification, and Docscan analysis in a single invocation. `new-function-threshold` (default 30) is the maximum allowable CRAP score for newly introduced functions that have no existing baseline entry. Existing functions are compared against their per-function baseline scores.

### Workflow Steps

1. Checkout code
2. Detect changed packages from PR (using `git diff --name-only` against the PR base branch, then extract unique Go package paths from changed `.go` files). If no Go files changed, skip all subsequent analysis steps
3. Set up Go (from `go-version-file`) — skipped if no Go changes
4. Install Gaze (`go install ... @{gaze-version}`) — skipped if no Go changes
5. Generate coverage profile (if not provided) scoped to changed packages
6. Run `gaze report --format=json --coverprofile=...` against changed packages (runs CRAP, Quality, Classification, and Docscan in a single invocation)
7. Extract CRAP data from report payload (`.crap` field) with path normalisation for baseline compatibility
8. Compare extracted CRAP results against baseline file (per-function comparison — see below)
9. Format PR comment body as markdown (regressions, improvements, new functions, quality metrics, quadrant distribution, analysis warnings)
10. Upload comment body and analysis data as artifacts for the caller workflow to consume
11. Set exit code based on threshold enforcement

> **Note**: The reusable workflow requires only `contents: read` permission. PR comment posting is the caller workflow's responsibility (`ci_crapload.yml`), which downloads the comment body artifact and posts it with `pull-requests: write`. This separation enables per-repository customization and avoids passing elevated permissions to the reusable workflow.

### Per-Function Baseline Comparison

Gaze does not provide built-in per-function baseline comparison. The per-function comparison requires a custom step:

1. Run `gaze report --format=json --coverprofile=...` to produce a combined report payload
2. Extract the `.crap` field from the report JSON using `jq`, applying path normalisation (stripping the repo root prefix from absolute file paths)
3. Parse the extracted CRAP data to get per-function scores (keyed by `file:FunctionName`)
4. Parse `.gaze/baseline.json` to load per-function baseline scores
5. For each function in the current analysis:
   - If function exists in baseline: compare current score against baseline score. Flag as regression if current > baseline, improvement if current < baseline.
   - If function does not exist in baseline: flag as new function and compare against `new-function-threshold` (default 30).
6. Fail the workflow if any regression or new-function violation is detected.

This comparison logic is implemented as a shell script step using `jq` to process the JSON files within the reusable workflow.

### Path Normalization

Gaze outputs absolute file paths in its JSON (e.g., `/home/user/project/cmd/foo.go`). Since the baseline is committed to the repository and must work across all developer machines and CI runners, all `file` fields MUST be normalized to repository-relative paths (e.g., `cmd/foo.go`) before saving to `.gaze/baseline.json`. This normalization is applied:

1. In the Makefile `crapload-baseline` target: strip the repo root prefix using `jq` after generation
2. In the reusable workflow: normalize current analysis output before comparing against baseline
3. Comparison keys use the format `relative/path:FunctionName` for uniqueness

### PR Comment Format

The comment will include:
- Overall status badge (PASS/FAIL)
- Summary table with aggregate metrics (functions analysed, avg complexity, avg line coverage, avg CRAP, CRAPload, avg contract coverage, avg GazeCRAP, GazeCRAPload, regressions, improvements, new functions)
- Quality metrics (avg contract coverage from quality analysis, avg over-specification score) when available from the gaze report
- Quadrant distribution table (Q1 Safe, Q2 Complex but Tested, Q3 Simple but Underspecified, Q4 Dangerous) when available
- Regressions section (functions exceeding baseline, highlighted)
- Improvements section (functions with better scores than baseline)
- New functions section (evaluated against default threshold)
- Analysis warnings section (any gaze report pipeline step failures, e.g., quality or classification errors)
- Link to full analysis output in workflow logs

## Main Branch Analysis & Metrics Publishing

The main branch workflow triggers on `push` to `main` with a paths filter (`**.go`, `go.mod`, `go.sum`) so it only runs when Go code changes. It also supports `workflow_dispatch` for on-demand execution.

The workflow has two jobs:

1. **Analysis**: Calls the reusable workflow with `post-comment: false` and `packages: ./...` to analyse the full codebase. The reusable workflow uploads analysis artifacts as `crapload-analysis`.

2. **Publish Metrics**: Downloads the `crapload-analysis` artifact, extracts summary metrics from `crapload-current.json`, appends a timestamped entry (with commit SHA, ref, and all summary fields) to `metrics.json` on the `gh-pages` branch, and pushes. If the `gh-pages` branch does not exist, it is initialised as an orphan branch.

The `metrics.json` file is served via GitHub Pages at `https://<org>.github.io/<repo>/metrics.json` and can be consumed by Grafana using the Infinity datasource plugin (free, supports JSON over HTTP).

## Baseline Establishment

1. Run `make test-unit` to generate `coverage.out`
2. Run `make crapload-baseline` which executes `gaze crap --format=json --coverprofile=coverage.out ./...` and normalises paths
3. Per-function scores are saved to `.gaze/baseline.json` (scores array and summary — no additional metadata)
4. Commit the baseline as part of the implementation PR

## Complexity Tracking

No constitution violations to justify. All design decisions align with established principles.
