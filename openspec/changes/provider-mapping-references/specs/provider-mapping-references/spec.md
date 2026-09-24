# Spec Delta

## Purpose

Enable providers to declare runtime data sources as MappingReferences
in their ScanResponse, merged with policy-level references into the
EvaluationLog metadata so evidence source reference-ids resolve for
auditors.

## ADDED Requirements

### Requirement: Proto API exposes MappingReference on ScanResponse

The provider gRPC API MUST include a `MappingReference` message with
fields `id`, `title`, `version`, `description`, and `url`, mirroring
`gemara.MappingReference`. The `ScanResponse` message MUST include a
`mapping_references` repeated field of type `MappingReference`. All
fields in `MappingReference` MUST be optional (proto3 string defaults).
The addition MUST be backward-compatible: providers compiled against
the prior proto definition MUST continue to function without
modification.

#### Scenario: Provider sends ScanResponse with mapping references

- **WHEN** a provider returns a `ScanResponse` with
  `mapping_references` containing an entry with `id: "sshd-config"`
  and `title: "SSH daemon configuration"`
- **THEN** complyctl MUST receive the `mapping_references` field
  intact with both values preserved

#### Scenario: Provider sends ScanResponse without mapping references

- **WHEN** a provider compiled against the prior proto (without
  `mapping_references`) returns a `ScanResponse`
- **THEN** complyctl MUST receive the response with
  `mapping_references` absent (empty repeated field) and all other
  fields populated as before

### Requirement: Internal SDK type carries MappingReference

The `pkg/provider` package MUST expose a `MappingReference` struct
with fields `ID`, `Title`, `Version`, `Description`, and `URL` (all
strings). The `ScanResponse` struct MUST include a
`MappingReferences []MappingReference` field. When `MappingReferences`
is nil or empty, no provider-declared references are present.

#### Scenario: Round-trip through proto mapping preserves references

- **WHEN** a provider populates `ScanResponse.MappingReferences`
  with an entry having all five fields set
- **THEN** the proto-to-internal and internal-to-proto mapping
  functions MUST preserve all five field values through the
  round-trip

#### Scenario: Partial fields preserved through round-trip

- **WHEN** a provider populates a `MappingReference` with only
  `ID` and `Title` set (other fields empty)
- **THEN** the round-trip MUST preserve both set values and leave
  the remaining fields as empty strings

#### Scenario: Empty references maps to nil slice

- **WHEN** a provider returns a `ScanResponse` with no
  `mapping_references` entries
- **THEN** the proto-to-internal mapping MUST produce a
  `ScanResponse` with `MappingReferences` nil or empty

### Requirement: Scan pipeline threads provider MappingReferences

The scan pipeline MUST collect `MappingReferences` from each
provider's `ScanResponse` across all targets and evaluator groups.
The collected provider references MUST be passed alongside the
policy-level `DependencyGraph.MappingReferences` to the merge step
before evaluator construction.

#### Scenario: Single provider declares runtime sources

- **WHEN** a single provider returns a `ScanResponse` with
  `MappingReferences` containing `[{id: "sshd-config"}, {id: "fw-config"}]`
- **THEN** both references MUST be available for merging with the
  policy-level references

#### Scenario: Multiple providers declare runtime sources

- **WHEN** two providers (OpenSCAP and OPA) each return
  `ScanResponse` entries with distinct `MappingReferences`
- **THEN** references from both providers MUST be collected and
  available for merging

#### Scenario: Provider returns no mapping references

- **WHEN** a provider returns a `ScanResponse` with no
  `mapping_references` entries
- **THEN** the scan pipeline MUST proceed without error, using
  only the policy-level references (if any)

### Requirement: MappingReferences merged with deduplication

The scan pipeline MUST merge policy-level MappingReferences
(Direction A) and provider-level MappingReferences (Direction B)
into a single deduplicated list before passing to evaluators. The
merged list MUST contain no duplicate `id` values, as the Gemara
CUE schema enforces `_uniqueRefIds` on `mapping-references`.

#### Scenario: No collision between policy and provider references

- **WHEN** the policy declares `[{id: "nist-r5"}]` and the
  provider declares `[{id: "sshd-config"}]`
- **THEN** the merged list MUST contain both references:
  `[{id: "nist-r5"}, {id: "sshd-config"}]`

#### Scenario: ID collision between policy and provider

- **GIVEN** the policy DependencyGraph declares
  `{id: "nist-r5", title: "NIST SP 800-53"}` and a provider scan
  completes
- **WHEN** the provider's ScanResponse also declares
  `{id: "nist-r5", title: "NIST 800-53 R5.1.1"}`
- **THEN** the merged list MUST retain the policy entry and
  discard the provider entry (policy is the authoritative source
  for catalog-type references)

#### Scenario: ID collision between two providers

- **GIVEN** providers are processed in ascending evaluator-ID order
- **WHEN** provider with evaluator-id `ampel` declares
  `{id: "system-config", title: "System Files"}` and provider with
  evaluator-id `openscap` declares
  `{id: "system-config", title: "K8s ConfigMaps"}`
- **THEN** the merged list MUST retain the entry from evaluator
  `ampel` (alphabetically first) and discard the entry from
  evaluator `openscap` (provider-to-provider collision is an
  authoring mistake, not a production scenario)

#### Scenario: Same provider declares duplicate reference IDs

- **WHEN** a single provider returns two MappingReferences both
  with `id: "config"` in one ScanResponse
- **THEN** the merge function MUST deduplicate them with the same
  collision warning channels as cross-provider collisions

#### Scenario: Provider sends MappingReference with empty id

- **WHEN** a provider returns a MappingReference with `id: ""`
- **THEN** the merge function MUST discard the entry and emit a
  structured `logger.Warn()` entry noting the empty-ID reference
  was skipped

### Requirement: Collision warnings reported to operator

When a MappingReference ID collision is detected during merge, the
system MUST report the collision through three channels:

1. A stderr WARNING message following the existing
   `FormatOperationalWarnings()` pattern, listing each collision
   with the retained and discarded entries.
2. A structured `logger.Warn()` entry with key-value metadata
   (collision id, retained title, discarded title).
3. A `Description` annotation on the retained `MappingReference`
   in the EvaluationLog documenting what was discarded, making the
   collision auditable in the serialized output.

When no collisions occur, no warnings MUST be emitted and no
`Description` annotations MUST be added.

#### Scenario: Policy-provider collision produces stderr warning

- **GIVEN** the policy and provider both declare `id: "nist-r5"`
- **WHEN** the merge function processes both sources
- **THEN** stderr MUST contain a WARNING line following the
  `FormatOperationalWarnings()` block format, identifying the
  collision, the retained entry (policy), and the discarded entry
  (provider)

#### Scenario: Collision annotates retained MappingReference

- **GIVEN** the policy declares
  `{id: "nist-r5", title: "NIST SP 800-53"}` and the provider
  declares `{id: "nist-r5", title: "NIST R5.1.1"}`
- **WHEN** the merge function resolves the collision
- **THEN** the retained MappingReference in the EvaluationLog MUST
  have its `description` field prepended with text documenting the
  discarded provider entry, preserving any existing description
  content. The annotation MUST include the discarded entry's `id`
  and `title` (if non-empty), and identify the source that was
  discarded (e.g., "provider").

#### Scenario: Collision preserves existing description

- **GIVEN** the policy declares
  `{id: "x", description: "Existing text"}` and the provider
  declares `{id: "x", title: "Other"}`
- **WHEN** the merge function resolves the collision
- **THEN** the retained MappingReference description MUST contain
  both the collision annotation AND "Existing text"

#### Scenario: No collision produces no warnings

- **WHEN** the policy and provider declare references with distinct
  IDs
- **THEN** the system MUST NOT emit any collision warnings to
  stderr and MUST NOT add `description` annotations to any
  MappingReference

### Requirement: Merged references populate EvaluationLog metadata

The evaluator MUST populate `EvaluationLog.Metadata.MappingReferences`
with the merged list of policy-level and provider-level references.
When neither source provides any references, the field MUST be nil
(absent from serialized output via `omitempty` tags).

#### Scenario: Both directions populate EvaluationLog

- **WHEN** the policy declares `[{id: "nist-r5"}]` and the
  provider declares `[{id: "sshd-config"}]`
- **THEN** the EvaluationLog YAML output MUST contain:
  ```yaml
  metadata:
    mapping-references:
      - id: nist-r5
      - id: sshd-config
  ```

#### Scenario: Only policy references populate EvaluationLog

- **WHEN** the policy declares mapping references and the provider
  declares none
- **THEN** the EvaluationLog MUST contain only the policy references
  (identical to current Direction A behavior)

#### Scenario: Only provider references populate EvaluationLog

- **WHEN** the policy declares no mapping references and the
  provider declares `[{id: "sshd-config"}]`
- **THEN** the EvaluationLog MUST contain only the provider
  references

#### Scenario: No references from either source

- **WHEN** neither the policy nor the provider declares any
  mapping references
- **THEN** the EvaluationLog MUST NOT contain a
  `mapping-references` key in the `metadata` block
