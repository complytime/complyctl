## Context

See proposal.md for motivation. The Gemara CUE schema defines
`#Evidence` with a `source` field of type `#EvidenceMapping`
(in `mapping_inline.cue`). The go-gemara SDK already exposes
this as `Evidence.Source` of type `EvidenceMapping` with fields
`ReferenceId`, `Coordinate`, `EntryId`, `Digest`, and `Remarks`
(see `vendor/.../go-gemara/generated_types.go:645-663`).

complyctl's proto API (`api/plugin/plugin.proto:148`) currently
has an `Evidence` message with five fields and no `source`. The
proto comment explicitly references "ADR 0023" for future
extension, but ADR 0023 was never written.

The pipeline has four conversion boundaries:

1. Provider -> proto (`internalEvidenceToProto`, `server.go:107`)
2. Proto -> internal (`protoEvidenceToInternal`, `client.go:263`)
3. Internal -> gemara (`evaluator.go:218`)
4. Gemara -> output (YAML/JSON/Markdown)

All four boundaries need the `source` field added.

## Goals / Non-Goals

**Goals:**

- Full alignment with go-gemara `Evidence.Source` field
- Backward-compatible proto change (additive field)
- Round-trip fidelity for all five `EvidenceMapping` subfields
- Evidence source rendered in Markdown reports
- Evidence source serialized in YAML/JSON evaluation logs

**Non-Goals:**

- OSCAL output evidence support (separate concern, no upstream
  OSCAL mapping for Gemara evidence exists yet)
- `Payload` type widening from `bytes` to `any` (breaking proto
  change, not needed for source provenance)
- Provider-side implementation in complytime-providers (tracked
  via separate issue)

## Decisions

### D1: New proto message vs. flattened fields

**Decision**: Add a new `EvidenceMapping` proto message and
reference it as a sub-message on `Evidence`.

**Rationale**: Mirrors the go-gemara struct hierarchy. A
sub-message naturally represents "optional struct" in proto3
(nil when absent vs. empty when present). Flattening the five
fields directly onto `Evidence` would lose the grouping
semantics and make future extensions harder.

**Alternative considered**: Flatten `reference_id`, `coordinate`,
`entry_id`, `digest`, `remarks` directly onto `Evidence` as
fields 6-10. Rejected because it obscures the semantic grouping
and diverges from go-gemara's type hierarchy.

### D2: Internal Go type naming

**Decision**: Name the internal type `EvidenceSource` (not
`EvidenceMapping`) to avoid confusion with the go-gemara type
of the same name.

**Rationale**: `pkg/provider/` types are the provider-facing
SDK. Using `EvidenceSource` makes the intent clear ("where did
this evidence come from?") without requiring providers to know
about Gemara's mapping model. The mapping between
`provider.EvidenceSource` and `gemara.EvidenceMapping` happens
internally in the evaluator.

**Alternative considered**: Reuse `EvidenceMapping` as the name.
Rejected because it would create import ambiguity when both
`provider` and `gemara` packages are used in the same file.

### D3: Pointer vs. value for Source field

**Decision**: `Source *EvidenceSource` (pointer) on the internal
`Evidence` type.

**Rationale**: Nil clearly signals "no source provided" vs.
zero-value struct with all empty strings. This matches proto3
semantics where an absent sub-message is nil. The evaluator
checks `if ev.Source != nil` before mapping.

### D4: Remove stale ADR 0023 reference

**Decision**: Remove the "ADR 0023" comment from `plugin.proto`
since this change implements the deferred extension directly.

**Rationale**: The comment references a document that was never
written. Leaving it after the `source` field is added would be
misleading. The proto comment on the `Evidence` message will be
updated to reference the Gemara `#Evidence` schema directly.

## Risks / Trade-offs

**[Proto regeneration]** Adding a new message and field requires
`make proto` (buf generate). The generated `plugin.pb.go` diff
will be large but mechanical.
-> Mitigation: Standard `make proto` workflow; CI validates via
`buf lint`.

**[Provider adoption lag]** Providers won't populate `source`
until they update to the new proto. Evidence entries will have
nil `source` during the transition period.
-> Mitigation: All code paths handle nil `source` gracefully.
Tracking issue filed on complytime-providers.

**[Markdown rendering density]** Adding source metadata to the
evidence line may make it long.
-> Mitigation: Only show `reference-id` and `coordinate` (the
two most useful fields). Full source details remain in the
YAML/JSON evaluation log.
