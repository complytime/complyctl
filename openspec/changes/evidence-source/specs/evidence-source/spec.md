## Purpose

Enable providers to report evidence provenance via `EvidenceMapping`,
allowing consumers to trace each evidence entry back to the artifact,
location, and content digest from which it was collected.

## ADDED Requirements

### Requirement: Proto API exposes EvidenceMapping source on Evidence

The provider gRPC API SHALL include an `EvidenceMapping` message
with fields `reference_id`, `coordinate`, `entry_id`, `digest`, and
`remarks`. The `Evidence` message SHALL include an optional `source`
field of type `EvidenceMapping`. All fields in `EvidenceMapping`
SHALL be optional (proto3 string defaults). The addition SHALL be
backward-compatible: providers compiled against the prior proto
definition SHALL continue to function without modification.

#### Scenario: Provider sends evidence with source populated

- **WHEN** a provider returns an `Evidence` entry with `source`
  containing `reference_id` and `coordinate`
- **THEN** complyctl SHALL receive the `source` field intact with
  both values preserved

#### Scenario: Provider sends evidence without source (backward compat)

- **WHEN** a provider compiled against the prior proto (without
  `source`) returns an `Evidence` entry
- **THEN** complyctl SHALL receive the evidence with `source` absent
  (nil) and all other fields populated as before

### Requirement: Internal SDK type carries evidence source

The `pkg/provider` package SHALL expose an `EvidenceSource` struct
with fields `ReferenceID`, `Coordinate`, `EntryID`, `Digest`, and
`Remarks` (all strings). The `Evidence` struct SHALL include a
`Source *EvidenceSource` field. When `Source` is nil, no source
provenance is present.

#### Scenario: Round-trip through proto mapping preserves source

- **WHEN** a provider populates `Evidence.Source` with all five
  fields set
- **THEN** the proto-to-internal and internal-to-proto mapping
  functions SHALL preserve all five field values through the
  round-trip

#### Scenario: Nil source maps to absent proto source

- **WHEN** a provider sets `Evidence.Source` to nil
- **THEN** the internal-to-proto mapping SHALL produce an `Evidence`
  message with no `source` field set

### Requirement: Evaluator maps source to gemara EvidenceMapping

The evaluator SHALL map `provider.EvidenceSource` to
`gemara.EvidenceMapping` when constructing evaluation log entries.
When `Evidence.Source` is nil, the evaluator SHALL leave
`gemara.Evidence.Source` at its zero value (empty struct).

#### Scenario: Evidence with source appears in evaluation log

- **WHEN** a provider returns evidence with `Source.ReferenceID`
  set to `"policy-ref"` and `Source.Coordinate` set to
  `"/etc/tls.conf"`
- **THEN** the evaluation log output SHALL contain an evidence
  entry with `source.reference-id: policy-ref` and
  `source.coordinate: /etc/tls.conf`

#### Scenario: Evidence without source omits source in output

- **WHEN** a provider returns evidence with `Source` nil
- **THEN** the evaluation log output SHALL not contain a `source`
  key on that evidence entry

### Requirement: Markdown report renders source provenance

The Markdown formatter SHALL include source provenance metadata
in evidence rendering when `Source` is present. The rendered
output SHALL include at minimum the `reference-id` value. When
`coordinate` is also present, it SHALL be appended. When `Source`
is absent, the Markdown output SHALL render identically to the
current behavior (no source information shown).

#### Scenario: Evidence with source and coordinate in markdown

- **WHEN** an evidence entry has `Source.ReferenceID` of
  `"config-repo"` and `Source.Coordinate` of `"/app/config.yaml"`
- **THEN** the Markdown evidence metadata SHALL include
  `source: config-repo @ /app/config.yaml`

#### Scenario: Evidence with source but no coordinate in markdown

- **WHEN** an evidence entry has `Source.ReferenceID` of
  `"scan-output"` and `Source.Coordinate` is empty
- **THEN** the Markdown evidence metadata SHALL include
  `source: scan-output` without an `@` separator

#### Scenario: Evidence without source in markdown

- **WHEN** an evidence entry has `Source` nil
- **THEN** the Markdown evidence metadata SHALL not include any
  source-related text
