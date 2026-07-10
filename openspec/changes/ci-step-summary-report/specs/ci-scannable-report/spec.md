## ADDED Requirements

### Requirement: Controls table shows status indicators

The Markdown report's Controls table MUST display an emoji status indicator
as a prefix to the Result column value for each control row and each
requirement row. The indicator MUST use the same emoji constants as the
terminal scan summary (`StatusPassed`, `StatusFailed`, `StatusSkipped`,
`StatusError`).

#### Scenario: Passed control displays check mark

- **GIVEN** a completed compliance scan with evaluation results
- **WHEN** a control evaluation has result `Passed`
- **THEN** the Controls table row for that control MUST display `✅ Passed` in the Result column

#### Scenario: Failed control displays cross mark

- **GIVEN** a completed compliance scan with evaluation results
- **WHEN** a control evaluation has result `Failed`
- **THEN** the Controls table row for that control MUST display `❌ Failed` in the Result column

#### Scenario: Not Applicable control displays skip indicator

- **GIVEN** a completed compliance scan with evaluation results
- **WHEN** a control evaluation has result `Not Applicable`
- **THEN** the Controls table row for that control MUST display `⏭️ Not Applicable` in the Result column

#### Scenario: Not Run result displays skip indicator

- **GIVEN** a completed compliance scan with evaluation results
- **WHEN** an assessment log has result `Not Run`
- **THEN** the requirement row MUST display `⏭️ Not Run` in the Result column

#### Scenario: Unknown result displays warning indicator

- **GIVEN** a completed compliance scan with evaluation results
- **WHEN** an assessment log has result `Unknown`
- **THEN** the requirement row MUST display `⚠️ Unknown` in the Result column

#### Scenario: Needs Review result displays warning indicator

- **GIVEN** a completed compliance scan with evaluation results
- **WHEN** an assessment log has result `Needs Review`
- **THEN** the requirement row MUST display `⚠️ Needs Review` in the Result column

#### Scenario: Requirement rows show individual status indicators

- **GIVEN** a completed compliance scan with evaluation results
- **WHEN** a control has multiple assessment logs with mixed results
- **THEN** each requirement row MUST display its own emoji prefix matching its individual result

### Requirement: Summary counts table shows status indicators

The summary counts table headers MUST be prefixed with their corresponding
emoji so the distribution is scannable at a glance without reading text.

#### Scenario: Counts table headers include emoji

- **GIVEN** a Markdown report is being generated for a policy scan
- **WHEN** the summary counts table is rendered
- **THEN** the counts table header row MUST display `✅ Passed | ❌ Failed | ⚠️ Needs Review | ⚠️ Unknown | ⏭️ N/A | ⏭️ Not Run | Total`

### Requirement: Findings group headers show status indicators

The findings section group headers MUST be prefixed with their corresponding
emoji to make group boundaries visually distinct when scrolling.

#### Scenario: Failed findings group header

- **GIVEN** a completed compliance scan with non-passing results
- **WHEN** the findings section contains a group of Failed findings
- **THEN** the group header MUST display `### ❌ Failed`

#### Scenario: Needs Review findings group header

- **GIVEN** a completed compliance scan with non-passing results
- **WHEN** the findings section contains a group of Needs Review findings
- **THEN** the group header MUST display `### ⚠️ Needs Review`

#### Scenario: Not Applicable findings group header

- **GIVEN** a completed compliance scan with non-passing results
- **WHEN** the findings section contains a group of Not Applicable findings
- **THEN** the group header MUST display `### ⏭️ Not Applicable`

#### Scenario: Unknown findings group header

- **GIVEN** a completed compliance scan with non-passing results
- **WHEN** the findings section contains a group of Unknown findings
- **THEN** the group header MUST display `### ⚠️ Unknown`

#### Scenario: Not Run findings group header

- **GIVEN** a completed compliance scan with non-passing results
- **WHEN** the findings section contains a group of Not Run findings
- **THEN** the group header MUST display `### ⏭️ Not Run`

### Requirement: Report is structurally compatible with GitHub Actions Step Summary

The Markdown report MUST produce valid HTML/Markdown structures that are
compatible with GitHub Actions Step Summary rendering when appended via
`cat report-*.md >> $GITHUB_STEP_SUMMARY`.

#### Scenario: Details blocks use valid HTML structure

- **GIVEN** a Markdown report with an embedded evaluation log
- **WHEN** the report contains `<details>` blocks
- **THEN** every `<details>` tag MUST have a matching `</details>` closing tag
- **AND** every `<details>` block MUST contain a `<summary>` element

#### Scenario: Tables have consistent column counts

- **GIVEN** a Markdown report with tables
- **WHEN** the report contains Markdown tables
- **THEN** all table rows MUST have the same number of columns as the header row

#### Scenario: Report stays within size limits

- **GIVEN** a policy scan with up to 200 controls
- **WHEN** the Markdown report is generated
- **THEN** the report size MUST be less than 512 KB

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
