# Design

## Context

See proposal.md for motivation. Direction A (PR #840) already
propagates `MappingReferences` from policy metadata through the
`DependencyGraph` → `Evaluator` → `EvaluationLog.Metadata` path.
The `NewEvaluator` function accepts `[]gemara.MappingReference`
and writes them to `GemaraLog().Metadata.MappingReferences`
(`internal/output/evaluator.go:135`). This path is source-agnostic
-- it does not care whether references originated from a policy or
a provider.

The provider gRPC API (`api/plugin/plugin.proto:101`) defines
`ScanResponse` with `assessments` (field 1) and `errors` (field 2).
There is no `MappingReference` message in the proto. The internal
`provider.ScanResponse` (`pkg/provider/client.go:71`) mirrors this
with `Assessments` and `Errors` fields.

The scan pipeline in `cmd/complyctl/cli/scan.go` collects
`ScanResponse` results through `scanSingleTarget()` →
`scanAllTargets()` → `scanOutput` → `processScanOutput()` →
`buildEvaluators()` → `NewEvaluator()`. Currently only
`Assessments` and `Errors` are threaded through; the scan pipeline
collects them via append loops in `scanSingleTarget()` (lines
697-714).

The Gemara CUE schema enforces `_uniqueRefIds` on
`mapping-references` -- no duplicate IDs are allowed in the
EvaluationLog output.

## Goals / Non-Goals

**Goals:**

- Providers can declare runtime data sources in `ScanResponse`
- Backward-compatible proto change (additive repeated field)
- Policy-level and provider-level references merged with dedup
- Collision warnings visible at runtime and in the EvaluationLog
- Minimal disruption to the existing scan pipeline

**Non-Goals:**

- Rendering MappingReferences in report formats (Markdown, SARIF,
  OSCAL) -- tracked as a separate follow-up issue. Applies equally
  to Direction A and B references. The EvaluationLog serialization
  already captures them.
- Provider-side implementation in complytime-providers -- tracked
  separately.
- Validating that evidence `source.reference-id` values resolve
  against declared MappingReferences -- complyctl is a transport
  layer (design decision D5 from evidence-source change).

## Decisions

### D1: New proto MappingReference message

**Decision**: Add a new `MappingReference` message to
`plugin.proto` with fields `id`, `title`, `version`,
`description`, `url` (all strings). Add
`repeated MappingReference mapping_references = 3` to
`ScanResponse`.

**Rationale**: Mirrors `gemara.MappingReference` field-for-field.
A dedicated message is cleaner than inlining five fields on
`ScanResponse`. Reusing `EvidenceMapping` is not appropriate -- it
has different fields (`reference_id`, `coordinate`, `entry_id`,
`digest`, `remarks`) that serve a different purpose (evidence
source provenance vs. reference declaration).

**Alternative considered**: Reuse `EvidenceMapping` message.
Rejected: different semantics and field set. `MappingReference`
declares metadata about a referenced artifact; `EvidenceMapping`
locates evidence within one.

### D2: Internal Go type named MappingReference

**Decision**: Name the internal type `provider.MappingReference`,
matching `gemara.MappingReference`.

**Rationale**: Unlike `EvidenceSource` (D2 from evidence-source
change), there is no import ambiguity risk here.
`provider.MappingReference` and `gemara.MappingReference` are in
different packages and are never used together in the same file --
the conversion from `provider.MappingReference` to
`gemara.MappingReference` happens in the scan pipeline merge
function, not in the evaluator. The evaluator already receives
`[]gemara.MappingReference` and does not import `pkg/provider`.

### D3: Policy wins on ID collision

**Decision**: When both the policy (Direction A) and a provider
(Direction B) declare a MappingReference with the same `id`, retain
the policy entry and discard the provider entry.

**Rationale**: Policy metadata is the authoritative declaration for
catalog-type references (e.g., `nist-800-53-r5`). It is authored
by policy writers and published in OCI artifacts. Provider-declared
references are runtime data sources (e.g., `sshd-config`) that
belong to a different semantic namespace. A collision indicates a
provider authoring mistake, not a legitimate override. Collision
risk is very low -- the two ID spaces are semantically orthogonal
(catalog framework names vs. runtime data source names), as
confirmed by analysis of all real-world IDs in the codebase.

**Alternative considered**: Provider wins (runtime context is
fresher). Rejected: it silently overrides the policy author's
intent, which is the authoritative source for catalog references.

**Alternative considered**: Error on collision. Rejected:
disproportionate -- a hard error for a low-risk provider authoring
mistake would block scan results unnecessarily.

### D4: Three-channel collision warning

**Decision**: Collisions produce:
1. A stderr WARNING block following the
   `FormatOperationalWarnings()` pattern
   (`internal/output/scan_summary.go:186`).
2. A structured `logger.Warn()` entry with key-value metadata.
3. A `Description` annotation on the retained MappingReference in
   the EvaluationLog, making the collision auditable.

**Rationale**: Channel 1 is immediate and always visible to
operators. Channel 2 provides structured data for debugging via
`--debug` or log file inspection. Channel 3 embeds the collision
record in the output artifact so downstream auditors can see it
without access to runtime logs.

The `MappingReference.Description` field
(`gemara.MappingReference.Description`, `omitempty`) is the only
free-form prose slot available per-reference. Using it for
collision documentation is appropriate because: the policy entry
likely has no description set (policy YAML test fixtures and
examples do not use it), and the annotation explains the
provenance of this specific reference.

### D5: Incremental pipeline threading (4th return value)

**Decision**: `scanSingleTarget()` gains a 4th return value
`[]provider.MappingReference`. `scanOutput` gains a
`mappingReferences []provider.MappingReference` field.

**Rationale**: Follows the established pattern -- `scanSingleTarget`
already returns `([]provider.AssessmentLog, []string, error)` where
assessments and errors are collected separately. Adding a 3rd
collected type (4th return value, counting the error) follows the
same structure. The alternative of
returning `*provider.ScanResponse` directly would change the
abstraction level and is better suited as a separate refactoring
if a 5th field is needed in the future (Principle III: incremental
improvement).

### D6: Provider-to-provider collision handling

**Decision**: When two providers declare MappingReferences with the
same `id`, first-wins with the same warning channels as D4.

**Rationale**: Multi-evaluator scans are the only scenario where
provider-to-provider collisions arise. Different providers scan
different controls for the same target -- they access different
data sources and should use different reference IDs. A collision
indicates a provider authoring mistake (e.g., both independently
chose `system-config` for unrelated sources). First-wins is
sufficient; deterministic ordering is achieved by sorting
evaluator IDs before iteration.

### D7: Merge function placement

**Decision**: `MergeMappingReferences()` and
`FormatMappingCollisions()` are placed in
`internal/output/scan_summary.go` alongside
`FormatOperationalWarnings()`.

**Rationale**: The merge is an output-pipeline concern (combining
references for the evaluator), not a provider SDK concern or a
policy resolution concern. Placing it alongside the existing
warning formatter keeps the warning-formatting pattern cohesive.
The function accepts `[]gemara.MappingReference` (Direction A) and
`[]provider.MappingReference` (Direction B), converts B to gemara
type during merge, and returns `[]gemara.MappingReference` plus a
collision report.

## Risks / Trade-offs

**[Proto regeneration]** Adding a new message and field requires
`make proto` (buf generate). The generated `plugin.pb.go` diff
will be large but mechanical.
-> Mitigation: Standard `make proto` workflow; CI validates via
`buf lint`.

**[Provider adoption lag]** Providers will not populate
`mapping_references` until they update to the new proto. Provider
responses will have an empty `mapping_references` during the
transition period.
-> Mitigation: All code paths handle empty/nil references
gracefully. Direction A references still flow through unchanged.
Tracking issue filed on complytime-providers.

**[Description field overwrite]** If a policy author sets
`Description` on a MappingReference and a collision occurs, the
collision annotation would overwrite it.
-> Mitigation: Prepend the collision note to the existing
description rather than replacing it. In practice, no policy
test fixtures or examples currently use the `Description` field.

**[Multi-evaluator ordering]** Go map iteration over evaluator
groups is non-deterministic. Provider-to-provider collision
first-wins depends on iteration order.
-> Mitigation: Sort evaluator IDs before iterating in
`scanSingleTarget()` to make collision resolution deterministic.
