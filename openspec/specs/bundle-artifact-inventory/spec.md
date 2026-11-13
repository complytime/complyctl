# Bundle Artifact Inventory Specification

## Purpose

Preserve complete Gemara bundle inventories and assessment semantics,
and reuse generated artifacts only for equivalent resolved content.

## Requirements

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

The resolver MUST require a Policy source and preserve every supporting
catalog, imported assessment, and mapping reference. Each assessment
MUST retain its declaring Policy's effective evaluator binding, without
root override. Root timeline and display metadata MUST take precedence.
Metadata totals MUST cover the complete inventory, and stale cached
display metadata MUST be refreshed.

#### Scenario: Resolve supporting artifacts

- **Given** multiple catalogs, guidance, and imported Policies
- **When** the resolver builds the dependency graph
- **Then** it MUST retain each catalog and imported assessment
- **And** it MUST report malformed supporting documents with context
- **And** each control or guidance catalog MUST have a separate graph
  entry preserving raw bytes and IDs
- **And** missing catalog IDs MUST use deterministic indexed fallback IDs
- **And** imports MUST contribute their mapping references
- **And** imports MUST supply the timeline only when the root lacks it
- **And** metadata MUST sum controls across all catalogs and assessments
  across the root and imported Policies
- **And** metadata MUST retain the root Policy's display metadata

#### Scenario: Reject a non-Policy source

- **Given** a catalog source and a Policy import
- **When** graph resolution or metadata extraction runs
- **Then** it MUST reject the source instead of using the import as root

### Requirement: FR-003 Identify the resolved Gemara dependency tree

The system MUST derive a deterministic identifier from the resolved
Gemara dependency tree, including imported assessments, catalogs,
effective evaluator bindings, parameters, timeline, and mapping
references. Equivalent resolved content MUST retain its identifier;
ordering that affects effective behavior MUST remain significant.

#### Scenario: Imported assessment changes without an OCI update

- **Given** successful generation for a resolved dependency tree
- **And** unchanged policy and complypack OCI digests
- **When** resolution adds, removes, or changes an imported assessment
- **Then** the resolved tree identifier MUST change
- **And** the next scan MUST regenerate before scanning

#### Scenario: Reuse unchanged resolved content

- **Given** successful generation and generated artifacts on disk
- **And** unchanged complypack digests
- **When** the resolved tree remains equivalent
- **Then** the next scan MUST reuse generated artifacts
- **And** a different root OCI digest alone MUST NOT force generation

#### Scenario: Non-semantic inputs change

- **Given** equivalent resolved Gemara content
- **When** map iteration order, unordered inventory order, YAML
  formatting, workspace variables, targets, complypack content, or
  executable versions change
- **Then** the resolved tree identifier MUST remain unchanged
- **And** independent complypack freshness checks MUST still apply

### Requirement: FR-004 Persist successful tree-based freshness

Explicit generate and scan-triggered generation MUST persist the same
resolved tree identifier only after successful generation. Freshness
MUST compare that identifier instead of a manual projection version
or root policy OCI digest. Changed complypack digests and missing
generated artifacts MUST still trigger regeneration independently.
Existing state without a tree identifier MUST require regeneration.

#### Scenario: Upgrade existing generation state

- **Given** generation state without a resolved tree identifier
- **When** a scan checks generation freshness
- **Then** it MUST regenerate and save the identifier on success
- **And** subsequent unchanged scans MUST reuse generated artifacts

#### Scenario: Preserve stale state on failure

- **Given** generation is required for a changed resolved tree
- **When** identifier calculation or generation fails
- **Then** the operation MUST return an error
- **And** it MUST NOT mark the changed tree successfully generated

#### Scenario: Complypack update remains independent

- **Given** an unchanged resolved tree and prior successful generation
- **When** the local complypack digest changes after a fetch
- **Then** the next scan MUST regenerate before scanning

### Requirement: FR-005 Preserve unambiguous downstream meaning

Reported assessments MUST preserve provider-returned plan IDs and use
requirement fallback only for unique plan mappings. Resolution MUST
preserve typed authored parameters in the tree identifier, separately
from workspace overrides. It MUST reject ambiguous downstream IDs and
conflicting mapping references before returning a graph, with the
conflicting ID and both declaration contexts. Duplicate-plan rejection
MUST remain in effect.

#### Scenario: Distinct plans assess one requirement

- **Given** two unique plans assessing the same requirement
- **When** providers return results containing their plan IDs
- **Then** reports MUST retain each result's original plan ID
- **And** requirement-only results MUST NOT select an arbitrary plan

#### Scenario: Imported evaluator and authored parameters survive

- **Given** root and imported plans with different evaluator bindings
- **And** an imported plan with authored parameter declarations
- **When** resolution, grouping, and tree identification run
- **Then** each plan MUST route to its own effective evaluator
- **And** changing an authored parameter MUST change the tree identifier
- **And** declarations MUST NOT become workspace string overrides

#### Scenario: Ambiguous downstream identifiers

- **Given** catalogs with colliding downstream IDs or Policies with
  conflicting mapping-reference declarations
- **When** resolution runs
- **Then** resolution MUST fail with both declaration contexts
- **And** no partial graph MUST reach generation or reporting
- **And** collisions MUST cover catalog identities, control IDs, and
  requirement IDs in the current unqualified downstream mappings
- **And** errors MUST identify the conflicting ID

#### Scenario: Mapping reference declarations agree

- **Given** equal mapping-reference declarations with the same ID
- **When** resolution assembles the graph
- **Then** it MAY coalesce the equal declarations
- **And** conflicting declarations MUST instead fail with both declaring
  Policy contexts

#### Scenario: Existing identity model remains unchanged

- **Given** valid authored Policies with unique plan IDs
- **When** resolution applies downstream ambiguity checks
- **Then** it MUST NOT introduce policy inheritance, plan replacement,
  tailoring, or a new namespaced provider identity model

#### Scenario: Refresh previously populated metadata

- **Given** cached metadata counting only root Policy assessments
- **And** a cached bundle containing additional imported assessments
- **When** get reuses the cached bundle
- **Then** stored metadata MUST reflect the complete assessment count

#### Scenario: Generated artifacts are missing

- **Given** unchanged resolved tree and complypack digests
- **When** a required evaluator artifact directory is missing or empty
- **Then** the next scan MUST regenerate before scanning
