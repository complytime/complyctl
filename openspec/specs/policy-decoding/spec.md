# Policy Decoding Specification

## Purpose

Accept guidance-only Gemara Policies without weakening typed input
validation or inventing assessments for Policies with no plans.

## Requirements

### Requirement: FR-001 Decode Policies independently of plan count

The shared decoder MUST accept a Gemara Policy with zero assessment
plans, whether absent or explicitly empty. This MUST apply identically
to root and supporting Policy roles in bundle graphs, split-layer
graphs, and root metadata extraction. This supersedes only R39's
nonempty-plan assumption and the corresponding missing-plan rejection
test; Gemara-only decoding without legacy fallback remains required.

#### Scenario: Guidance-only root

- **Given** a Policy that mandates guidance and has no assessment plans
- **When** loading, graph resolution, or metadata extraction runs
- **Then** it MUST succeed with zero assessments
- **And** preserve guidance, catalogs, references, and timeline
- **And** MUST NOT invent a plan or successful evaluation outcome

#### Scenario: Supporting governance Policy

- **Given** a normal root and a zero-plan supporting Policy
- **When** the consumer resolves the bundle
- **Then** it MUST retain the root's assessments and display metadata
- **And** append supporting references in inventory order
- **And** retain declaring-policy evaluator bindings and root-first
  timeline precedence
- **And** imports MUST still supply absent graph defaults

### Requirement: FR-002 Retain minimal input validity checks

The decoder MUST reject malformed YAML, empty/non-Policy documents,
missing or blank Policy IDs, absent or incorrect `metadata.type`, and
mistyped known Policy fields. It MUST use existing Gemara typed decoding
and type checks, without a new full-schema validator or dependency.
Zero plans alone MUST NOT constitute invalid input. Errors MUST retain
policy context and, for imports, the supporting artifact filename.

#### Scenario: Invalid Policy input

- **Given** invalid YAML, an empty document, or a mistyped artifact
- **When** any Policy decoding role runs
- **Then** it MUST fail with context rather than return an empty success

### Requirement: FR-003 Preserve existing execution semantics

The consumer MUST preserve normal assessment projection, per-plan
routing, defaults, metadata totals, and catalog/raw-byte preservation.
Verification MUST report actual zero-assessment scan behavior separately
from successful Policy loading. This correction MUST NOT add scan CLI
behavior, synthetic assessments, or multi-method routing changes.

#### Scenario: Zero-assessment scan observation

- **Given** a successfully resolved graph with no assessment plans
- **When** the existing scan CLI runs
- **Then** verification MUST record its actual exit and report behavior
- **And** MUST NOT claim that successful loading assessed requirements
