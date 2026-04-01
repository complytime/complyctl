# Tasks: CRAP Load Monitoring

**Input**: Design documents from `/specs/003-crapload-monitoring/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Not explicitly requested — test tasks omitted.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story. US4 (Reusable Workflow) is placed as the foundational phase because US1 and US3 both depend on it.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Makefile targets for local testing, enabling contributors to verify CRAP analysis before opening PRs

- [x] T001 Add `crapload` target to Makefile that installs gaze (if not present via `go install github.com/unbound-force/gaze/cmd/gaze@latest`), runs `go test -coverprofile=coverage.out ./...`, and executes `gaze crap --format=text ./...` to display CRAP and GazeCRAP scores in Makefile
- [x] T002 Add `crapload-baseline` target to Makefile that runs coverage generation and `gaze crap --format=json ./...`, saving per-function output to `.gaze/baseline.json` in Makefile
- [x] T003 Add `crapload-check` target to Makefile that runs coverage generation, executes `gaze crap --format=json ./...`, and compares results against `.gaze/baseline.json` reporting regressions in Makefile

---

## Phase 2: User Story 4 - Reusable Workflow (Priority: P2, Foundational)

**Goal**: Create a reusable GitHub Actions workflow (`workflow_call`) that performs CRAP/GazeCRAP analysis, compares against baseline, and produces a comment body artifact. Requires only `contents: read`. All other workflows (US1, US3) consume this.

**Independent Test**: Reference the reusable workflow from a test workflow in the same repo and verify it executes correctly.

### Implementation for User Story 4

- [x] T004 [US4] Create reusable workflow skeleton with `workflow_call` trigger, 6 configurable inputs with defaults (go-version-file, gaze-version, baseline-file, packages, coverprofile, new-function-threshold=30), required permissions (contents: read only), and outputs (status, crapload-count, gaze-crapload-count, regressions-count, improvements-count). Per-function CRAP/GazeCRAP thresholds (default 15) are managed internally by gaze (FR-009, FR-013, FR-014) in .github/workflows/reusable_crapload_analysis.yml
- [x] T005 [US4] Implement core analysis job steps: checkout code, detect changed packages using `git diff --name-only` against PR base branch to extract unique Go package paths from changed `.go` files (skip all subsequent steps if no Go changes), set up Go from go-version-file input, install Gaze via `go install github.com/unbound-force/gaze/cmd/gaze@${{ inputs.gaze-version }}`, generate coverage profile scoped to changed packages with `go test -coverprofile`, run `gaze report --format=json --coverprofile` against changed packages (runs CRAP, Quality, Classification, and Docscan in one invocation), and extract CRAP data from report payload with path normalisation for baseline compatibility in .github/workflows/reusable_crapload_analysis.yml
- [x] T006 [US4] Implement per-function baseline comparison step using `jq`: parse gaze JSON output keyed by `package.FunctionName`, load baseline from `inputs.baseline-file`, classify each function as regression (current score > baseline score), improvement (current score < baseline score), or new (no baseline entry, evaluate against `new-function-threshold` input, default 30); fail workflow if any regression or new-function violation detected (FR-005, FR-010, FR-011) in .github/workflows/reusable_crapload_analysis.yml
- [x] T007 [US4] Build PR comment body as markdown in the compare step: overall status (PASS/FAIL), summary metrics table, regressions section (highlighted) clearly distinguishing score increases from improvements (FR-010), improvements section, new functions section, and link to workflow logs; upload comment body as artifact for the caller workflow to post (FR-014) in .github/workflows/reusable_crapload_analysis.yml
- [x] T008 [US4] Implement edge case handling: detect PRs with no analysable code changes (post "no CRAP impact" comment and pass), handle Gaze installation failure (fail with clear error message), handle missing coverage data (report maximum risk scores), and set workflow outputs and exit code based on threshold enforcement in .github/workflows/reusable_crapload_analysis.yml

**Checkpoint**: Reusable workflow is complete and independently callable. US1 and US3 can now proceed.

---

## Phase 3: User Story 2 - Baseline Threshold Establishment (Priority: P1)

**Goal**: Assess current codebase CRAP and GazeCRAP scores and commit them as the quality floor baseline.

**Independent Test**: Run `make crapload-baseline` and verify `.gaze/baseline.json` contains per-function scores for all functions in the codebase.

### Implementation for User Story 2

- [x] T009 [US2] Create `.gaze/` directory and run `make crapload-baseline` against the current main branch codebase to generate `.gaze/baseline.json` with per-function CRAP and GazeCRAP scores
- [x] T010 [US2] Verify baseline file `.gaze/baseline.json` contains valid JSON with per-function entries in `scores[]` (function name, file path, package, complexity, line coverage, contract coverage, CRAP score, GazeCRAP score, quadrant) and aggregate `summary` data

**Checkpoint**: Baseline is established and committed. PR quality gate (US1) can now compare against it.

---

## Phase 4: User Story 1 - PR Quality Gate for CRAP Metrics (Priority: P1)

**Goal**: Every PR automatically receives a CRAP/GazeCRAP analysis comment and the CI check fails on regressions.

**Independent Test**: Open a test PR with intentionally complex, under-tested code and verify the CI posts a comment and fails the check.

### Implementation for User Story 1

- [x] T011 [US1] Create PR trigger workflow with `pull_request` (branches: main) trigger, calling `reusable_crapload_analysis.yml` with `contents: read` permission. Add a second job that downloads the comment body artifact and posts/updates a PR comment using `pull-requests: write` (FR-003, FR-004, FR-014) in .github/workflows/ci_crapload.yml

**Checkpoint**: PR quality gate is fully functional. Opening a PR triggers analysis, posts a comment, and fails on regressions.

---

## Phase 5: User Story 3 - Event-Based CRAP Metrics & Dashboard (Priority: P2)

**Goal**: An event-based CI job analyses the full codebase whenever Go code is merged to main, publishes aggregate metrics to GitHub Pages for Grafana dashboard consumption.

**Independent Test**: Merge a PR with Go changes (or trigger via `workflow_dispatch`) and verify the workflow runs full analysis and appends metrics to `metrics.json` on the `gh-pages` branch.

### Implementation for User Story 3

- [x] T012 [US3] Create main branch workflow with `push` trigger (branches: main, paths: `**.go`, `go.mod`, `go.sum`) and `workflow_dispatch` trigger. Analysis job calls `reusable_crapload_analysis.yml` with `post-comment: false` and `packages: ./...`. Publish-metrics job downloads `crapload-analysis` artifact, extracts summary from `crapload-current.json`, appends timestamped entry to `metrics.json` on `gh-pages` branch, and pushes. Initialises `gh-pages` as orphan branch if missing. In .github/workflows/ci_crapload_main.yml

**Checkpoint**: Event-based metrics publishing is operational. Merged PRs with Go changes produce updated metrics on GitHub Pages.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: End-to-end validation and documentation updates

- [x] T013 Validate end-to-end local workflow by running `make crapload`, `make crapload-baseline`, and `make crapload-check` sequentially and verifying correct output at each step
- [x] T014 [P] Update quickstart.md with any adjustments discovered during implementation (corrected commands, additional prerequisites, troubleshooting tips) in specs/003-crapload-monitoring/quickstart.md
- [x] T015 [P] Add `.gaze/` directory to `.gitignore` exclusion list if needed (ensure `baseline.json` is tracked but any transient analysis output files are ignored)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **US4 Reusable Workflow (Phase 2)**: Depends on Setup for Makefile context understanding but can technically start in parallel
- **US2 Baseline (Phase 3)**: Depends on Setup (T001-T003 for Makefile targets) — the `make crapload-baseline` command must work
- **US1 PR Gate (Phase 4)**: Depends on US4 (reusable workflow must exist) and US2 (baseline must be committed)
- **US3 Event-Based Metrics (Phase 5)**: Depends on US4 (reusable workflow must exist) — independent of US1 and US2
- **Polish (Phase 6)**: Depends on all previous phases

### User Story Dependencies

- **US4 (P2, Foundational)**: Can start after Setup — no dependencies on other stories. Must complete before US1 and US3.
- **US2 (P1)**: Can start after Setup — needs Makefile targets. Must complete before US1.
- **US1 (P1)**: Depends on US4 (reusable workflow) AND US2 (baseline file)
- **US3 (P2)**: Depends on US4 (reusable workflow) only — can run in parallel with US2 and US1

### Within Each User Story

- Workflow skeleton before step implementation
- Core analysis before baseline comparison
- Baseline comparison before PR comment formatting
- All steps complete before edge case handling

### Parallel Opportunities

- T002 and T003 can run in parallel (different Makefile targets, independent logic)
- T005 and T006 are sequential (T006 depends on T005's output format)
- T012 (US3) can run in parallel with T009-T011 (US2 + US1) since US3 only depends on US4
- T014 and T015 can run in parallel (different files)

---

## Parallel Example: User Story 4

```bash
# T004 must complete first (workflow skeleton)
# Then T005 and T006 are sequential (analysis → comparison)
# T007 depends on T006 (needs comparison results to format comment)
# T008 can partially overlap with T007 (edge cases are independent concerns)
```

## Parallel Example: Phase 4 + Phase 5

```bash
# After US4 completes:
# US1 (T011) and US3 (T012) can start in parallel since they're independent callers
# But US1 also needs US2 baseline, so US3 can start earlier
```

---

## Implementation Strategy

### MVP First (US4 + US2 + US1)

1. Complete Phase 1: Setup (Makefile targets)
2. Complete Phase 2: US4 (Reusable Workflow — the core engine)
3. Complete Phase 3: US2 (Baseline — establish the quality floor)
4. Complete Phase 4: US1 (PR Quality Gate)
5. **STOP and VALIDATE**: Open a test PR and verify comment + enforcement
6. Deploy to main

### Incremental Delivery

1. Setup + US4 → Reusable workflow ready
2. Add US2 → Baseline committed → Quality floor established
3. Add US1 → PR gate active (MVP!)
4. Add US3 → Event-based metrics publishing → Full feature complete
5. Polish → Documentation and edge case hardening

### Fastest Path to Value

US4 → US2 → US1 is the critical path. US3 can be deferred without blocking core value delivery.

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US4 is labelled P2 in the spec but implemented first because it is foundational
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- The reusable workflow file (.github/workflows/reusable_crapload_analysis.yml) is the most complex artifact — tasks T004-T008 are sequential within it
