# Proposal

## Why

Four pre-existing acceptance-test gosec findings block `/uf.unleash`
on another PR even though that PR did not introduce them. This change
resolves those two G704 and two G306 findings in the test code.
Standalone yamllint also scans vendored YAML and a generated report
that conflicts with repository style rules. Include the two explicit
exclusions manually added by the contributor to address that blocker.

## What Changes

- Resolve the two G704 findings at the registry preflight requests in
  `tests/acceptance/acceptance_test.go`, preserving private Compose
  registry access and existing failure behavior.
- Write test configuration with owner-only permissions at the two G306
  sites in `helpers_test.go` and `verification_helpers_test.go`.
- Verify the four findings with the existing acceptance-tagged lint
  command and run the affected acceptance tests.
- Exclude only `vendor/` and
  `governance/reports/evaluation-log.yaml` from standalone yamllint.
  Keep all existing style rules and checks on other files enabled.

## Capabilities

### New Capabilities

- `acceptance-test-security`: Keeps acceptance registry preflight
  requests compatible with trusted test infrastructure and generated
  configuration private to the test user.
- `yaml-lint-scope`: Excludes vendored YAML and the specific generated
  behavioral evaluation report while retaining checks on maintained
  YAML files.

### Modified Capabilities

- None.

## Impact

- Addresses the four acceptance-lint findings and the two YAML input
  exclusions in issue #965. Other repository-validation work remains
  outside this change.
- Affects the three named acceptance test files, directly needed
  regression assertions, and the two ignore entries in `.yamllint.yml`.
- No new validation targets, tool-version enforcement, CI changes,
  vendor rewrites, pre-commit exclusions, or workflow changes.
- The YAML input scope changes explicitly; existing lint rules,
  security scanners, and `/uf.unleash` execution policy remain intact.
