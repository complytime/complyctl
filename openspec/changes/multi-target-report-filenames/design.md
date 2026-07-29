## Context

`complyctl scan` evaluates multiple targets per policy. Each target produces
reports in four formats: EvaluationLog, OSCAL, SARIF, and Markdown. The
EvaluationLog formatter includes `targetID` in filenames and works correctly.
The other three formatters construct filenames with only `policyID` + timestamp,
causing collisions when targets process within the same second.

Target IDs are already available inside each formatter via `log.Target.Id`
(OSCAL, SARIF) and `m.evalLog.Target.Id` (Markdown). No signature changes
are needed — the fix is purely in filename construction.

`FilenameSafe` in `internal/complytime/consts.go` already sanitizes path
separators (`/` → `-`) and is sufficient for target IDs.

## Goals / Non-Goals

**Goals:**

- Eliminate filename collisions in multi-target scans for OSCAL, SARIF,
  and Markdown reports.
- Centralize filename construction so all four formatters use a single helper,
  preventing drift.
- Handle empty `targetID` gracefully (omit the segment, no double dashes).

**Non-Goals:**

- Changing the `behavioral-report` tool — it is single-target with hardcoded
  filenames and not affected.
- Changing function signatures of `ToOSCAL`, `ToSARIF`, or `Markdown.Write`.
- Modifying `FilenameSafe` — it already handles the sanitization needed.

## Decisions

### D1: Centralized helper over inline fix

**Decision:** Extract `BuildReportFilename` in `internal/output/filename.go`
rather than adding `targetID` inline in each formatter's `fmt.Sprintf`.

**Alternatives considered:**
- *Inline fix*: Smallest diff, but leaves four copies of the pattern to
  maintain. A fifth formatter would need to know the convention.
- *Centralized helper*: Single source of truth. Four callers, one pattern.
  The evaluator formatter already has the correct pattern — the helper
  codifies it.

**Rationale:** The bug exists because three formatters independently
constructed filenames and missed a field. A helper makes this class of
bug impossible for future formatters.

### D2: Empty targetID produces the old filename pattern

**Decision:** When `targetID` is empty, `BuildReportFilename` omits the
segment entirely, producing `{prefix}-{policyID}-{timestamp}.{ext}`.

**Rationale:** In production, `buildEvaluators()` always populates
`target.ID`. But test helpers (e.g., `mockGemaraEvalLog()` in OSCAL and
SARIF tests) do not set `Target.Id`. Omitting the segment avoids breaking
test expectations and prevents filenames with double dashes.

### D3: Refactor evaluator to use the same helper

**Decision:** Refactor `evaluator.go` to call `BuildReportFilename` even
though its current filename is already correct.

**Rationale:** Consistency. If the helper is the single source of truth,
all formatters should use it.

## Risks / Trade-offs

- **[Filename change breaks downstream consumers]** → The old filenames
  were already broken (overwritten). Any consumer relying on the old
  pattern was already receiving incomplete data. The EvaluationLog
  pattern (with targetID) has been stable, so consumers already handle
  target-qualified filenames.

- **[Timestamp resolution still 1-second]** → Two different policies
  scanning the same target within one second could still collide. This
  is not a regression — policyID already differentiates those filenames.
  Sub-second resolution would be a separate enhancement.

- **[Empty targetID fallback masks bugs]** → A missing target ID in
  production would silently produce the old filename pattern instead of
  failing. Acceptable: `buildEvaluators()` guarantees population, and
  the fallback matches existing behavior.
