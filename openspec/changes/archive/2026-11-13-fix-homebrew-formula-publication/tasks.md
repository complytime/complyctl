# Tasks

## 1. Credential Propagation Repair

- [x] 1.1 Run the existing Homebrew formula branch push with `GH_TOKEN` set
  from `TAP_TOKEN`; verify the remote URL and Git configuration contain no
  token.

## 2. Regression Coverage

- [x] 2.1 Add focused coverage that proves the Git credential helper receives
  a synthetic `GH_TOKEN` during the formula branch push and verify the prior
  scoped-only invocation fails; verify the token is neither logged nor written
  outside the push child-process scope.
- [x] 2.2 Run the affected workflow validation and regression coverage; verify
  the existing branch-and-PR publication flow remains unchanged.

## Workflow follow-up

- Run `/uf.review-council` and resolve all blocking findings before PR
  submission.

<!-- spec-review: passed -->

<!-- code-review: passed -->
