## Why

When users run `complyctl scan`, the terminal output only shows non-passing controls (failed, skipped, errors). Passing controls are excluded from the results table entirely. This makes it difficult to confirm that a previously-failing control has been fixed -- users must infer success from the absence of a requirement ID in the output. As reported in [#438](https://github.com/complytime/complyctl/issues/438), this is confusing when re-scanning after remediation, especially for policies with many similarly-named controls.

## Supersedes

This change partially supersedes the non-passing-only table principle established in FR-037 (spec 001, Session 2026-02-26d): "Passed results are NOT listed individually -- they appear only in the totals line." User feedback via [#438](https://github.com/complytime/complyctl/issues/438) demonstrated that the original rationale ("an admin scanning 50 controls with 47 passing doesn't need 47 green rows") does not hold when users are actively remediating and need to confirm fixes. The `--show-passing` flag (default: `true`) provides the new behavior while `--show-passing=false` preserves FR-037's original non-passing-only behavior.

## What Changes

- Show passing controls in the scan summary table with a `StatusPassed` emoji, sorted after non-passing entries
- Add a `--show-passing` flag (default: `true`) to `complyctl scan` so users can opt out with `--show-passing=false` for large policies where the table would be unwieldy
- Add `COMPLYTIME_SHOW_PASSING` environment variable as a CI-friendly override (following established `COMPLYTIME_*` pattern)
- Update the summary conclusion line to remain unchanged (already shows pass count)
- **Note**: The Markdown report (`--format pretty`) already includes all controls (passing and non-passing) via the `EvaluationLog`. No Markdown changes are needed.

## Capabilities

### New Capabilities
- `scan-passing-display`: Display passing controls alongside non-passing controls in scan terminal summary output

### Modified Capabilities
- `scan-summary`: `FormatScanSummary()` gains `showPassing bool` parameter; table now includes passing rows when enabled

### Removed Capabilities
None.

## Impact

- `internal/output/scan_summary.go`: `FormatScanSummary()` must include passing entries in the table output, sorted by severity (failures first, passes last)
- `cmd/complyctl/cli/scan.go`: Add `--show-passing` flag and `COMPLYTIME_SHOW_PASSING` env var, thread through to `FormatScanSummary()`
- `internal/complytime/consts.go`: `StatusPassed` constant already exists
- `internal/output/markdown.go`: No changes needed (already includes all results)
- `internal/terminal/table.go`: No changes expected (plain table renderer is generic)
- `CHANGELOG.md`: Entry under `### Added` for `--show-passing` flag
- `AGENTS.md`: Update Recent Changes section

## Documentation Impact

- `CHANGELOG.md`: Add entry for `--show-passing` flag and default behavior change
- `AGENTS.md`: Add entry to Recent Changes section
- Website issue: [unbound-force/website#171](https://github.com/unbound-force/website/issues/171) filed for CLI documentation update

## Constitution Alignment

**I. Autonomous Collaboration**: The `--show-passing` flag and `COMPLYTIME_SHOW_PASSING` env var follow established CLI patterns (`--workspace`/`COMPLYTIME_WORKSPACE`), requiring no special documentation for collaborating agents.

**II. Composability First**: The `showPassing bool` parameter to `FormatScanSummary()` is a minimal, composable addition. The function remains self-contained and testable in isolation.

**III. Observable Quality**: Passing controls become visible in the terminal, improving observability of compliance posture. CRAP score impact is minimal (one additional case in a switch statement).

**IV. Testability**: Each scenario (mixed results, all-passing, flag true/false) maps directly to a unit test case with deterministic fixtures.
