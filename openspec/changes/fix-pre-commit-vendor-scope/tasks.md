# Tasks

Agent implementation of protected hook edits is blocked by the
gatekeeping rule in `AGENTS.md`. No exception or authorization
process is documented. The unchecked tasks retain the proposed
implementation scope; they do not grant permission to edit gates.
A human contributor can perform task 2.4's protected edits manually;
the agent can then review and verify them without changing gates.

## 1. Establish implementation prerequisites

- [ ] 1.1 Verify reviewed specs are committed and pushed.
  Before source edits, record the spec commit and remote evidence.
- [ ] 1.2 Check the gatekeeping rule and report the blocker.
  Record the prohibition on agent edits to quality/governance
  gates and the absence of a documented exception process.
  Stop agent execution before protected hook edits; do not treat
  spec review or maintainer approval as an exception to the rule.
- [ ] 1.3 Verify local hook ownership and record the evidence.
  Check local history and the org-infra sync manifest; stop for
  separate upstream planning if the policy is shared infrastructure.
- [ ] 1.4 Verify hook prerequisites and record baseline versions.
  Record the revision and Git, Python, pre-commit, and Go versions;
  verify required hook environments can initialize
  in disposable state without modifying the user's checkout.

## 2. Correct hook scope with regression coverage

- [ ] 2.1 Build real-hook fixtures; verify isolated execution.
  Add normal Git fixture repositories in
  `tests/pre_commit_scope_test.py` using the candidate configuration
  and SPDX script; verify actual hook diagnostics and bounded
  failure reporting for missing prerequisites. Cover FR-007.
- [ ] 2.2 Add scope tests; verify they reproduce the original bug.
  Cover vendor byte preservation for staged and all-files runs,
  plus first-party failures and compliant controls for each
  formatter and all SPDX extensions. Include shebang, vendor-like
  paths, and second-run checks; verify failures reproduce before
  the exclusions and cover FR-001 through FR-003 and FR-007.
- [ ] 2.3 Test stage and file selection; verify hook execution.
  Add staged-versus-all-files fixtures and stage checks; verify an
  unstaged committed defect is found only by all-files
  selection and intended hooks actually execute. Cover FR-004.
- [ ] 2.4 Hand off hook edits to a human; verify the manual changes.
  A human contributor adds only `exclude: ^vendor/` to
  `trailing-whitespace`, `end-of-file-fixer`, and `check-spdx`.
  The agent MUST NOT apply or repair these protected hook edits.
  After the manual handoff, inspect the diff and verify regression
  tests now pass and a disposable copy
  with the exclusions removed fails vendor-preservation cases.
- [ ] 2.5 Test unrelated selection; verify other controls remain.
  Use an invalid vendored YAML fixture with `check-yaml`; verify
  its existing failure remains and scanner settings, stages, hook
  revisions, and other hook definitions are unchanged. Cover FR-005.
- [ ] 2.6 Add the test target and guidance; verify commands and lint.
  Add `make test-pre-commit-scope` using unittest discovery;
  document prerequisites, scope, and both stages in
  `docs/STYLE_GUIDE.md`. Verify the target and documented commands
  in disposable fixtures and lint the changed Python/YAML/Markdown.
- [ ] 2.7 Document the fix; verify documentation impact is covered.
  Add a `CHANGELOG.md` fix entry with this spec path;
  assess the new command's `AGENTS.md` documentation impact and
  record any protected agent-setting edit as blocked. Verify README
  needs no description update and no protected settings changed.

## 3. Reconcile first-party baseline findings

- [ ] 3.1 Compare baseline and candidate; verify vendor preservation.
  Run the three affected hooks individually in disposable normal
  checkouts at their configured stages; verify vendor bytes remain
  unchanged in the candidate
  and record per-path first-party findings with tool versions.
- [ ] 3.2 Repair scoped first-party findings; verify or track each.
  Fix only approved whitespace/EOF findings and rerun the affected
  hooks to verify each repair. Record exact path,
  hook, reason, and follow-up disposition under #972 for every
  remaining finding, including protected command templates.
- [ ] 3.3 Deliver the inventory; verify all findings have dispositions.
  Provide a copy-ready verification handoff for manual publication;
  verify every finding is resolved or explicitly tracked and no
  historical count is used
  as a pass criterion. Cover FR-006.

## 4. Verify repository integration

- [ ] 4.1 Run regression tests; verify success and checkout isolation.
  Run `make test-pre-commit-scope`; verify all cases pass
  after cleanup, missing prerequisites fail visibly, and temporary
  repositories leave no changes in the developer checkout.
- [ ] 4.2 Verify full-stage and staged runs against the inventory.
  In a disposable candidate checkout, run full all-files
  checks separately at pre-commit and pre-push, then exercise staged
  checks; verify exit statuses, vendor preservation, and first-party
  findings agree with the recorded inventory. Report tracked
  baseline failures explicitly rather than claiming a clean run.
- [ ] 4.3 Run applicable local CI checks; verify and classify results.
  Derive the checks from
  `.github/workflows/ci_checks.yml` and `ci_local.yml`, including
  delegated lint settings and the local unit/E2E/integration jobs;
  verify results against project toolchain versions and classify
  any pre-existing or environmental failures without changing gates.
- [ ] 4.4 Validate specs and patch; resolve pre-PR council findings.
  Run `openspec validate fix-pre-commit-vendor-scope --strict`,
  inspect the complete diff, and run `/uf.review-council` before
  PR submission; verify required findings are resolved and no vendor
  or unrelated user files enter
  the proposed patch.
