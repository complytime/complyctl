## ADDED Requirements

### Requirement: Scan summary displays passing controls
The scan summary table produced by `FormatScanSummary()` MUST include
passing controls as rows in the results table, using the `StatusPassed`
emoji in the STATUS column. This supersedes FR-037's non-passing-only
table principle per user feedback in [#438](https://github.com/complytime/complyctl/issues/438).

#### Scenario: Scan with mixed results shows passing and failing
- **GIVEN** a scan that produces 5 assessments: 3 Passed, 1 Failed, 1 Skipped
- **WHEN** `FormatScanSummary()` is called with `showPassing=true`
- **THEN** the summary table MUST contain 5 rows, with passing rows showing the `StatusPassed` emoji and failing rows showing the `StatusFailed` emoji

#### Scenario: All assessments pass
- **GIVEN** a scan that produces 3 assessments, all with Passed result
- **WHEN** `FormatScanSummary()` is called with `showPassing=true`
- **THEN** the summary table MUST display 3 passing rows with the `StatusPassed` emoji
- **AND** the conclusion line MUST read `3 requirements: 3 passed, 0 failed, 0 skipped, 0 errors`

Note: This changes the current behavior where all-passing scans render no table. With `showPassing=true` (default), all-passing scans will now render a table with passing rows.

#### Scenario: Passing entries sorted after non-passing
- **GIVEN** a scan that produces assessments with mixed results including Passed, Failed, Unknown, NeedsReview, and NotApplicable
- **WHEN** `FormatScanSummary()` is called with `showPassing=true`
- **THEN** the summary table MUST sort entries by priority: Failed (1) > Unknown (2) > NeedsReview (3) > NotApplicable/NotRun (4) > other (5) > Passed (6)

#### Scenario: All non-passing results with show-passing enabled
- **GIVEN** a scan that produces 3 assessments, all with Failed result
- **WHEN** `FormatScanSummary()` is called with `showPassing=true`
- **THEN** the summary table MUST contain 3 failed rows
- **AND** no passing rows MUST appear (since none passed)

#### Scenario: Zero assessments
- **GIVEN** a scan that produces zero assessments
- **WHEN** `FormatScanSummary()` is called with `showPassing=true`
- **THEN** no table MUST be displayed
- **AND** the conclusion line MUST read `0 requirements: 0 passed, 0 failed, 0 skipped, 0 errors`

#### Scenario: Passing assessment with no step message
- **GIVEN** a scan that produces 1 Passed assessment whose steps have no message text
- **WHEN** `FormatScanSummary()` is called with `showPassing=true`
- **THEN** the passing row MUST appear with an empty MESSAGE column

### Requirement: Show-passing flag controls passing row visibility
The `complyctl scan` command MUST accept a `--show-passing` boolean flag
that controls whether passing controls appear in the summary table. The
flag MUST also be configurable via the `COMPLYTIME_SHOW_PASSING` environment
variable, following precedence: flag > env var > default (`true`).

#### Scenario: Flag defaults to true
- **GIVEN** a policy with both passing and failing controls
- **WHEN** a user runs `complyctl scan` without specifying `--show-passing`
- **THEN** passing controls MUST appear in the summary table

#### Scenario: Flag set to false hides passing rows
- **GIVEN** a policy with both passing and failing controls
- **WHEN** a user runs `complyctl scan --show-passing=false`
- **THEN** only non-passing controls MUST appear in the summary table
- **AND** the conclusion line MUST still report the correct pass count

#### Scenario: Flag set to false with all passing
- **GIVEN** a policy where all controls pass
- **WHEN** a user runs `complyctl scan --show-passing=false`
- **THEN** no table MUST be displayed
- **AND** the conclusion line MUST report all requirements as passed

#### Scenario: Environment variable override
- **GIVEN** `COMPLYTIME_SHOW_PASSING=false` is set in the environment
- **WHEN** a user runs `complyctl scan` without the `--show-passing` flag
- **THEN** passing controls MUST NOT appear in the summary table

#### Scenario: Flag takes precedence over environment variable
- **GIVEN** `COMPLYTIME_SHOW_PASSING=false` is set in the environment
- **WHEN** a user runs `complyctl scan --show-passing=true`
- **THEN** passing controls MUST appear in the summary table

#### Scenario: Show-passing only affects terminal summary
- **GIVEN** a user runs `complyctl scan --format pretty --show-passing=false`
- **WHEN** the scan completes
- **THEN** the terminal summary table MUST NOT include passing rows
- **AND** the Markdown report file MUST still include all results (passing and non-passing) since the Markdown formatter operates on the full EvaluationLog

## MODIFIED Requirements
None.

## REMOVED Requirements
None.
