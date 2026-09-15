## Why

The Gemara `#Evidence` schema (defined in `auditlog.cue`) includes a
`source` field of type `#EvidenceMapping` that records **where**
evidence was collected from -- which artifact, which location within
it, and a content digest for integrity pinning. The go-gemara SDK
already exposes this as `Evidence.Source` (`EvidenceMapping` struct).

complyctl's gRPC provider API (`api/plugin/plugin.proto`) currently
carries only the basic evidence fields (`id`, `type`, `description`,
`payload`, `collected_at`) and explicitly defers the `source` field
with the comment: *"extensible for ADR 0023 EvidenceMapping origin
fields in a future revision."* ADR 0023 was never written.

Without `source`, providers cannot report evidence provenance --
auditors and downstream consumers cannot trace an evidence entry
back to the specific artifact, file path, API endpoint, or Gemara
entry that produced it. This gap was identified by the Gemara
project maintainers as a missing capability.

## What Changes

- Add `EvidenceMapping` message to the proto API with fields
  matching go-gemara's `EvidenceMapping` struct: `reference_id`,
  `coordinate`, `entry_id`, `digest`, `remarks`.
- Add `source` field (type `EvidenceMapping`) to the existing proto
  `Evidence` message (field number 6, backward-compatible).
- Add `EvidenceSource` struct to `pkg/provider/client.go` and a
  `Source *EvidenceSource` field on the existing `Evidence` type.
- Update proto-to-internal and internal-to-proto mapping functions
  to carry `source` through the gRPC boundary.
- Update the evaluator (`internal/output/evaluator.go`) to map
  `provider.EvidenceSource` to `gemara.EvidenceMapping` when
  constructing evaluation log entries.
- Update the Markdown formatter to render source provenance
  metadata when present.
- Update the test provider to emit evidence with `source` populated
  for E2E coverage.
- Remove the stale "ADR 0023" comment from `plugin.proto`.

## Capabilities

### New Capabilities

- `evidence-source`: Provider API and evaluator support for evidence
  source provenance via `EvidenceMapping`, enabling providers to
  report where evidence was collected from and consumers to trace
  evidence back to its origin.

### Modified Capabilities

(none -- no existing specs to modify)

## Impact

- **Proto API** (`api/plugin/plugin.proto`): Additive message and
  field. Fully backward-compatible -- providers compiled against the
  old proto will simply not send `source`, and it will be absent
  (nil) on the receiving side.
- **Provider SDK** (`pkg/provider/`): New exported types
  (`EvidenceSource` struct, `Source` field on `Evidence`). No
  breaking changes.
- **Evaluator** (`internal/output/evaluator.go`): Conditional
  mapping of `Source` when non-nil.
- **Markdown formatter** (`internal/output/markdown.go`): Enhanced
  evidence metadata rendering.
- **Test provider** (`cmd/test-provider/main.go`): Updated fixture
  data.
- **External**: Tracking issue on `complytime-providers` for
  provider-side adoption of the new `source` field.
