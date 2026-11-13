# Tasks

## 1. Resolve the Four Acceptance Lint Findings

- [x] 1.1 Record Go and golangci-lint versions, compare them with
  the project and CI context, and capture the existing acceptance-tagged
  lint output; verify the two G704 and two G306 locations against the
  unchanged source without treating version differences as regressions.
- [x] 1.2 Confirm that Compose supplies the registry and policy inputs,
  then add finding-local G704 annotations with trust-boundary comments
  at the two preflight requests; verify acceptance-tagged lint no longer
  reports those two findings and request behavior is unchanged.
- [x] 1.3 Change only the two reported test configuration writes to
  owner-only permissions and add focused permission/readability
  assertions where needed; verify those assertions and confirm the
  two G306 findings are resolved.

## 2. Verify the Contributor's YAML Scope Fix

- [x] 2.1 Verify the contributor-added `.yamllint.yml` change contains
  only the `vendor/` and exact generated-report exclusions; confirm
  the base configuration, rules, security scanners, and CI are intact.
- [x] 2.2 Use disposable fixtures to verify malformed YAML under each
  excluded path is ignored while malformed maintained YAML and a
  sibling governance YAML file still fail; do not add a new framework
  or commit intentionally invalid fixtures.
- [x] 2.3 Run `yamllint .`; verify excluded inputs no longer contribute
  findings even when the generated report exists, and distinguish
  remaining first-party warnings from errors without disabling rules.

## 3. Verify the Scoped Fix

- [x] 3.1 Run acceptance-tagged golangci-lint over
  `./tests/acceptance/...` with the recorded tool context; verify the
  four reported findings are resolved and report any other failures
  separately without expanding the change.
- [x] 3.2 Run the lifecycle and verification acceptance commands
  derived from `.github/workflows/`; verify private registry preflight,
  seed checks, and CLI configuration access still work in both profiles.
- [x] 3.3 Run applicable existing lint and formatting checks and assess
  CHANGELOG, AGENTS, and README impact; verify the diff is limited to
  the acceptance fixes, two YAML exclusions, and directly required
  assertions or change entries, with no security scanner, CI,
  workflow-policy, or unrelated validation changes.

## Workflow follow-up

- Run the required review workflow after implementation and before creating a pull request.
- Archive the change after the project's review requirements are satisfied.
