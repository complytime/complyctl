## Context

`complyctl scan` currently filters the terminal summary table to show only
non-passing controls (failed, skipped, errors). Passing controls are counted
but omitted from the table. This was a deliberate design decision documented
in FR-037 (spec 001, Session 2026-02-26d): "Passed results are NOT listed
individually -- they appear only in the totals line."

User feedback via [#438](https://github.com/complytime/complyctl/issues/438) demonstrated that this design makes it difficult to confirm remediation after re-scanning. This change supersedes that specific aspect of FR-037.

The relevant code path is:

1. `FormatScanSummary()` in `internal/output/scan_summary.go` iterates
   assessments, categorizes each by result, and only appends non-passing
   entries to the table rows.
2. The `nonPassingEntry` struct and `nonPassingSortPriority()` function
   encode this filter-and-sort behavior.
3. `ShowPlainTable()` in `internal/terminal/table.go` renders the final
   table -- it is generic and does not filter.

The `StatusPassed` emoji constant already exists in
`internal/complytime/consts.go` but is unused in the scan summary path.

The Markdown formatter (`internal/output/markdown.go`) already includes
ALL results (passing and non-passing) because it iterates
`m.evalLog.Evaluations` without result-based filtering. No changes to
the Markdown formatter are needed.

### Goals

- Show passing controls in the scan summary table so users can confirm
  remediation at a glance
- Provide a `--show-passing` flag (default: `true`) with a
  `COMPLYTIME_SHOW_PASSING` env var to let users suppress passing rows
- Maintain the existing sort order: failures first, then errors, then
  skipped, then passing (least urgent last)

### Non-Goals

- ANSI color output (no terminal color support today; out of scope)
- Changes to OSCAL, SARIF, or EvaluationLog output formats (these
  already include all results)
- Changes to the Markdown report format (already includes all results)
- Changes to the summary conclusion line format
- Interactive/filterable TUI for scan results

## Decisions

### D1: Rename `nonPassingEntry` to `summaryEntry`

**Decision**: Rename the `nonPassingEntry` struct to `summaryEntry` since
it will now hold both passing and non-passing entries.

**Rationale**: The struct fields are generic (targetID, requirementID,
controlID, result, emoji, message). Only the name implies non-passing
filtering. Renaming avoids confusion without changing any fields.

**Alternative considered**: Creating a separate `passingEntry` struct.
Rejected because the fields are identical; two structs would duplicate
code for no benefit.

### D2: Add `showPassing bool` parameter to `FormatScanSummary()`

**Decision**: Add a `showPassing bool` parameter to control whether
passing entries appear in the table. When `false`, behavior matches
today's output exactly (preserving FR-037's original behavior).

**Rationale**: Keeps the function self-contained and testable. The CLI
flag value flows directly to this parameter.

**Alternative considered**: Using a functional options pattern or a
config struct. Rejected as over-engineering for a single boolean.

### D3: Sort passing entries last

**Decision**: Extend `nonPassingSortPriority()` (renamed to
`sortPriority()`) to return priority 6 for `gemara.Passed`, placing
passing rows after all non-passing rows. The existing `default` case
remains at priority 5 for forward compatibility with unknown/future
result types.

**Rationale**: Users scanning for problems should see failures first.
Passing rows confirm remediation but should not push failure details
below the fold. Full sort order: Failed (1) > Unknown (2) > NeedsReview
(3) > NotApplicable/NotRun (4) > other (5) > Passed (6).

### D4: Default `--show-passing` to `true`

**Decision**: The `--show-passing` flag defaults to `true`, showing
passing controls by default.

**Rationale**: The issue (#438) requests showing passing controls to
make scan output more intuitive. Defaulting to `true` delivers the
requested behavior out of the box. Users with large policies (100+
controls) or CI pipelines can opt out with `--show-passing=false` or
`COMPLYTIME_SHOW_PASSING=false`.

### D5: Passing entry message

**Decision**: For passing assessments, display the matching step
message (same as non-passing entries). If no step message exists,
display an empty string.

**Rationale**: Consistency. Providers may include informative messages
on passing steps (e.g., "RBAC correctly configured"). Suppressing
them would discard useful context.

### D6: Environment variable support

**Decision**: Add `COMPLYTIME_SHOW_PASSING` environment variable
following the established precedence pattern: flag > env var > default.

**Rationale**: Consistent with existing patterns (`COMPLYTIME_WORKSPACE`,
`COMPLYTIME_EXPORT_ENABLED`). CI pipelines can set the env var once
rather than modifying every `complyctl scan` invocation.

### D7: No Markdown formatter changes

**Decision**: The Markdown report formatter does not need changes.

**Rationale**: The Markdown formatter (`internal/output/markdown.go`)
iterates `m.evalLog.Evaluations` without result-based filtering --
it already includes all results (passing and non-passing). The
`--show-passing` flag only affects the terminal summary table output,
not file-based report formats.

## Risks / Trade-offs

- **[Large table output]** Policies with many controls (e.g., 200+)
  will produce long tables when `--show-passing` is `true`.
  -> Mitigation: Users can use `--show-passing=false` or set
  `COMPLYTIME_SHOW_PASSING=false`. The conclusion line always
  shows counts regardless.

- **[Backward compatibility]** Existing scripts or CI pipelines that
  parse scan stdout may break if they assume only non-passing rows.
  -> Mitigation: `--show-passing=false` or `COMPLYTIME_SHOW_PASSING=false`
  restores the old behavior. The conclusion line format is unchanged.
  This change MUST be documented in `CHANGELOG.md` with migration
  guidance: "If your CI pipeline parses scan stdout, add
  `--show-passing=false` or set `COMPLYTIME_SHOW_PASSING=false` to
  preserve previous behavior."

- **[Function signature change]** Adding `showPassing bool` to
  `FormatScanSummary()` is a breaking change for any direct callers.
  -> Mitigation: The function is internal (not in `pkg/`), so only
  in-repo callers need updating. There is exactly one call site
  (`cmd/complyctl/cli/scan.go:415`). Existing tests
  (`TestFormatScanSummary_AllPassed`, `TestFormatScanSummary_SingleTarget`,
  etc.) must be updated to pass the new parameter.
