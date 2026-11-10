## ADDED Requirements

### Requirement: FR-001 Preserve the bundle inventory

The loader MUST return the source and every import with names, raw
bytes, order, and types intact. It MUST recover missing file types from
the SDK manifest. Type filters MUST retain every matching artifact.
`LoadBundleFiles` MUST retain its first-per-type behavior and input
validation.

#### Scenario: Repeated artifact types

- **Given** a root Policy, two control catalogs, and an imported Policy
- **When** the loader unpacks the standard SDK bundle
- **Then** the inventory MUST retain all four artifacts
- **And** the compatibility wrapper MUST return the first of each type

### Requirement: FR-002 Resolve the complete inventory

The resolver MUST require a Policy source and emit a separate graph
entry per control or guidance catalog, preserving raw bytes and IDs.
Missing catalog IDs MUST use deterministic indexed fallback IDs.
Imported Policies MUST contribute assessments and mapping references.
The root evaluator and timeline MUST take precedence; imports MUST
supply them only when absent. Metadata MUST sum controls across all
catalogs while retaining root Policy display metadata.

#### Scenario: Resolve supporting artifacts

- **Given** multiple catalogs, guidance, and imported Policies
- **When** the resolver builds the dependency graph
- **Then** it MUST retain each catalog and imported assessment
- **And** it MUST report malformed supporting documents with context

#### Scenario: Reject a non-Policy source

- **Given** a catalog source and a Policy import
- **When** graph resolution or metadata extraction runs
- **Then** it MUST reject the source instead of using the import as root
