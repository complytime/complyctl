# YAML Lint Scope Specification

## Purpose

Limits standalone YAML style lint to maintained inputs without
rewriting vendored files or depending on generated reports being absent.

## Requirements

### Requirement: FR-004 Exclude only the two specified YAML inputs

The repository yamllint configuration MUST exclude `vendor/` and
the exact path `governance/reports/evaluation-log.yaml`. It MUST
retain its existing base configuration and formatting rules. This
change MUST NOT add other exclusions or alter security scanners,
CI workflows, or workflow execution policy.

#### Scenario: Vendored YAML violates repository style

- **GIVEN** YAML under `vendor/` violates repository style rules
- **WHEN** standalone yamllint scans the repository
- **THEN** those vendored files do not contribute lint findings

#### Scenario: The behavioral report is regenerated

- **GIVEN** the generated evaluation report exists at the exact
  excluded path and violates repository indentation rules
- **WHEN** standalone yamllint scans the repository
- **THEN** the report does not contribute lint findings
  and need not be deleted or reformatted for lint to pass

### Requirement: FR-005 Retain checks on maintained YAML

Standalone yamllint MUST continue to report violations in maintained
YAML outside the two exclusions. Neither the whole governance tree
nor all Git-ignored files MAY be excluded by this change.

#### Scenario: Maintained YAML contains an indentation error

- **GIVEN** a maintained YAML file outside the exclusions has
  an indentation error
- **WHEN** standalone yamllint checks that file
- **THEN** it reports the error and exits nonzero

#### Scenario: A sibling governance YAML file contains an error

- **GIVEN** a YAML file beside the excluded report has an indentation
  error
- **WHEN** standalone yamllint checks that sibling file
- **THEN** it reports the error and exits nonzero
