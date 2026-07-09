## Context

The `complyctl scan` command currently uses `cobra.NoArgs` and requires `--policy-id`. Target filtering is done exclusively by `filterTargetsForPolicy()` which returns all targets referencing the given policy. There is no mechanism to select a specific target.

The scan flow is: `run()` → `scanPolicy()` → `filterTargetsForPolicy()` → `executeScanPhase()` → `scanAllTargets()`. The target filtering happens at one place (`scanPolicy()` line 157), making it straightforward to add an additional filter.

The `generate` command has the same pattern and could benefit from the same target scoping, but that is out of scope for this change.

## Goals / Non-Goals

**Goals:**
- Add optional positional `<target>` argument to `complyctl scan`
- Make `--policy-id` inferrable when a target references exactly one policy
- Maintain full backward compatibility — `complyctl scan --policy-id X` works identically
- Provide shell completion for target IDs

**Non-Goals:**
- Supporting multiple positional target arguments (single target only)
- Adding target scoping to the `generate` command (future work)
- Changing how targets are defined in `complytime.yaml`

## Decisions

### D1: Positional argument, not a flag

Use a positional argument (`complyctl scan <target>`) rather than a flag (`--target`).

Rationale: The target is the primary noun of the scan operation — "scan this target." Positional arguments are the natural CLI idiom for the primary subject of a command (like `git checkout <branch>`, `docker run <image>`). A flag would work but feels heavier for something used frequently.

### D2: Policy inference from single-policy targets

When a target references exactly one policy, `--policy-id` can be omitted — the policy is inferred. When a target references multiple policies, `--policy-id` is required to disambiguate. The error message in the multi-policy case lists the available policies for that target.

Rationale: Most targets reference a single policy. Requiring `--policy-id` in that case is redundant typing. The inference rule is simple and deterministic — no ambiguity.

### D3: Require at least one of target or --policy-id

`complyctl scan` with neither a target nor `--policy-id` is an error. The error message explains the two options.

Rationale: Scanning "everything" (all targets × all policies) is a complex operation that could be slow and produce confusing output. It's better to require explicit scoping. This also avoids a breaking change — `--policy-id` was previously required, so existing scripts that omit it already get an error.

### D4: Hard error on target/policy mismatch

When both `<target>` and `--policy-id` are specified but the target does not reference that policy, the command exits with an error: `target "X" does not reference policy "Y" (available policies: a, b)`.

Rationale: Silent skip or empty results would be confusing. A clear error with the target's available policies helps the user correct the invocation immediately.

### D5: cobra.MaximumNArgs(1) for the positional argument

Change from `cobra.NoArgs` to `cobra.MaximumNArgs(1)`. The argument is optional — when omitted, the command falls back to `--policy-id`-only behavior (scan all targets for that policy).

Rationale: `MaximumNArgs(1)` allows zero or one positional arg, preserving backward compatibility while enabling the new target-scoped flow.

### D6: Resolution in run(), filtering in scanPolicy()

Target resolution (lookup by ID, existence validation, policy inference) happens in `run()` alongside the existing config loading and policy lookup. The resolved target ID is passed through to `scanPolicy()`, which performs the actual filtering after `filterTargetsForPolicy()` — narrowing the `policyTargets` slice to just the specified target. This keeps `run()` as the validation layer and `scanPolicy()` as the execution layer, matching the existing pattern.

Rationale: Clean separation of concerns. `run()` validates inputs and resolves references. `scanPolicy()` already does target filtering via `filterTargetsForPolicy()` — adding a second filter step there is natural and minimal diff. Avoids mutating `cfg.Targets` before passing it through.

### D7: Target narrowing happens AFTER generation, BEFORE scan

In `scanPolicy()`, the target filter must be applied after `ensureGenerated()` but before `executeScanPhase()`. Generation freshness is tracked per-policy (via `GenerationState.PolicyDigest`), not per-target. If we narrow targets before generation, a sequence like:

```bash
complyctl scan prod --policy-id nist    → generates for prod only
complyctl scan staging --policy-id nist → skips generation (digest fresh) — staging never generated
```

would silently skip generation for `staging` because the policy digest hasn't changed. By keeping generation on the full `policyTargets` set and narrowing only for the scan execution, generation always covers all targets that reference the policy (matching current behavior), while the scan itself is scoped to the requested target.

Rationale: Correctness with minimal change. Fixing generation state to be target-aware would work but is scope creep. The ordering approach is a one-line difference — filter after `ensureGenerated()`, not before.

## Risks / Trade-offs

- **[UX] Positional + flag interaction**: Users must understand that `<target>` is the first positional arg, not a subcommand. Mitigated by clear help text and examples.
- **[Backward compat] --policy-id no longer marked required**: Cobra's `MarkFlagRequired` is removed, replaced by custom validation. Existing `--policy-id`-only invocations still work, but the cobra-generated "required flag" error message changes to our custom one.
- **[Performance] Generation runs for all targets even when scanning one**: When a target is specified, generation still runs for all targets referencing the policy. This matches current behavior and ensures correctness. Optimizing generation to be target-aware is future work.
