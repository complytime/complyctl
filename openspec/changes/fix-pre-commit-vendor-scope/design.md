# Design

## Context

See `proposal.md` for motivation and the `project-hook-scope` delta
for FR-001 through FR-008. This design changes hook selection,
not formatter rules or first-party licensing policy.

`.pre-commit-config.yaml` defaults hooks to `pre-push`; the local
SPDX hook explicitly uses `pre-commit` and selects `.py`, `.go`,
and `.sh` files. The original configuration excludes neither vendor
nor the three scaffolded commands identified below.
The SPDX script prepends Apache-2.0 when its first-three-line check
fails. Yamllint and MegaLinter exclusions are independent.

Local history records the original hook setup in `2e9208aa` and
the SPDX hook/script addition in `302b9523`. Neither target appears
in the inspected sibling org-infra `sync-config.yml` manifest.
This supports repository-local ownership; implementation MUST
recheck that evidence rather than assume a shared template change.

`AGENTS.md` protects quality/governance gates. These artifacts
propose the scope correction but do not grant an exception.
The rule prohibits agent edits and requires stopping and reporting;
it defines no maintainer-authorization exception. Neither the
constitution nor contributor guidance documents such a process.
Ordinary PR review and constitution amendments are separate from
this prohibition. Agent implementation of the hook edits is blocked
under the current instructions. Reviewed spec artifacts MUST also
be committed and pushed before implementation.

## Goals / Non-Goals

**Goals:**

- Make ownership selection explicit at each affected hook.
- Test real pre-commit dispatch, stages, file selection, return
  codes, and file mutations rather than only matching regexes.
- Separate current baseline findings from regressions without
  changing upstream files or hiding maintained inputs.

**Non-Goals:**

- Reimplement formatters or introduce an SPDX license parser.
- Change direct invocation behavior of the SPDX script; the
  repository hook configuration owns this selection boundary.
- Change hook stages, Go hooks, security scanners, CI policy,
  shared org-infra templates, or agent/workflow definitions.
- Normalize the vendor tree or repair every historical document.

## Decisions

### 1. Put root-anchored exclusions on three hooks

The proposed fix uses this hook-level exclusion on both
`trailing-whitespace` and `end-of-file-fixer`:

```yaml
exclude: '^(vendor/|\.opencode/commands/uf\.(review-council|review-pr|address-feedback)\.md$)'
```

It preserves the root `vendor/` tree and exactly three uf-scaffolded
files under `.opencode/commands/`: `uf.review-council.md`,
`uf.review-pr.md`, and `uf.address-feedback.md`. The command branch
is anchored to each complete path. Other commands, name lookalikes,
and nested copies remain eligible for the formatting hooks.

The SPDX hook retains `exclude: ^vendor/`; Markdown commands do
not match its existing extension filter. Retain pinned revisions,
rules, type filters, and stages for all hooks. Do not exclude
`.opencode/` or `openspec/changes/archive/` as whole directories.

This is a proposed configuration change, not an executable agent
instruction under the current gatekeeping rule. Agents MUST stop
and report before applying it; a recorded approval does not supply
an exception absent from the governing instructions.

A human contributor can apply the three exclusions manually.
After that handoff, the agent can inspect the actual diff, verify
that only the proposed hook scope changed, and run the regression
and integration checks. Any further protected hook edit remains
manual. Ordinary PR review still applies.

A global exclusion would change unrelated hook selection. A broad
`vendor` substring would hide maintained files. Duplicating path
policy inside the SPDX script would leave the formatter problem
unresolved and create a second selection boundary. None is needed.

### 2. Exercise the actual configuration in disposable repositories

Add `tests/pre_commit_scope_test.py` using pytest test functions
and fixtures, type annotations, and isolated temporary directories.
Python pack TC-001 requires pytest for new tests; do not introduce
`unittest.TestCase` subclasses. Copy the
candidate hook configuration and SPDX script into each fixture's
normal Git repository. Use argument-list subprocess calls with
explicit working directories, captured diagnostics, and bounded
timeouts. Never install hooks into the user's checkout.

Invoke one affected hook ID at a time through pre-commit so the
tests exercise its actual pinned implementation and selection
without running unrelated Go test hooks. Do not replace hooks
with stubs or construct an independent test-only exclusion policy.
Most fixtures need no commits: `git add` supplies tracked paths,
including forced additions where needed.

For the staged/all-files distinction, create a small committed
fixture baseline containing an eligible defect, then stage a
separate compliant change. Set fixture commit identity through
command-local environment variables; do not change user Git
configuration. Use an empty temporary hooks directory for fixture
commits so global commit hooks do not interfere with test setup.
These are fixture commits only, never commits in the source repo.

The fixture matrix MUST cover:

- Vendor Go, Python, shell, and text files with formatting defects
  and existing non-Apache license notices; assert exact bytes.
- All three exempt commands with trailing whitespace and invalid
  EOFs, separately for each formatter and staged/all-files runs;
  assert exact bytes and no findings on those paths. Include an
  eligible first-party control proving that the hook executes.
- Maintained command neighbors and path lookalikes with defects:
  `uf.other.md`, `uf.review-pr.md.bak`, `xuf.review-pr.md`, and a
  nested copy of `.opencode/commands/uf.review-pr.md`. Each MUST
  remain selected and corrected by both formatters. Archived spec
  fixtures MUST also remain selected.
- Matching first-party failures for each formatter and each SPDX
  file extension; verify the correction and nonzero exit status.
- Clean first-party inputs and second-run idempotence.
- Shebang preservation and vendor-like maintained path names.
- Staged-only and all-files selection at each configured stage.
- Stage assignments and unchanged unrelated controls.

Require evidence that each intended hook actually ran; a
wrong-stage skip is not a passing behavioral check. Excluded-only
selection may legitimately report no eligible files. Include
eligible first-party controls in mixed fixtures to distinguish
valid exclusion from disabled execution.

Mutation checks MUST independently remove each exclusion branch
in disposable fixtures. Removing only the command branch, while
keeping vendor excluded, MUST make command-preservation cases
fail for each formatter. Removing only the vendor branch MUST
make vendor-preservation cases fail while keeping the command
exceptions intact. For SPDX, remove its vendor-only exclusion.
These checks MUST NOT edit the developer's hook configuration.

### 3. Make prerequisites and stage coverage explicit

Add `make test-pre-commit-scope` to invoke
`python3 -m pytest tests/pre_commit_scope_test.py`. Require Git,
Python 3.10 or newer
(matching the existing script syntax), pre-commit with support for
the configured stage names, pytest installed for that Python
interpreter, and installed hook environments. Document pytest as
a development/test prerequisite, not a runtime dependency.

Initial environment setup may need network access for the pinned
pre-commit-hooks revision. Use an isolated `PRE_COMMIT_HOME` for
test-created state, reusable within the suite. Missing tools,
download failures, and timeouts MUST fail with actionable output;
they MUST NOT become skipped tests or implicit success.

Document separate execution of SPDX at pre-commit and the two
formatters at pre-push. Show both staged and all-files forms.
List the three command paths as formatter-only exceptions and
explain that the SPDX exemption remains vendor-only. Do not
describe excluded template defects as fixed formatting issues.
An unqualified all-files run alone does not cover both stages.
Full-stage runs remain a separate repository verification step.

### 4. Inventory baseline findings before accepting cleanup

Use two disposable normal checkouts: the recorded baseline and the
candidate snapshot. Keep candidate uncommitted artifacts available
in its snapshot without carrying unrelated user files. Record
revision, candidate diff, Python/pre-commit/Go versions, stage,
hook ID, exit status, and changed paths. Compare vendor bytes
before and after candidate hook runs.

Run the three hooks individually to attribute their mutations.
For each of the three exempt commands, record its exact path,
the baseline hook finding, and the disposition "excluded from
formatters; preserved uf-scaffolded bytes". Verify its candidate
bytes are unchanged; do not count its disappearance from formatter
output as a repair. Retain a source-template follow-up under #972
in the copy-ready handoff, without posting to GitHub.

Apply only verified first-party whitespace/EOF repairs within the
approved scope, including ordinary archived spec content. The
known archived design whitespace finding is a repair candidate,
not a reason to exclude archives. Do not mass-copy hook output
back to the working tree. For other protected or out-of-scope
findings, record exact path, hook, reason, and follow-up under
#972 in the verification handoff for the user to publish manually.
Do not create a broad exclusion to turn those findings green.

The verification handoff MUST state whether full runs passed or
remain blocked by tracked findings. Original issue counts are
historical observations, not test expectations.

### 5. Keep security and contributor guidance aligned

Compare the candidate configuration with baseline to show only
three hook-level exclusions. Verify unrelated hook definitions,
pinned revisions, and scanner configuration are unchanged.
Run an existing unrelated validation hook against an invalid
fixture that it previously selected to catch a global exclusion.

Extend `docs/STYLE_GUIDE.md` with scope, stages, prerequisites,
disposable-checkout guidance, and the regression target. Add a
`CHANGELOG.md` entry referencing this change. Assess `AGENTS.md`
for the new test command; record any needed protected agent-setting
edit as blocked under the existing rule. README needs no description
change because runtime behavior is unaffected.

## Risks / Trade-offs

- Hook exclusions affect a protected gate → agent implementation
  is blocked; stop and report rather than invent an approval path.
- Cached environments or stage skips hide failures → use isolated
  state, real hook execution, and positive first-party controls.
- Network setup fails → report an environment failure with
  diagnostics, not a passing or skipped regression suite.
- Repeated baseline runs rewrite vendor content → run only in
  disposable checkouts and retain byte comparisons.
- Protected first-party findings remain → track each explicitly
  and report an incomplete full-run result without weakening gates.
- Exact command exceptions preserve known formatting defects →
  report their exclusion disposition and source-template follow-up;
  regression controls MUST prevent a directory-wide exemption.
- Direct SPDX script callers can still pass vendor paths → document
  the hook boundary; script-wide hardening is outside this fix.

## Migration Plan

1. Review and commit/push these spec artifacts using the normal
   workflow. Record that agent hook edits are blocked by
   `AGENTS.md`; no gate-change authorization process is documented.
2. Add regression coverage without editing protected hook policy.
3. Hand off the three hook exclusions to a human contributor for
   manual editing. Stop agent work that depends on those edits
   until they are present; inspect the diff before verification.
4. Verify baseline/candidate behavior, perform scoped cleanup, and
   update contributor guidance.
5. Run relevant local CI checks and the pre-PR review council.

No runtime migration is required. Rollback restores the three
hook definitions through an ordinary reviewed change; it does not
rewrite vendor content or remove existing security checks.
