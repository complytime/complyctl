## Context

PR #595 rewrote `ParsePolicyRef` to delegate OCI reference parsing
to oras-go and the follow-up work on `opsx/fix-complypack-oci-ref`
split `PolicyRef.Version` into typed `Tag`/`Digest` fields. Two
gaps remain (issue #600):

1. `ParsePolicyRef` silently converts the legacy `@version`
   notation to `:tag` before delegating to oras-go. No deprecation
   signal reaches the user. In the OCI Distribution Spec, `@` is
   exclusively a digest separator; using it for tags is ambiguous.

2. Three `complyctl doctor` check functions (`CheckPolicyActivePeriod`,
   `CheckComplypacks`, `CheckVariables`) silently `continue` when
   `ParsePolicyRef` returns an error, producing no diagnostic output.
   `CheckPolicyVersions` already reports these as
   `StatusFail, Blocking: true` results -- the other functions
   should follow the same pattern.

### Current deprecation pattern

The codebase uses `fmt.Fprintf(os.Stderr, "WARNING: ...")` for
warnings (see `workspace.go:printDeprecationWarning`,
`get.go:235`, `scan.go:308`). A `DEPRECATED:` prefix variant
will distinguish deprecation from general warnings.

### Current error-reporting pattern in doctor.go

`CheckPolicyVersions` (lines 244-253) reports `ParsePolicyRef`
failures as:
```go
CheckResult{
    Name:     fmt.Sprintf("policy/%s", p.EffectiveID()),
    Status:   StatusFail,
    Message:  fmt.Sprintf("invalid policy reference: %v", err),
    Blocking: true,
}
```

## Goals / Non-Goals

**Goals:**

- Warn users that `@version` syntax is deprecated so they can
  migrate to `:tag` syntax before removal.
- Surface `ParsePolicyRef` errors in all `complyctl doctor` checks
  so configuration problems are visible to the user.
- Surface resolution errors in `CheckVariables` with specific
  error messages instead of a silent counter.

**Non-Goals:**

- Removing `@version` support -- this change adds the deprecation
  warning only. Removal is deferred to a future release.
- Changing `ParsePolicyRef` return values or signature -- the
  function continues to accept `@version` and produce correct
  output; only a stderr warning is added.
- Adding deprecation warnings for bare policy IDs with `@version`
  -- bare IDs (no slash) are a complytime convention, not OCI
  references, so the `@` separator is unambiguous there.

## Decisions

### D1: Deprecation warning location

**Decision**: Emit the warning inside `ParsePolicyRef` itself,
at the point where `@version` is converted to `:tag`.

**Rationale**: This is the single code path that handles the
conversion. Placing the warning here ensures every caller
benefits without duplicating logic. The warning writes to
`os.Stderr` matching existing patterns.

**Alternative considered**: Return a structured warning alongside
the result. Rejected because it would change the function
signature (`(PolicyRef, []Warning, error)`) and require all 10+
callers to handle a new return value for a temporary deprecation.

### D2: Warning deduplication

**Decision**: Deduplicate warnings by raw URL using a
package-level `map[string]bool` protected by `sync.Mutex`.
Each unique policy URL with `@version` emits one warning per
process invocation regardless of how many callers invoke
`ParsePolicyRef`.

**Rationale**: `ParsePolicyRef` is called by many code paths
(`EffectiveID`, `validatePolicyRefs`, `CheckPolicyVersions`,
`CheckPolicyActivePeriod`, `CheckComplypacks`, `CheckVariables`,
and CLI commands). Without deduplication, a single `@version`
policy could emit 10+ identical warnings per CLI invocation,
creating noise that obscures real diagnostic output.

### D3: Doctor error reporting -- StatusFail with Blocking: true

**Decision**: `CheckPolicyActivePeriod` and `CheckComplypacks`
report `ParsePolicyRef` failures as
`StatusFail, Blocking: true`, matching `CheckPolicyVersions`.

**Rationale**: A malformed policy URL is a configuration error
that prevents the check from running. `StatusFail` with
`Blocking: true` is the established pattern for this case.

**Alternative considered**: `StatusWarn, Blocking: false`.
Rejected because a parse failure is not a soft concern -- the
check cannot proceed, and the user must fix their config.

### D4: CheckVariables resolution error messages

**Decision**: Replace the silent `resolveFailures++` counter with
per-failure `CheckResult{Status: StatusWarn, Blocking: false}`
results that include the policy ID and error description.

**Rationale**: The user currently sees no indication that a
policy failed to resolve. Per-failure results surface the
specific broken policy and the reason (parse error, version not
found, graph resolution failure, or policy not found in config).

**Status**: `StatusWarn` rather than `StatusFail` because
variable checking can still proceed for other policies. The
`resolveFailures` counter is retained for the downstream
unmapped-target-vars control flow (`unmappedReason`), while
per-failure results are added for user visibility.

### D5: Deprecation warning message format

**Decision**: Use the format:
```text
DEPRECATED: @version notation in policy URL "<raw>".
Use ":tag" syntax instead (e.g., "registry.com/repo:v1.0").
@version support will be removed in a future release.
```

**Rationale**: Three-line message clearly identifies the problem,
the fix, and the timeline. The `DEPRECATED:` prefix distinguishes
it from operational `WARNING:` messages.

## Risks / Trade-offs

- **[Risk]** Deprecation warning on stderr may be noisy in
  automated pipelines.
  **Mitigation**: Warning is emitted per-URL, not per-call.
  Pipelines can redirect stderr or migrate URLs.

- **[Risk]** `CheckVariables` producing per-failure results
  changes doctor output for users with broken configs.
  **Mitigation**: Previously these failures were invisible.
  Surfacing them is strictly an improvement -- users with
  valid configs see no change.

- **[Risk]** `Blocking: true` on parse failures in
  `CheckPolicyActivePeriod`/`CheckComplypacks` may cause
  `complyctl doctor` to report blocking issues where it
  previously reported none.
  **Mitigation**: The parse failure would also be reported
  by `CheckPolicyVersions` (which already has this behavior),
  so the user would already see a blocking error. The
  additional results provide context about which checks were
  affected.
