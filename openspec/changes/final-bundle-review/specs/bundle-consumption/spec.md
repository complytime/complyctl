## ADDED Requirements

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

### Requirement: Importing evaluator precedence

The root Policy's resolved evaluator setting MUST take precedence over
imported settings. Without a root whole-graph evaluator, imported plans
MUST retain their individual routing without promoting an imported
evaluator into a whole-graph shortcut. Existing per-plan extraction and
timeline precedence MUST remain unchanged.

#### Scenario: Root and imported evaluators disagree

- **GIVEN** a root with one resolved evaluator and an import with another
- **WHEN** the resolved graph is extracted and grouped for providers
- **THEN** the root evaluator MUST receive both plans

#### Scenario: Root has no whole-graph evaluator

- **GIVEN** a multi-evaluator or zero-plan root without an evaluator
- **WHEN** imports define evaluators
- **THEN** grouping MUST use the individual plans' evaluator settings
- **AND** no first import MUST override other plan routing
