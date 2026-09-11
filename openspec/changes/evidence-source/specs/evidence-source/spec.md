## Purpose

Enable providers to report evidence provenance via `EvidenceMapping`,
allowing consumers to trace each evidence entry back to the artifact,
location, and content digest from which it was collected.

## ADDED Requirements

### Requirement: Proto API exposes EvidenceMapping source on Evidence

The provider gRPC API MUST include an `EvidenceMapping` message
with fields `reference_id`, `coordinate`, `entry_id`, `digest`, and
`remarks`. The `Evidence` message MUST include an optional `source`
field of type `EvidenceMapping`. All fields in `EvidenceMapping`
MUST be optional (proto3 string defaults). The addition MUST be
backward-compatible: providers compiled against the prior proto
definition MUST continue to function without modification.

#### Scenario: Provider sends evidence with source populated

- **WHEN** a provider returns an `Evidence` entry with `source`
  containing `reference_id` and `coordinate`
- **THEN** complyctl MUST receive the `source` field intact with
  both values preserved

#### Scenario: Provider sends evidence without source (backward compat)

- **WHEN** a provider compiled against the prior proto (without
  `source`) returns an `Evidence` entry
- **THEN** complyctl MUST receive the evidence with `source` absent
  (nil) and all other fields populated as before

### Requirement: Internal SDK type carries evidence source

The `pkg/provider` package MUST expose an `EvidenceSource` struct
with fields `ReferenceID`, `Coordinate`, `EntryID`, `Digest`, and
`Remarks` (all strings). The `Evidence` struct MUST include a
`Source *EvidenceSource` field. When `Source` is nil, no source
provenance is present.

#### Scenario: Round-trip through proto mapping preserves source

- **WHEN** a provider populates `Evidence.Source` with all five
  fields set
- **THEN** the proto-to-internal and internal-to-proto mapping
  functions MUST preserve all five field values through the
  round-trip

#### Scenario: Partial source fields preserved through round-trip

- **WHEN** a provider populates `Evidence.Source` with only
  `ReferenceID` and `Coordinate` set (other fields empty)
- **THEN** the round-trip MUST preserve both set values and
  leave the remaining fields as empty strings

#### Scenario: Nil source maps to absent proto source

- **WHEN** a provider sets `Evidence.Source` to nil
- **THEN** the internal-to-proto mapping MUST produce an `Evidence`
  message with no `source` field set

#### Scenario: Both coordinate and entry_id passed through

- **WHEN** a provider sets both `Source.Coordinate` and
  `Source.EntryID` on an evidence entry
- **THEN** complyctl MUST pass both values through without
  modification (the Gemara schema documents these as mutually
  exclusive, but enforcement is the provider's responsibility)

### Requirement: Evaluator maps source to gemara EvidenceMapping

The evaluator MUST map `provider.EvidenceSource` to
`gemara.EvidenceMapping` when constructing evaluation log entries.
When `Evidence.Source` is nil, the evaluator MUST NOT set
`gemara.Evidence.Source`, leaving it at its zero value.

#### Scenario: Evidence with source appears in evaluation log

- **WHEN** a provider returns evidence with `Source.ReferenceID`
  set to `"policy-ref"` and `Source.Coordinate` set to
  `"/etc/tls.conf"`
- **THEN** the evaluation log output MUST contain an evidence
  entry with `source.reference-id: policy-ref` and
  `source.coordinate: /etc/tls.conf`

#### Scenario: Evidence without source omits source in YAML output

- **WHEN** a provider returns evidence with `Source` nil
- **THEN** the default YAML evaluation log output MUST NOT contain
  a `source` key on that evidence entry (goccy/go-yaml omits
  zero-value structs with `omitempty`; JSON output may include an
  empty `"source": {}` due to encoding/json behavior -- this is
  a pre-existing upstream serialization characteristic)

### Requirement: Markdown report renders source provenance

The Markdown formatter MUST include source provenance metadata
in evidence rendering when `Source` is present. The rendered
output MUST include at minimum the `reference-id` value. When
`coordinate` is also present, it MUST be appended. When `Source`
is absent, the Markdown output MUST render identically to the
current behavior (no source information shown).

#### Scenario: Evidence with source and coordinate in markdown

- **WHEN** an evidence entry has `Source.ReferenceID` of
  `"config-repo"` and `Source.Coordinate` of `"/app/config.yaml"`
- **THEN** the Markdown evidence metadata MUST include
  `source: config-repo @ /app/config.yaml`

#### Scenario: Evidence with source but no coordinate in markdown

- **WHEN** an evidence entry has `Source.ReferenceID` of
  `"scan-output"` and `Source.Coordinate` is empty
- **THEN** the Markdown evidence metadata MUST include
  `source: scan-output` without an `@` separator

#### Scenario: Evidence without source in markdown

- **WHEN** an evidence entry has `Source` nil
- **THEN** the Markdown evidence metadata MUST not include any
  source-related text
