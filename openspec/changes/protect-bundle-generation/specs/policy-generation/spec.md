# Policy generation requirements

## ADDED Requirements

### Requirement: FR-001 Unambiguous assessment plan IDs

The resolver MUST reject duplicate plan IDs before returning a graph.
This MUST cover root/import, import/import, and internal duplicates in
bundle and split-layer Policies, even with equal requirement IDs or
different evaluators. Errors MUST name the conflicting plan ID and both
declaring artifacts and requirements. No partial graph may escape.
Unique authored IDs MUST remain unchanged. Existing root evaluator
precedence, multi-artifact inventory, and valid zero-plan roots and
imports MUST remain supported.

#### Scenario: Independently authored Policies reuse a plan ID

- **Given** two declarations with the same plan ID
- **When** the resolver assembles the Policy graph
- **Then** resolution MUST fail with both declaration contexts
- **And** generation and result mapping MUST NOT receive that graph

#### Scenario: A single valid Policy has unique plan IDs

- **Given** a valid Policy with unique plans or no plans
- **When** the resolver assembles its graph
- **Then** the existing projection and evaluator routing MUST persist

### Requirement: FR-002 Projection-aware generation freshness

Generation state MUST record the current projection version after
successful generation. Missing or mismatched versions MUST be stale,
even when policy and complypack digests match and artifacts exist.
Persisted current-version state MUST remain fresh when digests match
and required artifacts exist. Existing digest invalidation, missing
artifact checks, and generation failure behavior MUST persist.

#### Scenario: Upgrade from unversioned generation state

- **Given** persisted unversioned state, unchanged policy and complypack
  digests, and existing nonempty evaluator directories
- **When** scan checks whether to generate
- **Then** it MUST require generation with the current resolved plans
- **And** successful generation MUST persist current-version state
- **And** the next unchanged scan MUST reuse that generated state
