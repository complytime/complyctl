# Proposal

## Why

Four pre-existing acceptance-test gosec findings block `/uf.unleash`
on another PR even though that PR did not introduce them. This change
resolves those two G704 and two G306 findings in the test code.

## What Changes

- Resolve the two G704 findings at the registry preflight requests in
  `tests/acceptance/acceptance_test.go`, preserving private Compose
  registry access and existing failure behavior.
- Write test configuration with owner-only permissions at the two G306
  sites in `helpers_test.go` and `verification_helpers_test.go`.
- Verify the four findings with the existing acceptance-tagged lint
  command and run the affected acceptance tests.

## Capabilities

### New Capabilities

- `acceptance-test-security`: Keeps acceptance registry preflight
  requests compatible with trusted test infrastructure and generated
  configuration private to the test user.

### Modified Capabilities

- None.

## Impact

- Relates to the four acceptance-lint findings in issue #965, not its
  wider repository-validation work.
- Implementation is limited to the three named acceptance test files
  and regression assertions directly needed for these fixes.
- No new validation targets, tool-version enforcement, CI changes,
  vendor rewrites, YAML/pre-commit exclusions, or workflow changes.
- Existing lint rules and `/uf.unleash` gates remain enabled.
