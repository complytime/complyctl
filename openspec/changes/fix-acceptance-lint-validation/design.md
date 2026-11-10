# Design

## Context

See `proposal.md` for motivation. `make lint` currently runs the default
Go lint scope, while acceptance-tagged Go security analysis, repository-wide
YAML linting, and pre-commit checks use different scopes and are not a single
CI-equivalent contract. MegaLinter exclusions do not apply to standalone
tools.

## Goals / Non-Goals

**Goals:**

- Make each local validation profile explicit, reproducible, and documented.
- Keep CI-equivalent style scope separate from broader local checks.
- Keep acceptance security analysis enabled and separately actionable.
- Protect the intended scope with automated regression checks.

**Non-Goals:**

- Changing public CLI behavior or provider interfaces.
- Disabling gosec, MegaLinter security scanners, or pre-commit hooks.
- Treating every local validation command as an exact replacement for CI.

## Decisions

### Define named validation profiles

Add explicit Make targets for CI-parity style/lint validation and
acceptance-tagged security lint. Document their version, flags, build tags,
and input scope alongside the existing contributor guidance.

Rationale: named profiles make the intended comparison observable and avoid
claiming that broader local commands reproduce CI.

Alternative considered: expand `make lint` to include every local check.
Rejected because it would silently turn existing baseline findings into a
new gate and conflate unrelated validation purposes.

### Scope style exclusions at the invoking profile

Apply vendored and generated-file exclusions only to the standalone style
commands that need CI-equivalent file selection. Keep the exclusions out of
global security configuration.

Rationale: tool-specific scope is explicit and does not weaken security
coverage.

Alternative considered: rely on MegaLinter exclusions. Rejected because
standalone yamllint and pre-commit do not inherit them.

### Keep acceptance security findings actionable

Run acceptance-tagged security lint under a dedicated target. Resolve its
four baseline findings through code changes or narrow, reviewed
dispositions backed by the test infrastructure trust model.

Rationale: configured registry endpoints and test-file permissions need a
deliberate security decision, not a broad exclusion.

Alternative considered: add a global gosec exclusion. Rejected because it
would hide future findings of the same class.

## Risks / Trade-offs

- [CI tooling changes independently] -> Pin or document the intended version
  source and update the profile with CI changes.
- [Exclusions become too broad] -> Add regression tests that assert excluded
  paths and preserve security-lint execution.
- [Acceptance lint changes test behavior] -> Keep test execution and security
  lint as separate targets with independently documented prerequisites.

## Migration Plan

1. Add the validation profiles and their scope regression tests.
2. Resolve or narrowly document the acceptance security baseline findings.
3. Update contributor documentation and CI references.
4. Run the supported profiles against the baseline and verify that broad
   security scanning remains enabled.

Rollback consists of restoring the prior Make targets and configuration if a
new profile cannot reproduce its documented CI behavior. No persisted data or
user-facing API migration is required.
