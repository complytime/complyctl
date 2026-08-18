## Context

`complyctl scan` generates an EvaluationLog — the primary compliance assessment artifact — serialized exclusively as YAML via `internal/output.EvaluationLog.Write()`. The upstream `go-gemara` types already carry both `json:` and `yaml:` struct tags. However, the shadow structs in `internal/output/evaluator.go` (used to control serialization shape) only have `yaml:` tags.

Automation consumers (CI pipelines, policy-as-code tooling, jq-based scripts) prefer JSON. The `complyctl doctor --format json` precedent demonstrates the pattern: flag + env var, stdlib `encoding/json`, constant in `internal/complytime/consts.go`.

The `BuildReportFilename()` helper already accepts an `ext` parameter, so filename generation requires no changes beyond passing `"json"` vs `"yaml"`.

## Goals / Non-Goals

**Goals:**
- Add `--log-format yaml|json` flag to `complyctl scan` (default: `yaml`).
- Add `COMPLYTIME_LOG_FORMAT` env var override (flag takes precedence).
- Produce `evaluation-log-<policy>-<target>-<timestamp>.json` when `--log-format json`.
- Maintain full structural fidelity: JSON output uses the same kebab-case field names as the upstream go-gemara JSON tags.
- Add tests for the JSON output path.

**Non-Goals:**
- JSON support for OSCAL, SARIF, or Markdown reports (those have their own format flags).
- Streaming or NDJSON output.
- Changing YAML output structure or field naming.
- Modifying upstream `go-gemara` types.

## Decisions

### D1: Flag name `--log-format` (not `--eval-format` or extending `--format`)

`--format` controls secondary reports (oscal, pretty, sarif). Reusing it for EvaluationLog format would require a combined value scheme (e.g., `--format json,oscal`) and would conflate two independent decisions. `--log-format` is scoped and unambiguous.

*Alternative considered*: Separate `--log-json` boolean flag. Rejected — less extensible; future formats (e.g., protobuf binary) would require another flag.

### D2: Add `logFormat string` parameter to `Write(outDir, logFormat string)`

The `Write` method is the single serialization site. Adding the parameter there keeps format selection local to the output package and avoids threading a format constant through multiple intermediate call sites.

*Alternative considered*: Functional option `WithLogFormat(fmt string)` on the `EvaluationLog` struct. Rejected — adds indirection without benefit given a single call site.

### D3: JSON tags on shadow structs use kebab-case matching go-gemara

The shadow structs (`serializableEvaluationLog`, `serializableControlEvaluation`, `serializableAssessmentLog`) are the serialization boundary. JSON tags are added *alongside* existing YAML tags (not replacing them), e.g., `` `json:"metadata" yaml:"metadata"` ``. This preserves dual-format capability. Embedded go-gemara types (`Metadata`, `Resource`, `EntryMapping`, `Evidence`) already carry correct `json:` tags from upstream and require no modification. The shadow struct JSON tags must match the upstream `json:` keys in `vendor/github.com/gemaraproj/go-gemara/generated_types.go` (e.g., `gemara-version`, `assessment-logs`, `steps-executed`, `confidence-level`) to produce an interoperable output format.

*Alternative considered*: camelCase JSON tags. Rejected — would diverge from the upstream schema and break consumers expecting the Gemara JSON shape.

### D4: `encoding/json` with `MarshalIndent` (stdlib, no new dependency)

Pretty-printing with 2-space indent improves human readability while keeping machine parsability. No new dependency introduced.

*Alternative considered*: `encoding/json` compact (`Marshal`). Rejected — minimal size saving for compliance reports; readability matters for debugging.

### D5: Validation rejects values other than `yaml` and `json`

Unknown format values return an error at startup, not silently fall back to YAML. Fail-fast prevents silent data loss. Validation is performed at the CLI layer (in `scan.go`) before calling `Write()`. The `Write()` method itself does not validate the format value — it trusts its caller. This keeps `Write()` simple and avoids duplicated validation.

## Risks / Trade-offs

- **Shadow struct drift**: If new fields are added to go-gemara types and shadow structs are updated, JSON tags must be added alongside YAML tags. → Mitigation: code review checklist item; struct tags are co-located so the pattern is visible.
- **`Write()` signature change**: All callers must pass `logFormat`. Currently only one call site (`writeScanReports`). → Mitigation: compiler enforces; no silent breakage.
- **Env var collision**: `COMPLYTIME_LOG_FORMAT` is a new env var in the `COMPLYTIME_` namespace. → Mitigation: name is unambiguous; documented in flag help and `--help` output.

## Migration Plan

No data migration required. The change is additive:
- Default remains `yaml`; existing scripts are unaffected.
- `Write()` callers (currently one) are updated in the same PR.
- No config file schema change.

Rollback: revert the PR; no persistent state is affected.

## Open Questions

None — design is fully specified.
