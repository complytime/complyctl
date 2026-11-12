# Proposal

## Why

Project formatting and SPDX hooks select vendored files, causing
unrelated changes and adding project license metadata to third-party
code. Issue #972 needs a scoped fix so contributors can run the hooks
while preserving upstream sources and first-party checks.

Baseline verification also found EOF defects in three commands
scaffolded by uf. Preserve those managed files rather than apply
project formatting changes that would diverge from their source.

## What Changes

- Exclude the repository-root `vendor/` tree only from
  `trailing-whitespace`, `end-of-file-fixer`, and `check-spdx`.
- Also exclude exactly these scaffolded files from the two
  formatting hooks, with no directory-wide command exemption:
  - `.opencode/commands/uf.review-council.md`
  - `.opencode/commands/uf.review-pr.md`
  - `.opencode/commands/uf.address-feedback.md`
  Keep the SPDX exclusion vendor-only.
- Preserve the existing rules, pinned hook revisions, file-type
  filters, and stage assignments for maintained first-party files.
- Add regression checks using actual hook execution in temporary
  normal Git repositories, covering staged-file and all-files runs.
  Cover each command exception, neighboring paths, and independent
  vendor and command-exception mutation cases.
- Inspect first-party baseline findings in a disposable checkout;
  fix ordinary formatting findings within scope or record exact
  paths and a disposition for protected or out-of-scope findings.
  Record excluded command defects as preserved scaffolded content,
  not formatting repairs. Archived specs remain checked.
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
  named hooks; agent edits are blocked by the gatekeeping rule.
- Tests: a Python regression suite under `tests/`, invoking the
  real hooks and the repository SPDX script through pre-commit.
- Developer tooling: `Makefile`, `docs/STYLE_GUIDE.md`, and a
  contributor-facing changelog entry. Assess `AGENTS.md` for the
  new test command without changing protected agent settings.
- Baseline cleanup: only individually verified first-party
  formatting changes; no vendored or protected template rewrites.
- Dependencies: installed pytest as a development/test prerequisite,
  pre-commit, and its pinned hook environments; no new runtime
  dependency or provider API change.
- Ownership: local history and the sibling org-infra sync manifest
  indicate the target configuration and SPDX script are local.
  Recheck before implementation; shared policy changes require a
  separate upstream proposal rather than expanding this change.
- Agent constraint: `AGENTS.md` prohibits agents from modifying
  quality/governance gates and requires them to stop and report.
  Agent implementation of the proposed hook edits is blocked.

## Implementation Status

No process for authorizing agent edits to protected gates is
documented in `AGENTS.md`, the constitution, or contributor guidance.
The constitution and `docs/CONTRIBUTING.md` require ordinary PR
review; the constitution's amendment procedure applies to changes
to the constitution. Neither defines an exception to gatekeeping.

These artifacts describe the proposed fix but MUST NOT be treated
as permission to change protected gates. Spec review, PR approval,
or a recorded maintainer decision does not establish a documented
exception. Agents MUST stop and report the instruction conflict
before editing hook policy. This change does not amend that rule
or introduce an approval process.

A human contributor MAY apply the three proposed hook exclusions
manually. The agent MAY then review the resulting diff and run
verification without editing protected gates. This handoff does
not waive the repository's ordinary PR review requirements.

Issue: https://github.com/complytime/complyctl/issues/972
