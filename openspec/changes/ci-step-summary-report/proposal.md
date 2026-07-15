## Why

CI workflows that consume `complyctl scan --format pretty` currently duplicate
the report logic with custom Python scripts to produce a GitHub Actions Step
Summary (see `org-infra/reusable_compliance.yml`). This creates two divergent
representations of the same compliance data, doubling the maintenance surface
and requiring a `pip install pyyaml` dependency in CI.

The `main` branch already has significant improvements (summary metadata table,
pass/fail counts, controls table, findings with collapsible evidence, evaluation
log in `<details>`). Two remaining gaps prevent the Markdown report from being
directly usable as a Step Summary:

1. **No status indicators** -- the Controls table lacks visual pass/fail emoji,
   forcing users to read every row to find failures.
2. **Step Summary rendering verification** -- `<details>/<summary>` blocks,
   Markdown tables, and report length have not been validated against GitHub's
   Step Summary renderer and its ~1 MB size limit.

Closing these gaps lets CI consumers replace ~55 lines of custom Python with
a simple `cat report-*.md >> $GITHUB_STEP_SUMMARY`.

## What Changes

- Add emoji status indicators to the Controls table in the Markdown report
  (reusing existing `complytime.Status*` constants: `StatusPassed`,
  `StatusFailed`, `StatusSkipped`, `StatusError`)
- Add emoji status indicators to the summary counts table for scanability
- Update existing unit tests and add new tests to verify the indicators render
  correctly
- Verify `<details>/<summary>` blocks and Markdown tables are compatible with
  GitHub Actions Step Summary rendering constraints

## Capabilities

### New Capabilities

- `ci-scannable-report`: Add emoji status indicators to the Markdown report's
  Controls table and summary section so the report is scannable at a glance
  when used as a GitHub Actions Step Summary

### Modified Capabilities

_(none -- no existing spec-level requirements are changing)_

## Impact

- **Code**: `internal/output/markdown.go` -- `writeControlsTable()` and
  `writeSummary()` gain emoji indicators; `writeFindings()` gains
  result-specific emoji in group headers
- **Tests**: `internal/output/markdown_test.go` -- existing tests updated
  to expect emoji in output; new tests for indicator rendering
- **Dependencies**: None (reuses existing `complytime.Status*` constants)
- **Downstream**: Enables `complytime/org-infra` to simplify
  `reusable_compliance.yml` by removing custom Python summary generation
  and `pip install pyyaml` dependency
- **API**: No changes to CLI flags, protobuf, or provider interface

## Constitution Alignment

### I. Autonomous Collaboration

Not directly applicable -- no artifact-based communication changes. The change
is a presentational enhancement to an existing output format.

### II. Composability First

PASS -- the new `resultEmoji()` helper is a pure function that centralizes the
`gemara.Result` → emoji mapping. It is composable and reusable by any future
report format that needs status indicators.

### III. Observable Quality

PASS -- emoji status indicators improve visual scanability of compliance
reports, making quality signals (pass/fail) immediately observable without
reading every row. This directly supports the principle of making quality
claims visible and actionable.

### IV. Testability

PASS -- each emoji mapping is independently testable via a table-driven unit
test. Every insertion point (controls table, summary headers, findings headers)
is verified through dedicated test cases.
