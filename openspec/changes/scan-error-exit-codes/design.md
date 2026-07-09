## Context

The `bug/scan-proto` commit added `repeated string errors` to `ScanResponse` and `RouteScanResult()` / `ScanResult` to the provider manager. The CLI does not use them yet. `scanSingleTarget()` in `cmd/complyctl/cli/scan.go` calls `mgr.RouteScan()` (the backward-compatible wrapper) and receives only `[]AssessmentLog`. Operational errors are either lost (absorbed into synthetic assessment entries by `errorAssessments()`) or invisible to the exit code.

The scan execution chain is:

```text
scanCmd.RunE → run() → scanPolicy() → executeScanPhase()
  → runScanAndReport() → executeScan() → scanAllTargets()
    → scanSingleTarget() → mgr.RouteScan()
```

Errors must propagate from `scanSingleTarget` up to `runScanAndReport` where they can influence both the output and the return value that ultimately sets the exit code.

## Goals / Non-Goals

### Goals
- Wire `RouteScanResult()` into the CLI scan chain
- Exit non-zero when providers report operational errors
- Display operational errors distinctly from evaluation results
- Maintain backward compatibility for providers that don't populate `errors`

### Non-Goals
- Changing exit codes for policy violations (ResultFailed exits 0 — findings are data)
- Adding a `--fail-on-violation` flag (future work, separate concern)
- Provider-side changes (handled in complytime-providers)
- Modifying the proto (already committed)

## Decisions

### D1: Exit 0 for policy violations, non-zero for operational errors

`complyctl scan` is an evaluation tool, not a gate. Policy violations (ResultFailed) are expected output — they represent known compliance posture. Operational errors (clone failures, missing tools, config issues) mean the tool couldn't complete its job — compliance posture is unknown for affected targets.

Exit code semantics:

| Condition | Exit code | Rationale |
| :-- | :-- | :-- |
| All targets scanned, all passed | 0 | Clean |
| All targets scanned, some failed | 0 | Findings are data |
| Some targets had operational errors | non-zero | Coverage gap |
| Fatal error (no providers, bad config) | non-zero | Command failed |

A `--fail-on-violation` flag could be added later for CI gate use cases. Out of scope here.

### D2: Propagate ScanResult through the chain, return error at the end

`scanSingleTarget()` switches to `RouteScanResult()`. It returns both `[]AssessmentLog` and `[]string` (errors). `scanAllTargets()` collects errors across targets. `runScanAndReport()` writes reports from assessments (unchanged), then checks collected errors. If errors exist, it prints them as warnings and returns an error to trigger non-zero exit.

Reports are written BEFORE the error return so the operator gets partial results even when some targets failed. The error message is a summary, not a dump of all errors (those are in the warnings output).

### D3: Errors displayed as warnings before the scan summary

Operational errors are printed to stderr as `WARNING:` lines before `FormatScanSummary` renders the assessment table. They are not mixed into the assessment table rows. This keeps the assessment table clean — it contains only evaluation results where compliance posture is known.

Format:

```text
WARNING: 2 operational error(s) during scan:
  - target 'staging': clone failed: auth denied
  - target 'dev': missing required tool: conftest

Scan: nist-800-53-r5 | Target: prod, staging, dev | 42 requirements
...
42 requirements: 38 passed, 4 failed, 0 skipped, 0 error
```

### D4: Backward compatible — empty errors means no behavior change

Providers that don't populate `ScanResponse.Errors` produce `ScanResult{Errors: nil}`. `HasErrors()` returns false. Exit code and output are identical to current behavior. No existing provider breaks.

### D5: scanAllTargets returns a result struct

Rather than threading multiple return values through the chain, introduce a local `scanOutput` struct in `scan.go`:

```go
type scanOutput struct {
    assessments      []provider.AssessmentLog
    assessmentTargets []string
    errors           []string
}
```

This keeps the propagation clean without changing public API signatures.

## Risks / Trade-offs

- **[UX] Non-zero exit for partial success**: A scan that evaluates 4 of 5 targets successfully now exits non-zero. CI pipelines that only check exit code will see "failure" even though 4 targets have valid results. Mitigated by: writing reports before returning the error, printing clear warnings explaining which targets failed and why.
- **[Backward compat] Exit code change**: Scripts that depend on exit 0 meaning "scan RPC succeeded" will break if providers start populating `errors`. Mitigated by: this only triggers when providers explicitly populate the field — existing providers return empty errors.
