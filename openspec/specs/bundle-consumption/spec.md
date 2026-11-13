# Bundle Consumption Specification

## Purpose

Reject ambiguous Policy streams and mislabeled supporting artifacts
while retaining each assessment's declaring-policy evaluator binding.

## Requirements

### Requirement: Single-artifact parsing

Every Policy loading role MUST accept exactly one YAML or JSON document.
An empty or null second document MUST count as an additional document.
Valid zero-plan Policies MUST remain valid.

#### Scenario: Additional Policy document

- **GIVEN** a root, split layer, metadata source or imported Policy
- **WHEN** its stream contains a second document
- **THEN** loading MUST fail with Policy and available filename context

### Requirement: Supporting artifact type validation

ControlCatalog and GuidanceCatalog bytes MUST declare the selected type
before typed projection. Missing catalog IDs MUST retain existing fallback.

#### Scenario: Policy mislabeled as catalog

- **GIVEN** a bundle file labeled ControlCatalog or GuidanceCatalog
- **WHEN** its bytes declare Policy or do not declare an artifact type
- **THEN** graph resolution MUST reject it contextually
- **AND** control metadata extraction MUST reject the wrong catalog

### Requirement: Declaring-policy evaluator bindings

Each assessment MUST retain its declaring Policy's effective evaluator.
The root evaluator MUST NOT replace an imported assessment's binding.
Existing per-plan extraction and timeline precedence remain unchanged.

Superseded: the former root-evaluator override requirement and scenario
are replaced by FR-002 in
`openspec/specs/bundle-artifact-inventory/spec.md`.

#### Scenario: Root and imported evaluators disagree

- **GIVEN** a root with one resolved evaluator and an import with another
- **WHEN** the resolved graph is extracted and grouped for providers
- **THEN** each evaluator MUST receive its declaring Policy's plan

#### Scenario: Root has no whole-graph evaluator

- **GIVEN** a multi-evaluator or zero-plan root without an evaluator
- **WHEN** imports define evaluators
- **THEN** grouping MUST use the individual plans' evaluator settings
- **AND** no first import MUST override other plan routing
