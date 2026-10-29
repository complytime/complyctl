# Spec Delta

## Purpose

Protects provider-facing public APIs from incompatible changes before they reach
a supported release and records policy guidance for a future major migration.

## ADDED Requirements

### Requirement: Release-relative public API comparison
The system MUST compare the provider protobuf contract and Go API against one
baseline: the highest stable release tag in the candidate's major version. It
MUST ignore prerelease tags and report the selected tag and resolved commit.

#### Scenario: Stable baseline is available
- **WHEN** a candidate in a major version has one or more stable release tags
- **THEN** the system SHALL select the highest stable tag in that major version
- **AND** report the selected tag and its resolved commit

#### Scenario: Prerelease tag is newer than a stable release
- **WHEN** the highest tag in the candidate's major version is a prerelease
- **THEN** the system SHALL select the highest stable tag instead

#### Scenario: No stable baseline is available
- **WHEN** no stable release tag exists in the candidate's major version
- **THEN** the system MUST fail with an error that identifies baseline resolution

### Requirement: Fail-closed compatibility analysis
The system MUST evaluate both the `api/plugin` protobuf contract and the
`pkg/provider` exported Go API against the same baseline commit. It MUST fail
when either analysis identifies an incompatible change or cannot complete.

#### Scenario: Compatible public API change
- **WHEN** both analyses complete without incompatible changes
- **THEN** the compatibility check SHALL pass

#### Scenario: Incompatible protobuf or Go API change
- **WHEN** either analysis identifies an incompatible public contract change
- **THEN** the compatibility check MUST fail and report the affected contract

#### Scenario: Analysis cannot complete
- **WHEN** baseline retrieval, tool installation, package loading, or analysis fails
- **THEN** the compatibility check MUST fail and identify the failed operation

### Requirement: Supported local Buf versions
Local protobuf analysis MUST accept stable Buf semantic versions greater
than or equal to `1.50.0`. It MUST validate the complete version before
comparing numeric major, minor, and patch components. Valid build metadata
MUST NOT affect precedence. It MUST reject older, malformed, and
prerelease versions with a tool-setup diagnostic.

Version validation MUST NOT bypass compatibility or operational failures.

#### Scenario: Local Buf meets the minimum
- **GIVEN** a stable Buf version equal to or newer than `1.50.0`
- **WHEN** a contributor runs the protobuf compatibility command
- **THEN** tool-version validation SHALL pass
- **AND** the command MUST run compatibility analysis and preserve its
  success or failure result

#### Scenario: Local Buf is older than the minimum
- **GIVEN** Buf `1.49.0` is installed
- **WHEN** a contributor runs the protobuf compatibility command
- **THEN** the command MUST fail during tool setup
- **AND** the diagnostic MUST include
  `buf >= 1.50.0 required, found 1.49.0`

#### Scenario: Local Buf reports an unsupported version
- **GIVEN** Buf reports a malformed version or a prerelease such as
  `1.73.0-rc.1`
- **WHEN** a contributor runs the protobuf compatibility command
- **THEN** the command MUST fail with a tool-setup diagnostic
- **AND** it MUST NOT run compatibility analysis

#### Scenario: Local Buf cannot report its version
- **GIVEN** Buf is missing or its version command fails
- **WHEN** a contributor runs the protobuf compatibility command
- **THEN** the command MUST fail with a tool-setup diagnostic

### Requirement: Reproducible CI analyzer installation
CI MUST continue installing exactly Buf `1.50.0` for required compatibility
evidence. The Go API analyzer MUST retain its exact pinned module revision
in local and CI execution.

#### Scenario: CI produces compatibility evidence
- **GIVEN** CI evaluates a pull request or supported release-line push
- **WHEN** it installs the compatibility tools
- **THEN** it MUST install exactly Buf `1.50.0`
- **AND** it MUST install the existing exact `apidiff` module revision
- **AND** it MUST run the required compatibility analysis

### Requirement: Merge and release compatibility evidence
The system MUST make successful compatibility analysis available as a required
pull-request and release-preflight check for supported release lines. The check
MUST preserve the compatibility diagnostic output.

#### Scenario: Candidate is evaluated for a supported release line
- **WHEN** a pull request or release candidate targets a supported release line
- **THEN** the system SHALL require successful compatibility analysis before it
  is accepted by the configured merge or release-preflight process

#### Scenario: Compatibility check is unsuccessful
- **WHEN** compatibility analysis reports an incompatible API or operational error
- **THEN** the merge and release-preflight processes MUST treat the check as failed

### Requirement: Current-major compatibility enforcement
The system MUST have no breaking-change override for the current major. An
incompatible API or operational analysis error on `main`, the current supported
release line, MUST fail the compatibility check.

#### Scenario: Incompatible change on the current release line
- **WHEN** an incompatible public API targets `main`
- **THEN** the system MUST reject the candidate

#### Scenario: Future major migration policy
- **WHEN** maintainers approve a future major migration
- **THEN** they MUST propose a separate authorization workflow before enabling
  an exception; that workflow MUST preserve diagnostics, bind approval to the
  reviewed commit, document consumer impact and migration steps, and reject
  operational analysis failures

### Requirement: Compatibility scope and limitations
The system MUST document that static compatibility analysis does not replace
runtime provider integration tests. It MUST document known Go source-compatibility
limits, including unkeyed composite literals and struct comparability, and refer
to downstream provider validation.

#### Scenario: Contributor evaluates a public API change
- **WHEN** a contributor accesses compatibility-check guidance
- **THEN** the guidance SHALL identify the analyzed contracts, limitations, and
  downstream validation responsibility
