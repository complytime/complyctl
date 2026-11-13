# Spec Delta

## Purpose

Protect the full provider-to-OSCAL scan pipeline with acceptance checks
that detect lost evidence and invalid Assessment Results documents.

## ADDED Requirements

### Requirement: FR-001 Provider evidence survives the scan workflow

The acceptance suite MUST exercise the CLI and provider workflow and
verify that the expected assessment observation in the generated
Assessment Results contains nonempty `relevant-evidence`. The evidence
description and populated source provenance MUST match the fixture.

#### Scenario: Provider returns evidence with provenance

- **GIVEN** a configured policy and provider with known evidence
- **WHEN** the suite fetches the policy, generates assessments, and
  scans the target with OSCAL output enabled
- **THEN** the expected assessment observation contains the expected
  evidence description, reference ID, coordinate, and digest
- **AND** assertions do not depend on observation order or generated
  UUID and timestamp values

#### Scenario: Evidence disappears but observations remain

- **GIVEN** a report with the expected observations and findings
- **WHEN** the expected observation has no `relevant-evidence`, an
  empty array, or evidence whose expected values do not match
- **THEN** the evidence acceptance check fails

### Requirement: FR-002 Findings link to evidence-bearing observations

The acceptance suite MUST verify that the expected control finding
references the evidence-bearing observation for its assessment by UUID.
Evidence on an unrelated observation MUST NOT satisfy that check.

#### Scenario: Expected finding references the assessment observation

- **GIVEN** a report containing the expected control and assessment
- **WHEN** the suite checks the finding's related observations
- **THEN** a referenced UUID resolves to the expected assessment
  observation containing the fixture evidence

#### Scenario: Evidence exists only on an unrelated observation

- **GIVEN** a report with evidence on another assessment observation
- **WHEN** the expected finding does not reference the expected
  evidence-bearing assessment observation
- **THEN** the linkage acceptance check fails

### Requirement: FR-003 Evidence-bearing output is schema-valid

The acceptance suite MUST validate the complete generated Assessment
Results document against OSCAL 1.1.3. Schema validity MUST be checked
alongside evidence assertions because schema-valid output can omit
optional evidence.

#### Scenario: Valid report contains expected evidence

- **GIVEN** a full-scan report containing the fixture evidence
- **WHEN** the suite validates the complete document
- **THEN** OSCAL 1.1.3 schema validation succeeds
- **AND** the suite independently verifies evidence and linkage

#### Scenario: Report contains schema-invalid evidence

- **GIVEN** an Assessment Results document containing evidence
- **WHEN** the evidence violates the OSCAL 1.1.3 schema
- **THEN** the schema acceptance check fails
