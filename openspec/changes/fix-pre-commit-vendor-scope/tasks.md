# Tasks

Agent implementation of protected hook edits is blocked by the
gatekeeping rule in `AGENTS.md`. No exception or authorization
process is documented. The unchecked tasks retain the proposed
implementation scope; they do not grant permission to edit gates.
A human contributor can perform task 2.4's protected edits manually;
the agent can then review and verify them without changing gates.

## 1. Establish implementation prerequisites

- [x] 1.1 Verify reviewed specs are committed and pushed.
  Before source edits, record the spec commit and remote evidence.
  After a scope amendment, repeat this verification for the revised
  artifacts and obtain a fresh spec review before implementation.
- [x] 1.2 Check the gatekeeping rule and report the blocker.
  Record the prohibition on agent edits to quality/governance
  gates and the absence of a documented exception process.
  Stop agent execution before protected hook edits; do not treat
  spec review or maintainer approval as an exception to the rule.
- [x] 1.3 Verify local hook ownership and record the evidence.
  Check local history and the org-infra sync manifest; stop for
  separate upstream planning if the policy is shared infrastructure.
- [x] 1.4 Verify hook prerequisites and record baseline versions.
  Record the revision and Git, Python, pre-commit, and Go versions;
  verify required hook environments can initialize
  in disposable state without modifying the user's checkout.

## 2. Correct hook scope with regression coverage

- [x] 2.1 Build real-hook fixtures; verify isolated execution.
  Add normal Git fixture repositories in
  `tests/pre_commit_scope_test.py` using pytest test functions and
  fixtures, the candidate configuration, and SPDX script; do not
  introduce `unittest.TestCase` subclasses (Python pack TC-001).
  Verify pytest is installed for the selected Python interpreter;
  verify `go` and `gofmt` match the toolchain declared in `go.mod`.
  Verify actual hook diagnostics and bounded
  failure reporting for missing prerequisites. Cover FR-007.
- [x] 2.2 Add scope tests; verify they reproduce the original bug.
  Cover vendor byte preservation for staged and all-files runs,
  plus first-party failures and compliant controls for each
  formatter and all SPDX extensions. Include shebang, vendor-like
  paths, and second-run checks; verify failures reproduce before
  the exclusions and cover FR-001 through FR-003 and FR-007.
  For FR-008, cover all three exempt command paths for both
  whitespace/EOF hooks under staged/all-files selection. Assert
  exact bytes with eligible controls proving hook execution. Verify
  neighboring commands, prefix/suffix lookalikes, nested copies,
  and archived specs remain selected. Independently remove only
  the command exemption or only the vendor exemption in disposable
  fixtures; verify the corresponding preservation cases fail.
  For FR-009, execute pinned `go-fmt` on minimal valid Go fixtures
  with staged/all-files vendor and first-party Go controls. Check
  vendor bytes, non-Apache notices, defective/compliant maintained
  inputs, vendor-like paths, repair exit status, and idempotence.
  Remove only its vendor exclusion in a fresh disposable fixture;
  verify vendor-preservation failures and audit actual changed paths.
- [x] 2.3 Test stage and file selection; verify hook execution.
  Add staged-versus-all-files fixtures and stage checks; verify an
  unstaged committed defect is found only by all-files
  selection and intended hooks actually execute. Include `go-fmt`
  at pre-push, preserving all existing stage assignments. Cover
  FR-004 and FR-009.
- [x] 2.4 Hand off hook edits to a human; verify the manual changes.
  A human contributor sets the whitespace/EOF exclusions to the
  exact vendor-or-three-command expression in design decision 1.
  `go-fmt` and `check-spdx` each use `exclude: ^vendor/`.
  No directory-wide command or archive exclusion is permitted.
  The agent MUST NOT apply or repair these protected hook edits.
  After the manual handoff, inspect the diff and verify regression
  tests pass and independent disposable mutations detect removal
  of either exclusion branch and `go-fmt`'s exclusion. Cover FR-001,
  FR-008, and FR-009.
- [x] 2.5 Test unrelated selection; verify other controls remain.
  Use an invalid vendored YAML fixture with `check-yaml`; verify
  its existing failure remains and scanner settings, stages, hook
  revisions, `go-unit-tests`, and hooks outside the four affected
  IDs are unchanged. Cover FR-005.
- [x] 2.6 Add the test target and guidance; verify commands and lint.
  Add `make test-pre-commit-scope` to invoke
  `python3 -m pytest tests/pre_commit_scope_test.py`;
  document pytest as a development/test prerequisite, not a runtime
  dependency, along with existing prerequisites, scope, and stages in
  `docs/STYLE_GUIDE.md`. Enumerate the three whitespace/EOF command
  exceptions, vendor-only `go-fmt` and SPDX exclusions, and checks
  on other commands and archives. Include `go-fmt` staged/all-files
  pre-push commands and the Go version from `go.mod`. Verify the
  target and documented commands in disposable fixtures; lint
  changed Python/YAML/Markdown.
- [x] 2.7 Document the fix; verify documentation impact is covered.
  Add a `CHANGELOG.md` fix entry with this spec path;
  assess the new command's `AGENTS.md` documentation impact and
  record any protected agent-setting edit as blocked. Verify README
  needs no description update and no protected settings changed.

## 3. Reconcile first-party baseline findings

- [x] 3.1 Compare baseline and candidate; verify vendor preservation.
  Run the four affected hooks individually in disposable normal
  checkouts at their configured stages; verify vendor bytes remain
  unchanged in the candidate, along with all three exempt commands,
  and record per-path first-party findings with tool versions.
  Attribute baseline/candidate `go-fmt` mutations separately under
  the declared Go toolchain; record vendor exclusion, not repairs.
- [x] 3.2 Repair scoped first-party findings; verify or track each.
  Fix only approved whitespace/EOF findings and rerun the affected
  hooks to verify each repair. Record exact path,
  hook, reason, and follow-up disposition under #972 for every
  remaining finding. Record each exempt command's baseline defect
  as preserved scaffolded content with a source-template follow-up;
  exclusion is not a repair. Verify the archived design's scoped
  whitespace repair without excluding other archived content.
- [x] 3.3 Deliver the inventory; verify all findings have dispositions.
  Provide a copy-ready verification handoff for manual publication;
  distinguish verified repairs, vendor exclusions on four hooks,
  the three whitespace/EOF command exceptions, and unresolved
  findings. Verify every finding has a disposition and no historical
  count is used as a pass criterion.
  Cover FR-006.

## 4. Verify repository integration

- [x] 4.1 Run regression tests; verify success and checkout isolation.
  Run `make test-pre-commit-scope`; verify all cases pass
  after cleanup, missing prerequisites fail visibly, and temporary
  repositories leave no changes in the developer checkout.
- [x] 4.2 Verify full-stage and staged runs against the inventory.
  In a disposable candidate checkout, run full all-files
  checks separately at pre-commit and pre-push, then exercise staged
  checks. Verify vendor and scaffolded-command byte preservation.
  Verify exit statuses, exclusions, and first-party findings agree
  with the recorded inventory. Report tracked
  baseline failures explicitly rather than claiming a clean run.
- [x] 4.3 Run applicable local CI checks; verify and classify results.
  Derive the checks from
  `.github/workflows/ci_checks.yml` and `ci_local.yml`, including
  delegated lint settings and the local unit/E2E/integration jobs;
  verify results against project toolchain versions and classify
  any pre-existing or environmental failures without changing gates.
- [x] 4.4 Validate specs and patch; resolve pre-PR council findings.
  Run `openspec validate fix-pre-commit-vendor-scope --strict`,
  inspect the complete diff, and run `/uf.review-council` before
  PR submission; verify required findings are resolved and no vendor
  or unrelated user files enter
  the proposed patch.
