## Why

`complyctl scan` writes the EvaluationLog exclusively as YAML, but automation pipelines — CI step summaries, policy-as-code tooling, and data processing scripts — consume JSON more naturally. Providing JSON as an opt-in output format eliminates the need for downstream YAML-to-JSON conversion and aligns with the machine-readable output precedent already set by `complyctl doctor --format json`.

## What Changes

- New `--log-format yaml|json` flag on `complyctl scan` (default: `yaml`) controls EvaluationLog serialization format.
- New `COMPLYTIME_LOG_FORMAT` environment variable mirrors the flag (flag takes precedence).
- `internal/output.EvaluationLog.Write()` gains a `logFormat string` parameter; serializes to JSON (`evaluation-log-*.json`) or YAML (`evaluation-log-*.yaml`) based on the value.
- Shadow structs in `internal/output/evaluator.go` gain `json:` struct tags using the same kebab-case keys as the upstream `go-gemara` types.
- Shell completion for `--log-format` enumerates `yaml` and `json`.

## Capabilities

### New Capabilities

- `evallog-json-format`: Ability to serialize the EvaluationLog in JSON format via `--log-format json` / `COMPLYTIME_LOG_FORMAT=json`, producing `evaluation-log-*.json` output files.

### Modified Capabilities

<!-- No existing spec-level requirements are changing; this is a new additive capability. -->

### Removed Capabilities

None.

## Constitution Alignment

- **I. Centralized Constants**: `EvalLogFormatEnvVar` constant
  centralizes the env var name; no magic strings.
- **V. Do Not Reinvent the Wheel**: Uses stdlib
  `encoding/json` — no custom JSON serializer.
- **VI. Composability**: JSON output enables standard
  `jq`-based pipelines and CI integrations.
- **VII. Convention Over Configuration**: YAML default
  preserved; users only configure when deviating.

## Impact

- **`cmd/complyctl/cli/scan.go`**: `scanOptions` gains `logFormat string`; `--log-format` flag registered; env var override; validation; `writeScanReports()` passes `logFormat` to `eval.Write()`.
- **`internal/output/evaluator.go`**: `Write(outDir, logFormat string)` signature change; shadow structs gain `json:` tags; JSON marshal path added.
- **`internal/complytime/consts.go`**: `EvalLogFormatEnvVar = "COMPLYTIME_LOG_FORMAT"` constant added.
- **`internal/output/evaluator_test.go`**: New test cases cover JSON output path and format validation.
- **No breaking changes**: YAML remains the default; existing callers of `Write()` require a one-argument update to pass `"yaml"`.
- **No new dependencies**: Uses stdlib `encoding/json`.
