## Why

Multi-target scans under the same policy produce OSCAL, SARIF, and Markdown
reports whose filenames contain only `policyID` + timestamp. When targets
process within the same second, timestamps collide and each file overwrites
the previous one — only the last target's report survives. The EvaluationLog
formatter already includes `targetID` and is unaffected. Fixes #773.

## What Changes

- Extract a shared `BuildReportFilename` helper in `internal/output/filename.go`
  that all four formatters call for consistent filename construction.
- Add `targetID` to OSCAL, SARIF, and Markdown report filenames, matching the
  existing EvaluationLog pattern.
- Refactor the EvaluationLog formatter to use the same shared helper.
- Handle empty `targetID` gracefully: omit the segment instead of producing
  double dashes.

## Capabilities

### New Capabilities

- `report-filename-helper`: Centralized `BuildReportFilename` function that
  constructs `{prefix}-{policyID}-{targetID}-{timestamp}.{ext}` with
  sanitization via `complytime.FilenameSafe` and graceful empty-targetID
  handling.

### Modified Capabilities

<!-- No existing spec-level behavior changes — this is a bug fix to internal
     filename construction. -->

## Impact

- **Code**: `internal/output/oscal.go`, `internal/output/sarif.go`,
  `internal/output/markdown.go`, `internal/output/evaluator.go` (inline
  `fmt.Sprintf` replaced with helper call). New file
  `internal/output/filename.go`.
- **Tests**: New `internal/output/filename_test.go`. Updates to
  `oscal_test.go` and `sarif_test.go` to assert `targetID` in filenames.
- **APIs**: No signature changes. `ToOSCAL`, `ToSARIF`, and `Markdown.Write`
  remain unchanged.
- **Dependencies**: None added.
- **Not changed**: `cmd/behavioral-report/main.go` (single-target, hardcoded
  filenames). `internal/complytime/consts.go` (`FilenameSafe` already
  sufficient).
