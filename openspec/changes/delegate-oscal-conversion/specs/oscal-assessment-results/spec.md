# Spec Delta

## Purpose

Produce interoperable OSCAL assessment-results reports that retain
evaluation evidence and link each finding to its underlying observations.

## ADDED Requirements

### Requirement: Evidence-preserving OSCAL assessment results
The system MUST write a valid OSCAL 1.1.3 assessment-results document for an
evaluation log. Each assessment that contains evidence MUST produce an
observation with `relevant-evidence` entries that retain the evidence
description and available source provenance.

#### Scenario: Assessment contains evidence
- **WHEN** an assessment log contains one or more evidence entries
- **THEN** its OSCAL observation contains a corresponding
  `relevant-evidence` entry for each item
- **AND** each entry retains its description and populated source fields

#### Scenario: Assessment has no evidence
- **WHEN** an assessment log contains no evidence entries
- **THEN** the OSCAL report remains valid without `relevant-evidence` for
  that observation

### Requirement: Findings relate to assessment observations
The system MUST represent each control evaluation as an OSCAL finding and
MUST link that finding to the observations produced for its assessment logs.

#### Scenario: Control has multiple assessments
- **WHEN** a control evaluation contains multiple assessment logs
- **THEN** the OSCAL report contains one finding for the control
- **AND** the finding links to every observation for that control

### Requirement: Stable report delivery
The system MUST preserve the OSCAL report delivery contract: it MUST use the
configured output directory, create it when absent, write the report with
owner-only file permissions, and use a policy- and target-specific filename.
The report MUST reference the policy-specific assessment plan.

#### Scenario: Separate scan targets
- **WHEN** the system writes OSCAL reports for the same policy and two
  different targets
- **THEN** it writes distinct report paths

#### Scenario: Missing output directory
- **WHEN** the configured OSCAL output directory does not exist
- **THEN** the system creates the directory and writes the report there

### Requirement: Upstream OSCAL result semantics
The system MUST use the result status mappings and document structure
provided by the pinned Gemara OSCAL assessment-results converter. The system
MUST NOT apply local result remapping or tailoring-specific finding remarks.

#### Scenario: Non-passing result
- **WHEN** an evaluation has a non-passing Gemara result
- **THEN** its OSCAL status and reason match the pinned Gemara converter's
  output for that result
