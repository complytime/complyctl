# Spec Delta

## Purpose

Keep project formatting and license-header hooks on maintained inputs
without rewriting third-party sources or hiding first-party defects.

## ADDED Requirements

### Requirement: FR-001 Preserve vendored content

The `trailing-whitespace`, `end-of-file-fixer`, and `check-spdx`
hooks MUST exclude the repository-root `vendor/` tree. Each MUST
leave vendored file bytes and license notices unchanged for both
staged-file and repository-wide runs at its configured stage.

#### Scenario: Staged vendored files violate project conventions

- **GIVEN** staged vendored files contain trailing whitespace,
  invalid EOF formatting, and non-Apache license notices
- **WHEN** each affected hook runs at its configured stage
- **THEN** those files produce no findings from these hooks
- **AND** their bytes and license notices remain unchanged

#### Scenario: Repository-wide selection includes vendor files

- **GIVEN** tracked vendored files violate project conventions
- **WHEN** each affected hook runs with all-files selection
- **THEN** those files remain unchanged and excluded from findings

### Requirement: FR-002 Retain first-party formatting checks

Whitespace and EOF hooks MUST retain their existing checks and
failure behavior on applicable first-party files except `vendor/`
and the three scaffolded-command exceptions in FR-008. No other
first-party exemption is introduced. Other commands and
archived specs MUST retain existing formatting checks.
Scope exclusions MUST NOT hide files merely because another path
component or filename contains the string `vendor`.

#### Scenario: Maintained files have formatting defects

- **GIVEN** applicable first-party files have trailing whitespace
  or invalid EOF formatting
- **WHEN** the corresponding formatting hook runs at pre-push
- **THEN** it reports failure and applies its existing correction

#### Scenario: Vendor-like first-party paths have defects

- **GIVEN** first-party paths such as `tests/vendor_cases/` or
  `internal/vendor_helpers.go` contain formatting defects
- **WHEN** the relevant formatting hook selects those paths
- **THEN** it detects the defects rather than excluding the paths

#### Scenario: Compliant maintained files are checked

- **GIVEN** applicable first-party files satisfy existing rules
- **WHEN** the formatting hooks run at pre-push
- **THEN** they pass without changing the files

### Requirement: FR-003 Retain first-party SPDX checks

The SPDX hook MUST continue to check applicable first-party Go,
Python, and shell files at pre-commit. It MUST preserve the existing
Apache-2.0 header policy and shebang placement behavior. Scope fixes
MUST NOT change the license-header policy for maintained sources.

#### Scenario: First-party files lack the project header

- **GIVEN** applicable first-party files lack the required header
- **WHEN** the SPDX hook runs at pre-commit
- **THEN** it reports failure and adds the project header
- **AND** any shebang remains the first line

#### Scenario: First-party files already carry the project header

- **GIVEN** applicable first-party files carry compliant headers
- **WHEN** the SPDX hook runs at pre-commit
- **THEN** it passes without changing file bytes

### Requirement: FR-004 Preserve stages and file selection

The affected hooks MUST retain their stage assignments: formatting
at pre-push and SPDX at pre-commit. Staged-file runs MUST check
eligible staged content; all-files runs MUST also check eligible
tracked content outside the staged selection.

#### Scenario: A defect is outside the staged selection

- **GIVEN** a compliant staged file and a separate committed file
  with a defect applicable to the selected hook
- **WHEN** the hook runs first on staged files and then on all files
  at its configured stage
- **THEN** the staged-file run passes without changing the other file
- **AND** the all-files run detects the committed defect

#### Scenario: A hook is invoked at a different stage

- **GIVEN** a hook's existing stage assignment
- **WHEN** a contributor invokes it at an unconfigured stage
- **THEN** this change does not broaden that assignment
- **AND** a stage-filtered skip does not establish verification success

### Requirement: FR-005 Preserve other controls

This change MUST NOT add a global vendor exclusion, disable hooks,
alter hook revisions or rules, or change unrelated hook selection.
Applicable security scanning MUST retain its existing scope and
configuration.

#### Scenario: An unrelated check would select an input

- **GIVEN** an input selected by an unrelated hook or scanner
- **WHEN** the three hook-specific scope exclusions take effect
- **THEN** that unrelated control retains its existing selection

### Requirement: FR-006 Account for first-party baseline findings

Verification MUST distinguish baseline findings from introduced
defects using a recorded revision and tool versions. Each remaining
first-party finding MUST be fixed within the approved scope or
tracked with its exact path, hook, reason, and follow-up disposition.
For the three command exceptions in FR-008, the inventory MUST
distinguish exclusions from repairs and retain their dispositions.
Historical issue counts MUST NOT define the expected total.

#### Scenario: A first-party finding is outside the cleanup scope

- **GIVEN** baseline verification identifies a protected template
  outside the FR-008 exceptions or another out-of-scope file
- **WHEN** verification results are recorded
- **THEN** the finding remains explicitly tracked under #972
- **AND** no broader exclusion or unauthorized edit hides it

#### Scenario: A scaffolded command has a baseline defect

- **GIVEN** an FR-008 command has a baseline formatter defect
- **WHEN** candidate verification excludes that path
- **THEN** the inventory retains the path, hook, and original defect
- **AND** its disposition MUST state "excluded from formatters;
  preserved uf-scaffolded bytes" with a source-template follow-up
- **AND** the inventory MUST NOT count the exclusion as a repair

### Requirement: FR-007 Provide repeatable contributor verification

Contributor guidance MUST state affected hook scope, stage-specific
execution, prerequisites, and how to run regression checks. The
guidance MUST list the exact FR-008 formatter exceptions and
distinguish them from the vendor-only SPDX exclusion. The
regression checks MUST use isolated normal Git repositories, fail
on missing prerequisites, and verify unchanged vendored bytes and
continued first-party defect detection.

#### Scenario: A contributor runs the regression suite

- **GIVEN** the documented prerequisites and hook environments
- **WHEN** the contributor runs the documented test target
- **THEN** the checks exercise actual hook behavior
- **AND** a missing tool or unavailable hook environment fails visibly
- **AND** a second run on corrected inputs passes unchanged

### Requirement: FR-008 Preserve three scaffolded commands

The whitespace and EOF hooks MUST exclude exactly these additional
repository-root paths, preserving their bytes under staged-file
and all-files selection at pre-push:

- `.opencode/commands/uf.review-council.md`
- `.opencode/commands/uf.review-pr.md`
- `.opencode/commands/uf.address-feedback.md`

This exception MUST NOT broaden the SPDX exclusion, affect other
hooks, or exclude an entire command or archive directory.

#### Scenario: An exempt scaffolded command has formatting defects

- **GIVEN** any listed command contains trailing whitespace or an
  invalid EOF and an eligible control proves hook execution
- **WHEN** either formatter runs at pre-push on staged or all files
- **THEN** the listed command produces no formatter finding
- **AND** its exact bytes remain unchanged
- **AND** regression checks MUST exercise every listed command
  with each formatter and each selection mode

#### Scenario: A neighboring or lookalike path has a defect

- **GIVEN** a different command, prefix/suffix lookalike, nested
  copy, or archived spec contains an applicable formatting defect
- **WHEN** the whitespace or EOF hook selects it at pre-push
- **THEN** the hook reports failure and applies its usual correction
- **AND** regression checks MUST cover neighboring commands,
  prefix/suffix lookalikes, nested copies, and archived specs

#### Scenario: The command exemption is removed independently

- **GIVEN** a disposable configuration retains the vendor exclusion
  but removes only the scaffolded-command exemption
- **WHEN** the command-preservation cases run for each formatter
- **THEN** those cases fail because the defective commands change
- **AND** the vendor-preservation cases still pass

#### Scenario: The vendor exemption is removed independently

- **GIVEN** a disposable formatter configuration retains the three
  command exceptions but removes only its vendor exclusion
- **WHEN** the vendor and command preservation cases run
- **THEN** vendor-preservation cases fail
- **AND** command-preservation cases still pass
