# Tasks

## 1. Resolve the Four Acceptance Lint Findings

- [ ] 1.1 Record Go and golangci-lint versions, compare them with
  the project and CI context, and capture the existing acceptance-tagged
  lint output; verify the two G704 and two G306 locations against the
  unchanged source without treating version differences as regressions.
- [ ] 1.2 Confirm that Compose supplies the registry and policy inputs,
  then add finding-local G704 annotations with trust-boundary comments
  at the two preflight requests; verify acceptance-tagged lint no longer
  reports those two findings and request behavior is unchanged.
- [ ] 1.3 Change only the two reported test configuration writes to
  owner-only permissions and add focused permission/readability
  assertions where needed; verify those assertions and confirm the
  two G306 findings are resolved.

## 2. Verify the Scoped Fix

- [ ] 2.1 Run acceptance-tagged golangci-lint over
  `./tests/acceptance/...` with the recorded tool context; verify the
  four reported findings are resolved and report any other failures
  separately without expanding the change.
- [ ] 2.2 Run the lifecycle and verification acceptance commands
  derived from `.github/workflows/`; verify private registry preflight,
  seed checks, and CLI configuration access still work in both profiles.
- [ ] 2.3 Run applicable existing lint and formatting checks and assess
  CHANGELOG, AGENTS, and README impact; verify the diff is limited to
  these fixes and directly required assertions or change entries, with
  no scanner, CI, workflow-gate, or unrelated validation changes.

## Workflow follow-up

- Run the required review workflow after implementation and before creating a pull request.
- Archive the change after the project's review requirements are satisfied.
