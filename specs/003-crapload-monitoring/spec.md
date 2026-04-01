# Feature Specification: CRAP Load Monitoring

**Feature Branch**: `003-crapload-monitoring`
**Created**: 2026-03-10
**Status**: Implemented
**Input**: User description: "Implement crapload monitoring in this repository so we can continually know if new code is increasing, reducing or maintaining the risks caused by complexity and lack of robust tests."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - PR Quality Gate for CRAP Metrics (Priority: P1)

As a contributor, when I open a pull request, the CI automatically analyses the changes and posts a comment showing the CRAP and GazeCRAP scores for affected functions. If any function's CRAP or GazeCRAP score increases beyond the established baseline thresholds, the CI check fails and blocks the merge.

**Why this priority**: This is the core value proposition. Without PR-level enforcement, complexity regressions silently enter the codebase. This story provides immediate, actionable feedback at the point where changes are introduced.

**Independent Test**: Can be fully tested by opening a test PR with intentionally complex, under-tested code and verifying the CI posts a comment with CRAP/GazeCRAP scores and fails the check.

**Acceptance Scenarios**:

1. **Given** a PR with code changes that do not increase any function's CRAP or GazeCRAP score beyond the baseline, **When** the CI workflow runs, **Then** a comment is posted summarizing the scores and the check passes.
2. **Given** a PR with code changes that increase at least one function's CRAP score beyond the baseline threshold, **When** the CI workflow runs, **Then** a comment is posted highlighting the offending functions, the check fails, and the PR is blocked from merging.
3. **Given** a PR with code changes that increase at least one function's GazeCRAP score beyond the baseline threshold, **When** the CI workflow runs, **Then** a comment is posted highlighting the offending functions, the check fails, and the PR is blocked from merging.
4. **Given** a PR that reduces CRAP or GazeCRAP scores (improves quality), **When** the CI workflow runs, **Then** a comment is posted celebrating the improvement and the check passes.

---

### User Story 2 - Baseline Threshold Establishment (Priority: P1)

As a project maintainer, I need the current CRAP and GazeCRAP scores for the entire codebase assessed and recorded as baseline thresholds. These thresholds serve as the quality floor: no PR may increase any function's scores beyond these values.

**Why this priority**: The PR quality gate (Story 1) depends on having established thresholds. Without baselines, there is nothing to compare against.

**Independent Test**: Can be tested by running the analysis tool against the current codebase and verifying that a threshold configuration is generated with per-function CRAP and GazeCRAP values.

**Acceptance Scenarios**:

1. **Given** the current codebase on the main branch, **When** the baseline assessment is executed, **Then** CRAP and GazeCRAP scores are computed for all functions and stored as threshold configuration.
2. **Given** a stored baseline, **When** a PR introduces a function with scores exceeding the baseline, **Then** the CI correctly identifies the regression against the stored thresholds.
3. **Given** a stored baseline, **When** a maintainer wants to tighten the quality floor after improvements, **Then** they run a Makefile target to regenerate the baseline and commit the updated file in a separate PR.

---

### User Story 3 - Event-Based CRAP Metrics & Dashboard (Priority: P2)

As a project maintainer, I want CRAP and GazeCRAP metrics for the entire codebase to be automatically collected whenever code is merged to main and published to a GitHub Pages endpoint for Grafana dashboard consumption. This provides a longitudinal view of code quality trends without unnecessary scheduled runs when no code has changed.

**Why this priority**: While PR-level checks catch regressions at the point of change, longitudinal metrics provide trend visibility and help identify gradual quality drift or areas that need attention. Event-based triggers avoid wasted CI runs.

**Independent Test**: Can be tested by merging a PR with Go code changes and verifying the workflow runs full analysis and appends metrics to `metrics.json` on the `gh-pages` branch. Can also be triggered manually via `workflow_dispatch`.

**Acceptance Scenarios**:

1. **Given** a PR with Go code changes is merged to main, **When** the push event fires, **Then** the workflow analyses the full codebase and appends aggregate metrics to `metrics.json` on the `gh-pages` branch.
2. **Given** accumulated metrics in `metrics.json`, **When** a maintainer configures a Grafana dashboard with the GitHub Pages URL as a JSON datasource, **Then** they can visualise CRAP and GazeCRAP trends over time.
3. **Given** the workflow, **When** it is triggered manually (workflow_dispatch), **Then** it produces the same analysis and metrics update on demand.
4. **Given** a push to main that does not modify any Go files, **When** the push event fires, **Then** the workflow does not run (paths filter).

---

### User Story 4 - Reusable Workflow (Priority: P2)

As a CI/CD engineer, I want the CRAP analysis workflow to be designed as a reusable workflow so that other repositories in the organisation can adopt it without duplicating configuration.

**Why this priority**: Reusability multiplies the value of this investment across the organisation and reduces maintenance burden.

**Independent Test**: Can be tested by referencing the reusable workflow from a separate repository (or a test workflow in the same repo) and verifying it executes correctly.

**Acceptance Scenarios**:

1. **Given** a reusable workflow definition, **When** another repository references it with `uses:`, **Then** the CRAP analysis runs successfully in the calling repository's context.
2. **Given** the reusable workflow, **When** it is called with configurable inputs (e.g., threshold overrides, report format), **Then** it respects those inputs and adapts its behaviour accordingly.

---

### Edge Cases

- What happens when a PR introduces entirely new functions that have no baseline? The system should evaluate new functions against a default maximum CRAP threshold (e.g., CRAP score of 30, the industry-standard "danger" threshold).
- What happens when the analysis tool fails to install or encounters a runtime error? The CI should fail gracefully with a clear error message rather than silently passing.
- What happens when a PR only modifies non-code files (documentation, configuration)? The workflow should detect no analysable changes before installing Gaze or running tests, post a comment indicating no CRAP impact, and pass the check.
- What happens when the codebase has no test coverage data available? The CRAP formula yields the maximum risk score (complexity^2 + complexity), and this should be clearly reported.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST compute CRAP scores for all functions using the formula: CRAP(m) = complexity^2 * (1 - lineCoverage/100)^3 + complexity
- **FR-002**: The system MUST compute GazeCRAP scores for all functions using the formula: GazeCRAP(m) = complexity^2 * (1 - contractCoverage/100)^3 + complexity
- **FR-003**: The system MUST run CRAP and GazeCRAP analysis automatically on every pull request
- **FR-004**: The system MUST post a comment on each pull request summarising the CRAP and GazeCRAP scores for all functions in packages containing changed files. The reusable workflow produces the comment body as an artifact; the caller workflow is responsible for posting it
- **FR-005**: The system MUST fail the CI check when any function's CRAP or GazeCRAP score exceeds the established baseline threshold
- **FR-006**: The system MUST establish baseline CRAP and GazeCRAP thresholds by analysing the current state of the codebase
- **FR-007**: The system MUST run a full codebase CRAP and GazeCRAP analysis whenever Go code is merged to main (push event with paths filter) and publish aggregate metrics to a GitHub Pages endpoint for dashboard consumption
- **FR-008**: The main branch analysis workflow MUST also support manual triggering (workflow_dispatch)
- **FR-009**: The PR analysis workflow MUST be designed as a reusable workflow callable by other repositories
- **FR-010**: The system MUST clearly distinguish between CRAP regressions (score increases) and improvements (score decreases) in PR comments
- **FR-011**: New functions introduced in a PR (with no existing baseline) MUST be evaluated against a default maximum CRAP threshold
- **FR-012**: The system MUST handle PRs with no analysable code changes gracefully, indicating no CRAP impact
- **FR-013**: The reusable workflow MUST accept configurable inputs (e.g., `new-function-threshold`, `baseline-file`, `packages`, `coverprofile`). Per-function CRAP/GazeCRAP thresholds (default 15) are managed internally by gaze
- **FR-014**: The reusable workflow MUST require only `contents: read` permission. PR comment posting and other write operations MUST be the caller workflow's responsibility, enabling per-repository customization without passing elevated permissions to the reusable workflow

### Key Entities

- **CRAP Score**: A per-function risk metric combining cyclomatic complexity with line coverage. Higher scores indicate higher risk of defects when modifying the function.
- **GazeCRAP Score**: A per-function risk metric combining cyclomatic complexity with contract coverage (whether tests assert on the function's contractual obligations rather than just executing its code). Higher scores indicate functions whose tests may pass despite not verifying correct behaviour.
- **Baseline Threshold**: The recorded CRAP and GazeCRAP scores from the current codebase state, serving as the maximum allowable values. Any PR that causes a function to exceed its baseline triggers a CI failure.
- **PR Comment Report**: An automated comment posted on pull requests containing a summary of CRAP and GazeCRAP scores, highlighting regressions and improvements.
- **Metrics Entry**: A timestamped record of codebase-wide CRAP and GazeCRAP aggregate metrics, appended to `metrics.json` on the `gh-pages` branch whenever Go code is merged to main, enabling longitudinal trend analysis via Grafana.

## Clarifications

### Session 2026-03-10

- Q: Should baseline thresholds be per-function or aggregate? → A: Per-function — each function has its own baseline score; a regression occurs when any individual function exceeds its recorded score.
- Q: What is the scope of PR analysis ("affected functions")? → A: All functions in packages containing changed files.
- Q: How should the baseline be updated after improvements? → A: Manual only — maintainer runs a Makefile target and commits the updated baseline in a separate PR.
- Q: What constitutes an "explicit override" for intentional CRAP regressions? → A: Update the baseline file in the same PR so the regression is visible in the diff and approved by reviewers.

### Session 2026-03-13

- Q: How should the PR comment body be assembled to reduce workflow complexity? → A: Build the complete markdown body in the compare step (shell) instead of a separate JavaScript step. This eliminates intermediate temp files and the github-script dependency for report building.
- Q: Should Gaze installation be deferred until after confirming Go file changes exist? → A: Yes — move change detection before Gaze installation and skip installation when no Go files changed, avoiding unnecessary `go install` overhead on non-code PRs.

### Session 2026-03-18

- Q: Should the reusable workflow post PR comments directly? → A: No — the reusable workflow should only require `contents: read` and produce the comment body as an artifact. The caller workflow handles posting, enabling per-repository customization and avoiding elevated permissions in the reusable workflow.

## Assumptions

- The repository uses a Go codebase compatible with the Gaze analysis tool.
- Line coverage data is available (generated by Go's built-in test coverage tooling).
- The industry-standard CRAP threshold of 30 will be used as the default maximum for new functions without an existing baseline.
- Metrics are published as a JSON file on the `gh-pages` branch, served via GitHub Pages, and consumable by Grafana's Infinity datasource plugin or any HTTP-capable JSON datasource.
- Baseline thresholds will be stored as a committed file in the repository so they are versioned alongside the code.
- The reusable workflow will target the repository's existing CI platform, consistent with current CI infrastructure.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of pull requests with code changes receive an automated CRAP/GazeCRAP analysis comment within the CI pipeline execution time
- **SC-002**: No PR that increases any function's CRAP or GazeCRAP score beyond the baseline can be merged unless the baseline file is updated in the same PR (making the regression visible in the diff and subject to reviewer approval)
- **SC-003**: Metrics are published to GitHub Pages on every Go code merge to main, with data consumable by Grafana via JSON datasource
- **SC-004**: The reusable workflow is successfully callable from at least one other repository without modification
- **SC-005**: Contributors can identify which functions need quality improvement within 30 seconds of reading a PR comment or metrics dashboard
- **SC-006**: The baseline is established and committed within the initial implementation, providing a concrete starting point for quality tracking
