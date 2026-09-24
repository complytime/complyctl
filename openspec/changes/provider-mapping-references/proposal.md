# Proposal

## Why

PR #840 plumbs `MappingReferences` from **policy metadata** into the
EvaluationLog (Direction A), making `control.reference-id` and
`requirement.reference-id` resolvable. However, providers have no
mechanism to declare **runtime data sources** they discover during a
scan -- the config files they read, API endpoints they queried, or OCI
artifacts they inspected. An evidence entry with
`source.reference-id: sshd-config` dangles because no
`MappingReference` in the EvaluationLog declares what `sshd-config` is.
The Gemara canonical examples show both policy-level and runtime-level
references coexisting in a single EvaluationLog. Direction B completes
the evidence traceability chain, making evidence traceable to its
actual origin for auditors.

## What Changes

- Add `MappingReference` proto message and `mapping_references` repeated
  field (field 3) to `ScanResponse` in `api/plugin/plugin.proto`.
  Additive change -- providers that do not send references are unaffected.
- Add `MappingReference` Go type and `MappingReferences` field to
  `provider.ScanResponse` in `pkg/provider/client.go`.
- Add proto-to-internal and internal-to-proto conversion functions for
  `MappingReference` in `pkg/provider/client.go` and
  `pkg/provider/server.go`, following the existing `EvidenceSource`
  mapping pattern.
- Thread provider-returned `MappingReferences` through the scan
  pipeline: `scanSingleTarget()` collects them from each
  `ScanResponse`, `scanAllTargets()` aggregates them into `scanOutput`,
  and `processScanOutput()` merges Direction A (policy graph) with
  Direction B (providers) before passing to `buildEvaluators()`.
- Implement `mergeMappingReferences()` with deduplication by `id`:
  - Policy wins on ID collision (policy is the authoritative
    declaration for catalog-type references).
  - Collisions produce a stderr WARNING (following the
    `FormatOperationalWarnings()` pattern), a structured `logger.Warn()`
    entry, and a `Description` annotation on the retained
    `MappingReference` in the EvaluationLog documenting what was
    discarded.
  - Provider-to-provider collisions (multi-evaluator, same ID from
    different providers) emit the same warning; first-wins is
    sufficient since this is an unintentional provider authoring
    mistake, not a production scenario.

## Capabilities

### New Capabilities

- `provider-mapping-references`: Provider-side declaration of runtime
  data sources as `MappingReferences` in `ScanResponse`, merged with
  policy-level references into the EvaluationLog metadata.

### Modified Capabilities

_(none -- no existing spec-level behavior changes)_

## Impact

- **Proto API** (`api/plugin/plugin.proto`): New `MappingReference`
  message and field 3 on `ScanResponse`. Backward-compatible (additive
  repeated field). Requires `make proto` (buf generate).
- **Provider SDK** (`pkg/provider/client.go`, `pkg/provider/server.go`):
  New type + conversion functions. Provider authors gain the ability to
  populate `MappingReferences` in their `ScanResponse`.
- **Scan pipeline** (`cmd/complyctl/cli/scan.go`): `scanSingleTarget()`
  gains a 4th return value; `scanOutput` gains a
  `mappingReferences` field; `processScanOutput()` gains merge logic
  before `buildEvaluators()`.
- **Output evaluator** (`internal/output/evaluator.go`): No changes --
  `NewEvaluator` already accepts `[]gemara.MappingReference` regardless
  of source.
- **Output scan summary** (`internal/output/scan_summary.go`): New
  `MergeMappingReferences()` and `FormatMappingCollisions()` functions
  for dedup and collision formatting (D7).
- **Provider manager** (`pkg/provider/manager.go`): `ScanResult` gains
  `MappingReferences` field; `RouteScanResult()` updated to populate it
  from `ScanResponse` in both targeted and broadcast code paths.
- **Cross-repo**: Provider-side adoption (populating the new field)
  tracked separately in complytime-providers.
- **Follow-up**: Rendering `MappingReferences` in report formats
  (Markdown, SARIF, OSCAL) is deferred to a separate follow-up issue
  (to be filed after implementation). Applies equally to Direction A
  and B references. Currently these references are only visible in
  the raw EvaluationLog YAML/JSON.
