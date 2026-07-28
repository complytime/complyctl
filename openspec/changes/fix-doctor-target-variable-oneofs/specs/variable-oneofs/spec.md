## ADDED Requirements

### Requirement: Provider declares optional target variable groups
A provider SHALL be able to declare groups of mutually-exclusive target
variables via `optional_target_variable_groups` in its `DescribeResponse`.
Each group is a pipe-delimited string (e.g., `"url|input_path"`).

#### Scenario: Provider declares one optional group
- **WHEN** the OPA provider's `Describe()` returns
  `optional_target_variable_groups: ["url|input_path"]`
- **THEN** the `DescribeResponse` proto message carries the group to the
  client without loss

#### Scenario: Provider declares no optional groups
- **WHEN** a provider's `Describe()` does not set
  `optional_target_variable_groups`
- **THEN** the field defaults to an empty list and has no effect on
  validation

### Requirement: Doctor validates optional groups as at-least-one
`CheckVariables()` SHALL treat each optional target variable group as
satisfied when at least one member of the group is present in the
target's `Variables` map.

#### Scenario: Target provides one member of an optional group
- **WHEN** doctor checks a target with `input_path` set but `url` absent,
  and the provider declares `optional_target_variable_groups: ["url|input_path"]`
- **THEN** the group counts as resolved and doctor reports StatusPass
  for the variable check (assuming all other required variables are present)

#### Scenario: Target provides no members of an optional group
- **WHEN** doctor checks a target with neither `url` nor `input_path` set,
  and the provider declares `optional_target_variable_groups: ["url|input_path"]`
- **THEN** doctor reports StatusFail with a message indicating the missing
  group

#### Scenario: Target provides both members of an optional group
- **WHEN** doctor checks a target with both `url` and `input_path` set,
  and the provider declares `optional_target_variable_groups: ["url|input_path"]`
- **THEN** the group counts as resolved (doctor does not enforce mutual
  exclusivity; that is the provider's scan-time responsibility)

### Requirement: OPA provider uses optional group for url and input_path
The OPA provider SHALL move `url` and `input_path` from
`RequiredTargetVariables` to `OptionalTargetVariableGroups: ["url|input_path"]`
so that doctor accepts configurations with either variable alone.

#### Scenario: OPA Describe response structure
- **WHEN** the OPA provider responds to a `Describe` RPC
- **THEN** `required_target_variables` does not contain `url` or
  `input_path`, and `optional_target_variable_groups` contains
  `"url|input_path"`
