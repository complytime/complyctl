# Tasks

## 1. Specification checkpoint

- [ ] 1.1 Validate this change with strict OpenSpec validation,
  then commit and push only its spec artifacts on the feature
  branch before editing implementation files. Verify the remote
  branch contains that commit.

## 2. Publishing authentication and regression coverage

- [ ] 2.1 Add the offline publishing-step harness described in
  design.md and a `test-homebrew-publish` Makefile target. Run
  against the current workflow and confirm the new-formula
  success assertion fails for missing helper credentials.
- [ ] 2.2 Rename the step environment to `GH_TOKEN` and remove
  all inline TAP_TOKEN assignments. Verify the focused test
  passes, existing-PR execution performs no push, and push
  failures remain nonzero without a success notice.
- [ ] 2.3 Verify a temporary TAP_TOKEN-only mutation fails the
  regression test. Confirm real credential-helper invocation,
  isolated config, bounded subprocesses, no network writes,
  and no token values in output or persisted Git config.
- [ ] 2.4 Document authentication and existing-release recovery
  in docs/RELEASE_PROCESS.md and record the fix in CHANGELOG.md.
  Document the added test command in AGENTS.md without changing
  gates or agent settings; assess README impact. Verify docs
  distinguish old-run retries from corrected workflow execution.

## 3. Integration verification

- [ ] 3.1 Run `make test-homebrew-publish` and applicable local
  CI-equivalent checks derived from the configured workflows:
  actionlint, zizmor, ShellCheck, yamllint, and Markdown lint on
  affected files. Record results and any unavailable tools;
  do not weaken gates or claim missing checks passed.
- [ ] 3.2 Inspect the final diff to confirm unchanged App scope,
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
