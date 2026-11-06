# Tasks

## 1. Preview coordinator and focused tests

- [ ] 1.1 Add the Python coordinator with centralized upstream and
  timeout constants, prerequisite checks, bounded subprocesses, and
  secret-safe failures; verify missing tools, YAML support, invalid
  versions, authentication failures, and timeouts with unit tests.
- [ ] 1.2 Implement automatic latest-release discovery and one resolved
  HEAD comparison without a baseline override; verify canonical API
  requests, inherited override rejection, malformed responses, and
  ahead, identical, behind, diverged, and unavailable candidates.
- [ ] 1.3 Implement canonical remote selection and process-local Git
  overrides; verify fork-origin, upstream preference, branch-name
  independence, detached-checkout failures, and unchanged persistent
  settings in isolated test repositories.
- [ ] 1.4 Generate the minimal temporary GoReleaser configuration and
  invoke its changelog pipeline; verify exact changelog-setting
  preservation, build and hook suppression, safe skip flags, output
  channels, cleanup on success and failure, and no partial Markdown.
- [ ] 1.5 Add `tests/release_notes_preview_test.py` and document helper
  usage, prerequisites, and committed-candidate limits in
  `docs/RELEASE_PROCESS.md`; verify the suite runs using Python's
  standard-library test runner and all documented prerequisites match
  the helper checks.

## 2. Make interface and documentation gate

- [ ] 2.1 Add quiet, phony `release-notes-preview` and
  `test-release-notes-preview` Make targets without a `FROM` input;
  verify Make invocation, help entries, and redirectable Markdown
  with focused regression tests and `make help`.
- [ ] 2.2 Complete release-process instructions for both targets and
  empty or failed previews; verify documented commands locally and
  explain that previewing does not select a tag or publish a release.
- [ ] 2.3 Add a focused changelog entry and assess `AGENTS.md` command
  documentation and `README.md` impact; verify changes are limited
  to this helper and record why unrelated description changes are
  unnecessary.

## 3. Integrated verification before local handoff

- [ ] 3.1 Run `make test-release-notes-preview`, Python lint and format
  checks, and changed-file Markdown checks; verify positive and
  negative cases pass without network access or publishing tools.
- [ ] 3.2 Run a real `make release-notes-preview` against automatic
  upstream discovery; verify the candidate and baseline diagnostics,
  Markdown or explicit empty result, and unchanged working-tree
  contents, tags, and persistent Git configuration.
- [ ] 3.3 Verify non-empty rendering against generated published notes
  using an isolated historical fixture; compare after trimming
  trailing whitespace and confirm no public baseline override exists.
- [ ] 3.4 Replicate applicable checks from the current CI workflows:
  `buf lint`, `make api-compatibility`, `make test-unit`,
  `make test-e2e`, `make test-integration`, and `make lint`;
  record results and report blockers without changing gates;
  leave incomplete verification unchecked.
- [ ] 3.5 Run strict OpenSpec validation and `git diff --check`;
  inspect the complete diff and provide local testing
  instructions; verify no PR has been opened.

## Workflow follow-up

- Wait for the maintainer's local testing before PR submission.
- Run `/uf.review-council` and resolve requested changes before a PR.
- Archive the change after implementation and review acceptance.
