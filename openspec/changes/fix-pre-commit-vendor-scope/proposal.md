# Proposal

## Why

Project formatting and SPDX hooks select vendored files, causing
unrelated changes and adding project license metadata to third-party
code. Issue #972 needs a scoped fix so contributors can run the hooks
while preserving upstream sources and first-party checks.

## What Changes

- Exclude the repository-root `vendor/` tree only from
  `trailing-whitespace`, `end-of-file-fixer`, and `check-spdx`.
- Preserve the existing rules, pinned hook revisions, file-type
  filters, and stage assignments for maintained first-party files.
- Add regression checks using actual hook execution in temporary
  normal Git repositories, covering staged-file and all-files runs.
- Inspect first-party baseline findings in a disposable checkout;
  fix ordinary formatting findings within scope or record exact
  paths and a disposition for findings needing separate approval.
- Document hook scope, stage-specific execution, prerequisites, and
  a Make target for the regression suite.
- Leave unrelated hooks and security scanning policy intact.

## Capabilities

### New Capabilities

- `project-hook-scope`: Ownership-aware selection for the project's
  whitespace, EOF, and SPDX hooks, with continued first-party checks
  across their configured Git stages.

### Modified Capabilities

None. The canonical capability inventory is empty. The in-flight
`yaml-lint-scope` capability under `fix-acceptance-lint-validation`
covers standalone yamllint, not these hooks.

## Impact

- Configuration: `.pre-commit-config.yaml`, limited to the three
  named hooks after the required policy authorization.
- Tests: a Python regression suite under `tests/`, invoking the
  real hooks and the repository SPDX script through pre-commit.
- Developer tooling: `Makefile`, `docs/STYLE_GUIDE.md`, and a
  contributor-facing changelog entry. Assess `AGENTS.md` for the
  new test command through the repository's approval process.
- Baseline cleanup: only individually verified first-party
  formatting changes; no vendored or protected template rewrites.
- Dependencies: installed pre-commit and its pinned hook
  environments; no new runtime dependency or provider API change.
- Ownership: local history and the sibling org-infra sync manifest
  indicate the target configuration and SPDX script are local.
  Recheck before implementation; shared policy changes require a
  separate upstream proposal rather than expanding this change.
- Governance: this proposal does not authorize a protected gate
  change. Implementation MUST stop before editing hook policy
  unless the applicable maintainer authorization is established.

Issue: https://github.com/complytime/complyctl/issues/972
