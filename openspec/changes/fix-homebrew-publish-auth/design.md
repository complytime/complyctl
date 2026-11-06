# Design

## Context

See proposal.md for the failure history. The publishing step
exports `TAP_TOKEN`, but GitHub CLI reads `GH_TOKEN`. Inline
assignments on `gh` commands do not reach the later Git push.
The security-sensitive boundary is the credential helper
spawned by Git, not App token generation.

## Goals / Non-Goals

**Goals:** Make the existing credential inherit correctly and
prove the behavior without production credentials or writes.

**Non-Goals:** Change release gates, App permissions, branch
protection, formula generation, or existing retry semantics.
Do not fix the unrelated catch-all commit failure handler.

## Decisions

### Use one step-scoped token variable

Rename the publishing environment entry to `GH_TOKEN` with the
same App action output. Remove every inline `TAP_TOKEN`
assignment. Keep credential-helper setup, optionally scoped
explicitly to github.com. The helper reads the token when Git
invokes it; setup does not save an environment-supplied token.

An inline assignment on Git push alone would fix this failure
but retain redundant naming and invite the same mistake later.
Persisting a token or embedding one in a URL is unnecessary.

### Exercise the workflow's actual publishing script

Add `tests/homebrew_publish_test.sh`, using the existing shell
test plus Python standard-library harness convention. Extract
the uniquely identified publishing step's environment and run
block from release.yml, failing if its expected shape changes.
Resolve only the known App output and tag expressions to test
values; never evaluate arbitrary workflow expressions.

Run the extracted script in a temporary working directory with
an allowlisted environment, isolated HOME and Git config, and
noninteractive Git. Stub download, clone, push transport, and
PR operations so no network request or remote mutation occurs.
Verify API commands inherit the dummy token. For the push
interception, invoke real Git credential fill and the real gh
credential helper, with a command-scoped helper configuration;
assert the supplied credential without printing its value.

Cover success, the original TAP_TOKEN-only environment,
existing-open-PR no-op, and push failure propagation. Mutating
the fixed environment back to TAP_TOKEN in a temporary fixture
MUST fail the success assertion, proving regression detection.
Keep test subprocesses bounded and reject unknown commands.

Expose `make test-homebrew-publish`; leave the installation
smoke test and existing CI gates unchanged. Tests require Bash,
Python 3, Git, and gh, with no new Python package dependency.

## Risks / Trade-offs

- Offline tests cannot prove server-side App authorization.
  Mitigation: report live push/PR validation as a separately
  authorized follow-up, not as completed validation.
- Workflow extraction can become stale. Mitigation: reject
  missing or ambiguous publishing steps rather than silently
  running a copied script or skipping the test.
- An older failed run retains the old workflow revision.
  Mitigation: document manual formula PR recovery for v1.2.0;
  do not rerun the full release or move the existing tag.

## Migration Plan

Commit and push these spec artifacts before implementation.
Implement on the same feature branch and run focused tests and
applicable configured workflow/documentation linters locally.
Update release documentation and the changelog; assess AGENTS
for the added test command. Run the required review council
before any implementation PR submission.

After merge, future releases use the corrected environment.
Recover v1.2.0 through a separately authorized manual formula
PR without recreating its release. Rollback, if necessary,
reverts the workflow-only fix through a reviewed PR, without
changing release tags or existing tap PRs.
