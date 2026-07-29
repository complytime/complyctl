## ADDED Requirements

### Requirement: Centralized report filename construction

The system MUST provide a `BuildReportFilename(prefix, policyID, targetID, ext string) string`
function in `internal/output/filename.go` that all report formatters call to
construct output filenames.

The function MUST produce filenames matching the pattern
`{prefix}-{sanitized(policyID)}-{sanitized(targetID)}-{timestamp}.{ext}`
where `timestamp` uses `time.Now().Format("20060102-150405")` and sanitization
uses `complytime.FilenameSafe`.

Tests MUST use pattern matching (e.g., `strings.HasPrefix`/`strings.Contains`)
for the timestamp segment — exact timestamp assertion is not required. This is
an accepted limitation of embedding `time.Now()` in the helper, consistent with
the existing formatter implementations.

#### Scenario: Multi-target scan produces distinct filenames

- **GIVEN** a workspace with two configured targets (`web-server`, `db-server`)
  under policy `test-policy`
- **WHEN** `complyctl scan` evaluates both targets within the same second
- **THEN** each formatter produces files with distinct names containing
  the respective target ID (e.g., `report-test-policy-web-server-{ts}.md`
  and `report-test-policy-db-server-{ts}.md`)

#### Scenario: Policy ID with path separators is sanitized

- **GIVEN** a policy with ID `org/policy/v1` containing forward slashes
- **WHEN** `BuildReportFilename` is called with that policy ID
- **THEN** slashes are replaced with dashes in the filename
  (e.g., `report-org-policy-v1-{targetID}-{ts}.md`)

#### Scenario: Target ID with path separators is sanitized

- **GIVEN** a target with ID `ns/target` containing forward slashes
- **WHEN** `BuildReportFilename` is called with that target ID
- **THEN** slashes are replaced with dashes in the filename
  (e.g., `report-{policyID}-ns-target-{ts}.md`)

### Requirement: Empty target ID omits segment

When `targetID` is an empty string, `BuildReportFilename` MUST omit the
target segment entirely, producing `{prefix}-{sanitized(policyID)}-{timestamp}.{ext}`.
The filename MUST NOT contain double dashes from the omitted segment.

#### Scenario: Empty target ID produces backward-compatible filename

- **GIVEN** a formatter invoked without a target scope
- **WHEN** `BuildReportFilename` is called with an empty `targetID`
- **THEN** the returned filename matches `{prefix}-{policyID}-{ts}.{ext}`
  with no double dashes

### Requirement: Compound file extensions

`BuildReportFilename` MUST support compound extensions (e.g., `sarif.json`)
by appending `.{ext}` to the constructed base name.

#### Scenario: SARIF compound extension

- **GIVEN** the SARIF formatter constructing its output filename
- **WHEN** `ext` is `sarif.json`
- **THEN** the filename ends with `.sarif.json`
  (e.g., `scan-{policyID}-{targetID}-{ts}.sarif.json`)

### Requirement: All formatters use the shared helper

The OSCAL, SARIF, Markdown, and EvaluationLog formatters MUST all construct
their output filenames by calling `BuildReportFilename` instead of inline
`fmt.Sprintf` calls.

#### Scenario: OSCAL formatter includes target ID

- **GIVEN** an evaluation log with `Target.Id` set to `web-server`
- **WHEN** `ToOSCAL` writes a report for that target
- **THEN** the output filename contains `web-server`
  (e.g., `assessment-results-{policyID}-web-server-{ts}.json`)

#### Scenario: SARIF formatter includes target ID

- **GIVEN** an evaluation log with `Target.Id` set to `web-server`
- **WHEN** `ToSARIF` writes a report for that target
- **THEN** the output filename contains `web-server`
  (e.g., `scan-{policyID}-web-server-{ts}.sarif.json`)

#### Scenario: Markdown formatter includes target ID

- **GIVEN** an evaluation log with `Target.Id` set to `web-server`
- **WHEN** `Markdown.Write` writes a report for that target
- **THEN** the output filename contains `web-server`
  (e.g., `report-{policyID}-web-server-{ts}.md`)

#### Scenario: EvaluationLog formatter uses shared helper

- **GIVEN** an evaluator constructed with `policyID` and `targetID`
- **WHEN** `Evaluator.Write` writes a report
- **THEN** the output filename matches the same pattern produced by
  `BuildReportFilename("evaluation-log", policyID, targetID, "yaml")`
