# Policy generation requirements

## ADDED Requirements

### Requirement: FR-001 Unambiguous assessment plan IDs

The resolver MUST reject duplicate plan IDs before returning a graph.
This MUST cover root/import, import/import, and internal duplicates in
bundle and split-layer Policies, even with equal requirement IDs or
different evaluators. Errors MUST name the conflicting plan ID and both
declaring artifacts and requirements. No partial graph may escape.
Unique authored IDs MUST remain unchanged. Per-assessment effective
evaluator bindings, multi-artifact inventory, and valid zero-plan roots
and imports MUST remain supported.

#### Scenario: Independently authored Policies reuse a plan ID

- **Given** two declarations with the same plan ID
- **When** the resolver assembles the Policy graph
- **Then** resolution MUST fail with both declaration contexts
- **And** generation and result mapping MUST NOT receive that graph

#### Scenario: A single valid Policy has unique plan IDs

- **Given** a valid Policy with unique plans or no plans
- **When** the resolver assembles its graph
- **Then** the existing projection and evaluator routing MUST persist

### Requirement: FR-002 Resolved-tree generation freshness

Generation state MUST record the resolved tree identifier after
successful generation. Missing or different identifiers MUST be stale.
Equal identifiers and complypack digests MUST remain fresh when required
artifacts exist. A policy OCI digest change alone MUST NOT invalidate
generation. Missing-artifact and failure behavior MUST persist.

The original projection-version requirement is superseded by FR-003
and FR-004 in
`preserve-bundle-artifacts/specs/bundle-artifact-inventory/spec.md`.

#### Scenario: Upgrade from unversioned generation state

- **Given** persisted unversioned state, unchanged policy and complypack
  digests, and existing nonempty evaluator directories
- **When** scan checks whether to generate
- **Then** it MUST require generation with the current resolved plans
- **And** successful generation MUST persist the resolved tree identifier
- **And** the next unchanged scan MUST reuse that generated state
