# Tasks

## 1. Specification checkpoint

- [x] 1.1 Validate this change with strict OpenSpec validation,
  then commit and push only its spec artifacts on the feature
  branch before editing implementation files. Verify the remote
  branch contains that commit.

## 2. Publishing authentication and regression coverage

- [x] 2.1 Add the offline publishing-step harness described in
  design.md and a `test-homebrew-publish` Makefile target. Run
  against the current workflow and confirm the new-formula
  success assertion fails for missing helper credentials.
- [x] 2.2 Rename the step environment to `GH_TOKEN` and remove
  all inline TAP_TOKEN assignments. Verify the focused test
  passes, existing-PR execution performs no push, and push
  failures remain nonzero without a success notice.
- [x] 2.3 Verify a temporary TAP_TOKEN-only mutation fails the
  regression test. Confirm real credential-helper invocation,
  isolated config, bounded subprocesses, no network writes,
  and no token values in output or persisted Git config.
- [x] 2.4 Document authentication and existing-release recovery
  in docs/RELEASE_PROCESS.md and record the fix in CHANGELOG.md.
  Document the added test command in AGENTS.md without changing
  gates or agent settings; assess README impact. Verify docs
  distinguish old-run retries from corrected workflow execution.

## 3. Integration verification

- [x] 3.1 Run `make test-homebrew-publish` and applicable local
  CI-equivalent checks derived from the configured workflows:
  actionlint, zizmor, ShellCheck, yamllint, and Markdown lint on
  affected files. Record results and any unavailable tools;
  do not weaken gates or claim missing checks passed.
- [x] 3.2 Inspect the final diff to confirm unchanged App scope,
  PR-based flow, release gates, formula content, and unrelated
  user files. Validate the OpenSpec change strictly again.

## Workflow follow-up

- Run `/uf.review-council` and resolve all requested changes
  before implementation PR submission; do not change code
  between approval and submission.
- Seek separate authorization for live App push/PR validation
  and manual v1.2.0 formula recovery. Do not move release tags
  or rerun the old workflow expecting it to pick up this fix.
- Archive the change after implementation and required review.

## Local verification results

- Strict spec validation passed before implementation. Spec commit
  `c244d61c` was pushed to and verified on the origin feature branch.
- The original publishing environment failed the new-formula success
  assertion at credential lookup. The corrected environment passes.
- `make test-homebrew-publish` passes all six checks, including the
  temporary legacy mutation, real credential-helper resolution,
  existing-PR no-op, and failed-push propagation.
- The applicable linters enabled in `.mega-linter.yml` via
  `.github/workflows/ci_checks.yml` passed: actionlint, offline
  zizmor, ShellCheck, and yamllint on affected workflow/shell/YAML
  files. Ruff also passed for both embedded Python programs.
- Markdown lint was unavailable: neither `markdownlint` nor
  `markdownlint-cli2` is installed on PATH. That check was not
  run and is not claimed as passed.
- README impact: none; the CLI description and usage are unchanged.
- Final diff inspection and strict OpenSpec validation passed. App
  scope, PR flow, release gates, and generated formula content are
  unchanged; existing unrelated untracked files remain untouched.
- No live tap push, PR creation, release rerun, or manual recovery
  was performed. Server-side App authorization remains unverified.
