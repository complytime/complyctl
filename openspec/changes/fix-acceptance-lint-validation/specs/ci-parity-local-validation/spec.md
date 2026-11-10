# Spec Delta

## Purpose

Defines reproducible local validation profiles that distinguish CI-equivalent
style checks from acceptance security analysis without weakening either.

## ADDED Requirements

### Requirement: CI-parity validation profile
The project MUST provide a documented local validation command for each
supported CI-equivalent lint and style check. Each command MUST specify the
tool version, flags, build tags, and input scope that it represents.

#### Scenario: Running a CI-parity profile
- **WHEN** a contributor runs a documented CI-parity validation command
- **THEN** it uses the documented tool version, flags, build tags, and file scope

#### Scenario: CI scope does not include acceptance tests
- **WHEN** the corresponding CI lint check does not enable the acceptance build tag
- **THEN** its CI-parity local command MUST NOT imply that it evaluated acceptance-tagged code

### Requirement: Style-input exclusions are explicit
CI-parity style validation MUST exclude vendored files and ignored generated
reports when those files are outside the corresponding CI style scope. The
exclusions MUST be limited to style validation and MUST NOT disable intended
security scanning.

#### Scenario: Vendored YAML is present
- **WHEN** CI-parity style validation encounters YAML files under `vendor/`
- **THEN** those files are excluded from that style validation profile

#### Scenario: Generated report is present
- **WHEN** CI-parity style validation encounters an ignored generated report
- **THEN** the report is excluded from that style validation profile

### Requirement: Acceptance security analysis remains separate
The project MUST provide a separately named acceptance security-lint command
that evaluates acceptance-tagged code with all configured gosec checks enabled.
Baseline findings MUST be fixed or have a narrowly documented disposition;
the command MUST NOT use blanket suppressions.

#### Scenario: Acceptance security lint runs
- **WHEN** a contributor runs the acceptance security-lint command
- **THEN** acceptance-tagged code is evaluated with the acceptance build tag and configured gosec checks

#### Scenario: A new acceptance security finding is introduced
- **WHEN** acceptance security lint detects a new unsuppressed finding
- **THEN** the command fails and reports the finding

### Requirement: Validation profiles are documented
Contributor documentation MUST distinguish CI-parity validation, broader
pre-commit checks, and acceptance security analysis. It MUST state the
expected scope and purpose of each profile so contributors can select the
appropriate verification command.

#### Scenario: Selecting local verification
- **WHEN** a contributor reads the validation documentation
- **THEN** they can identify which command matches CI style checks and which command evaluates acceptance security
