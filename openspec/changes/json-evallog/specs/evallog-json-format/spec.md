## ADDED Requirements

### FR-001: EvaluationLog JSON format flag

`complyctl scan` MUST accept a `--log-format` flag with
valid values `yaml` and `json` (default: `yaml`) that
controls the serialization format of the EvaluationLog
output file. Values are case-sensitive.

#### Scenario: Default YAML output (no flag)
- **GIVEN** a configured workspace with at least one
  policy
- **WHEN** `complyctl scan` is run without `--log-format`
- **THEN** the EvaluationLog is written as
  `evaluation-log-<policy>-<target>-<timestamp>.yaml`

#### Scenario: JSON output via flag
- **GIVEN** a configured workspace with at least one
  policy
- **WHEN** `complyctl scan --log-format json` is run
- **THEN** the EvaluationLog is written as
  `evaluation-log-<policy>-<target>-<timestamp>.json`
  with valid JSON content
- **AND** the file path printed to stderr ends with
  `.json`

#### Scenario: YAML output via flag
- **GIVEN** a configured workspace with at least one
  policy
- **WHEN** `complyctl scan --log-format yaml` is run
- **THEN** the EvaluationLog is written as
  `evaluation-log-<policy>-<target>-<timestamp>.yaml`

#### Scenario: Invalid format value
- **GIVEN** a configured workspace with at least one
  policy
- **WHEN** `complyctl scan --log-format xml` is run
- **THEN** the command returns a non-zero exit code and
  prints an error indicating the invalid format value

### FR-002: EvaluationLog JSON format env var

`complyctl scan` MUST read `COMPLYTIME_LOG_FORMAT` as an
environment variable equivalent to `--log-format`. The
`--log-format` flag MUST take precedence when both are
set.

#### Scenario: JSON output via env var
- **GIVEN** a configured workspace with at least one
  policy
- **WHEN** `COMPLYTIME_LOG_FORMAT=json complyctl scan`
  is run without `--log-format`
- **THEN** the EvaluationLog is written as a `.json`
  file

#### Scenario: Flag takes precedence over env var
- **GIVEN** a configured workspace with at least one
  policy
- **WHEN** `COMPLYTIME_LOG_FORMAT=json complyctl scan
  --log-format yaml` is run
- **THEN** the EvaluationLog is written as a `.yaml`
  file

#### Scenario: Invalid env var value
- **GIVEN** a configured workspace with at least one
  policy
- **WHEN** `COMPLYTIME_LOG_FORMAT=xml complyctl scan`
  is run without `--log-format`
- **THEN** the command returns a non-zero exit code and
  prints an error indicating the invalid format value

### FR-003: JSON EvaluationLog structural fidelity

The JSON EvaluationLog MUST use kebab-case field names
matching the upstream go-gemara JSON schema (e.g.,
`gemara-version`, `assessment-logs`, `steps-executed`,
`confidence-level`). JSON struct tags are added alongside
existing YAML struct tags on the shadow structs (not
replacing them). Embedded go-gemara types already carry
correct JSON tags and require no modification.

#### Scenario: JSON field names are kebab-case
- **GIVEN** a configured workspace with at least one
  policy
- **WHEN** `complyctl scan --log-format json` produces
  a JSON file
- **THEN** top-level fields include `metadata`, `result`,
  `evaluations`, and `target` (kebab-case, not camelCase
  or PascalCase), and `metadata` contains
  `gemara-version`

#### Scenario: JSON output contains expected structure
- **GIVEN** a configured workspace with at least one
  policy
- **WHEN** `complyctl scan --log-format json` produces
  a JSON file
- **THEN** the file deserializes into a structure
  containing `metadata`, `result`, `evaluations`, and
  `target` top-level fields with non-empty values
  matching the scan results

### FR-004: Shell completion for --log-format

`complyctl scan` `--log-format` MUST provide shell
completion enumerating `yaml` and `json`.

#### Scenario: Completion lists valid values
- **GIVEN** the complyctl binary is installed
- **WHEN** a user presses tab after
  `complyctl scan --log-format `
- **THEN** the shell offers `yaml` and `json` as
  completions
