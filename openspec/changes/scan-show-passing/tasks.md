<!-- Tasks use [P] marker for parallel-eligible tasks. -->
<!-- [P] tasks within a phase can be executed concurrently. -->
<!-- Sequential tasks (no [P]) must complete in order. -->

## 1. Refactor scan summary types

- [x] 1.1 Rename `nonPassingEntry` struct to `summaryEntry` in `internal/output/scan_summary.go`
- [x] 1.2 Rename `nonPassingSortPriority()` to `sortPriority()` and add `gemara.Passed` case returning priority 6 (existing `default` case remains at 5)
- [x] 1.3 Update all references to renamed types within `scan_summary.go`

## 2. Include passing entries in FormatScanSummary

- [x] 2.1 Add `showPassing bool` parameter to `FormatScanSummary()` signature
- [x] 2.2 In the `gemara.Passed` case, build a `summaryEntry` with `StatusPassed` emoji and append to entries list
- [x] 2.3 When `showPassing` is `false`, filter out passing entries before building table rows (preserve pass count in conclusion)
- [x] 2.4 Update the call site in `cmd/complyctl/cli/scan.go` to pass the new `showPassing` argument

## 3. Add --show-passing CLI flag and env var

- [x] 3.1 Add `showPassing` field to the `scanOptions` struct in `cmd/complyctl/cli/scan.go`
- [x] 3.2 Register `--show-passing` as a `BoolVar` flag on the scan command with default `true`
- [x] 3.3 Add `COMPLYTIME_SHOW_PASSING` environment variable support (flag > env var > default precedence)
- [x] 3.4 Thread `scanOptions.showPassing` through `processScanOutput()` to the `FormatScanSummary()` call

## 4. Tests

- [x] 4.1 Update `TestFormatScanSummary_SingleTarget` to pass `showPassing=true` and verify passing rows appear
- [x] 4.2 Update `TestFormatScanSummary_AllPassed` to pass `showPassing=true` and verify table renders with passing rows (change from current `NotContains "TARGET ID"` assertion)
- [x] 4.3 [P] Add `TestFormatScanSummary_ShowPassingFalse` to verify passing rows are hidden when `showPassing=false` and pass count is preserved in conclusion
- [x] 4.4 [P] Add `TestFormatScanSummary_SortOrder` to verify sort priority: Failed (1) > Unknown (2) > NeedsReview (3) > NotApplicable/NotRun (4) > other (5) > Passed (6)
- [x] 4.5 [P] Add `TestFormatScanSummary_PassingWithEmptyMessage` to verify passing row with no step message shows empty MESSAGE column
- [x] 4.6 Update `TestFormatScanSummary_MultipleTargets` to pass `showPassing` parameter
- [x] 4.7 Update `TestFormatScanSummary_MissingTargetID` to pass `showPassing` parameter
- [x] 4.8 Update `TestFormatScanSummary_ControlIDMissing` to pass `showPassing` parameter
- [x] 4.9 Run `make test-unit` and verify all tests pass

## 5. Verification and documentation

- [x] 5.1 Run `make lint` and fix any issues
- [x] 5.2 Run `make vet` and fix any issues
- [x] 5.3 Run `make test-e2e` and verify no regressions in scan-related E2E tests
- [x] 5.4 Update `CHANGELOG.md` with entry under `### Added` for `--show-passing` flag and `COMPLYTIME_SHOW_PASSING` env var, including migration note
- [x] 5.5 Update `AGENTS.md` Recent Changes section with scan-show-passing entry
<!-- spec-review: passed -->
<!-- code-review: passed -->
