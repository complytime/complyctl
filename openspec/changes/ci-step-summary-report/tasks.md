## 1. Result-to-Emoji Helper

- [x] 1.1 Add `resultEmoji(r gemara.Result) string` helper function in `internal/output/markdown.go` that maps each `gemara.Result` to its `complytime.Status*` emoji constant (`Passed`→`StatusPassed`, `Failed`→`StatusFailed`, `NotApplicable`/`NotRun`→`StatusSkipped`, `Unknown`/`NeedsReview`→`StatusError`)

## 2. Controls Table Indicators

- [x] 2.1 Update `writeControlsTable()` in `internal/output/markdown.go` to prefix each control row's Result column with `resultEmoji(ce.Result)` (e.g., `✅ Passed`)
- [x] 2.2 Update `writeControlsTable()` requirement sub-rows to prefix Result column with `resultEmoji(al.Result)`

## 3. Summary Counts Indicators

- [x] 3.1 Update `writeSummary()` in `internal/output/markdown.go` to prefix each counts table header with its emoji (e.g., `✅ Passed | ❌ Failed | ⚠️ Needs Review | ⚠️ Unknown | ⏭️ N/A | ⏭️ Not Run | Total`)

## 4. Findings Group Header Indicators

- [x] 4.1 Update `writeFindings()` in `internal/output/markdown.go` to prefix each result group heading with `resultEmoji(currentResult)` (e.g., `### ❌ Failed`)

## 5. Update Existing Tests

- [x] 5.1 Update `TestMarkdown_Write` assertions in `internal/output/markdown_test.go` to expect emoji prefixes in Controls table output
- [x] 5.2 Update `TestMarkdown_ControlsTableShowsAllControls` assertions to expect emoji in Result column
- [x] 5.3 Update `TestMarkdown_PassRate` assertions for emoji in counts table headers
- [x] 5.4 Update `TestMarkdown_FindingsSortOrder` assertions for emoji in findings group headers
- [x] 5.5 Update `TestMarkdown_ConfidenceLevelShown` assertions for emoji in findings headers
- [x] 5.6 Update `TestMarkdown_FindingsWithRecommendationAndEvidence` assertions for emoji in findings headers

## 6. Add New Tests

- [x] 6.1 Add table-driven `TestResultEmoji` unit test exercising all 6 `gemara.Result` types plus a default case
- [x] 6.2 Add `TestMarkdown_ControlsTableStatusIndicators` test verifying each result type renders with the correct emoji prefix in the Controls table
- [x] 6.3 Add `TestMarkdown_SummaryCountsTableHeaders` test verifying counts table headers contain emoji prefixes
- [x] 6.4 Add `TestMarkdown_FindingsGroupHeaderEmoji` test verifying findings group headers contain emoji prefixes for all non-Passed result types

## 7. Verification

- [x] 7.1 Run `make test-unit` and confirm all tests pass
- [x] 7.2 Run `make lint` and confirm no lint violations
- [x] 7.3 Verify a sample report: generate report output and confirm (a) emoji render as graphical icons, (b) `<details>` blocks have matching open/close tags, (c) table column counts are consistent across rows, (d) no raw HTML artifacts appear in rendered view

## 8. Documentation

- [x] 8.1 Update `CHANGELOG.md` with entry for emoji status indicators in Markdown report
- [x] 8.2 Update `AGENTS.md` Recent Changes section with `ci-step-summary-report` entry
<!-- spec-review: passed -->
<!-- code-review: passed -->
