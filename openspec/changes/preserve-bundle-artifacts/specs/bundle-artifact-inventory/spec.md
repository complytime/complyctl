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
Each assessment MUST retain its declaring Policy's effective evaluator
binding. A root evaluator MUST NOT replace an imported assessment's
binding. The root timeline MUST take precedence; imports MUST supply
it only when absent. Metadata MUST sum controls across all catalogs
and assessments across the root and imported Policies while retaining
root Policy display metadata. Cached display metadata MUST be refreshed
when it no longer matches the resolved inventory.

#### Scenario: Resolve supporting artifacts

- **Given** multiple catalogs, guidance, and imported Policies
- **When** the resolver builds the dependency graph
- **Then** it MUST retain each catalog and imported assessment
- **And** it MUST report malformed supporting documents with context

#### Scenario: Reject a non-Policy source

- **Given** a catalog source and a Policy import
- **When** graph resolution or metadata extraction runs
- **Then** it MUST reject the source instead of using the import as root

### Requirement: FR-003 Identify the resolved Gemara dependency tree

The system MUST derive a deterministic content identifier from the
resolved Gemara dependency tree, including imported assessments,
catalogs, effective evaluator bindings, parameters, timeline, and
mapping references. It MUST exclude workspace variables, targets,
complypack content, and executable versions from this identifier.
Equivalent resolved content MUST retain its identifier despite map
iteration order, unordered inventory order, or YAML formatting.
Ordering that affects effective behavior MUST remain significant.

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

Reported assessments MUST preserve provider-returned plan IDs. A
requirement-only result MAY recover a plan ID only when the requirement
identifies exactly one plan. Resolution MUST preserve authored Gemara
parameter declarations in their SDK types and include them in the
resolved tree identifier, separately from workspace parameter overrides.

Before returning a graph, resolution MUST reject catalog identities,
control IDs, or requirement IDs that collide in the current unqualified
downstream mappings. Errors MUST identify the conflicting ID and both
declarations. Equal mapping-reference declarations MAY be coalesced;
conflicting declarations with the same reference ID MUST be rejected
with both declaring Policy contexts.

Existing duplicate-plan rejection MUST remain in effect. These checks
MUST NOT introduce policy inheritance, plan replacement, tailoring, or
a new namespaced provider identity model.

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

#### Scenario: Refresh previously populated metadata

- **Given** cached metadata counting only root Policy assessments
- **And** a cached bundle containing additional imported assessments
- **When** get reuses the cached bundle
- **Then** stored metadata MUST reflect the complete assessment count

#### Scenario: Generated artifacts are missing

- **Given** unchanged resolved tree and complypack digests
- **When** a required evaluator artifact directory is missing or empty
- **Then** the next scan MUST regenerate before scanning
