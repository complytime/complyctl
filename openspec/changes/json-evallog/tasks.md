## 1. Constants

- [x] 1.1 Add `EvalLogFormatEnvVar = "COMPLYTIME_LOG_FORMAT"` constant to `internal/complytime/consts.go`

## 2. Output Package

- [x] 2.1 Add `json:` struct tags (kebab-case, alongside existing `yaml:` tags) to `serializableEvaluationLog`, `serializableControlEvaluation`, and `serializableAssessmentLog` in `internal/output/evaluator.go`
- [x] 2.2 Update `Write(outDir string)` signature to `Write(outDir, logFormat string)` in `internal/output/evaluator.go`
- [x] 2.3 Add JSON serialization branch in `Write()`: when `logFormat == "json"`, use `json.MarshalIndent` with 2-space indent and pass `ext = "json"` to `BuildReportFilename()`; add `encoding/json` import

## 3. CLI Integration

- [x] 3.1 Add `logFormat string` field to `scanOptions` struct in `cmd/complyctl/cli/scan.go`
- [x] 3.2 Register `--log-format` flag (default `"yaml"`, valid values `yaml|json`) in the scan command setup, with help text documenting the `COMPLYTIME_LOG_FORMAT` env var override
- [x] 3.3 Add shell completion for `--log-format` enumerating `"yaml"` and `"json"`
- [x] 3.4 Add env var override block: read `COMPLYTIME_LOG_FORMAT` when `--log-format` is not explicitly set (flag takes precedence)
- [x] 3.5 Add validation: return error for any value other than `"yaml"` or `"json"` (case-sensitive)
- [x] 3.6 Update `writeScanReports()` to pass `opts.logFormat` to `eval.Write()`

## 4. Tests

- [x] 4.1 Add test `TestEvaluationLog_Write_JSON` in `internal/output/evaluator_test.go` verifying JSON file is created with `.json` extension and valid JSON content
- [x] 4.2 Add test `TestEvaluationLog_Write_JSON_FieldNames` verifying kebab-case field names (e.g., `gemara-version` inside `metadata`) and top-level fields (`metadata`, `result`, `evaluations`, `target`) appear in JSON output
- [x] [P] 4.3 Add test `TestEvaluationLog_Write_YAML_Default` verifying YAML path is unchanged when `logFormat == "yaml"`
- [x] [P] 4.4 Add test for invalid `logFormat` value — verifying the CLI validation function returns an error
- [x] 4.5 Add test verifying `COMPLYTIME_LOG_FORMAT=json` produces JSON output when `--log-format` is not set
- [x] 4.6 Add test verifying `--log-format yaml` takes precedence over `COMPLYTIME_LOG_FORMAT=json`
- [x] 4.7 Add test verifying invalid env var value (e.g., `COMPLYTIME_LOG_FORMAT=xml`) returns an error

## 5. Verification

- [x] 5.1 Run `make test-unit` and confirm all tests pass
- [x] 5.2 Run `make lint` and confirm zero issues
- [x] 5.3 Run `make build` and do a manual smoke test: `complyctl scan --log-format json` produces a valid JSON EvaluationLog
- [x] 5.4 Verify `COMPLYTIME_LOG_FORMAT=json complyctl scan` also produces JSON
- [x] 5.5 Verify `complyctl scan --log-format xml` exits non-zero with a clear error message

## 6. Documentation

- [x] 6.1 Add CHANGELOG.md entry for `--log-format` flag and `COMPLYTIME_LOG_FORMAT` env var
- [x] 6.2 Update AGENTS.md Recent Changes section with json-evallog summary
- [x] 6.3 Assess website gate: `--log-format` is a CLI-only flag addition with no user-facing documentation on the website; exempt per internal-only scope (no new workflow, no breaking change)
<!-- spec-review: passed -->
