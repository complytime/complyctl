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
