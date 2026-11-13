# Design

## Context

See `proposal.md` for motivation. The four reported findings are in
acceptance-tagged test code:

- G704: the registry reachability and seeded-policy requests in
  `acceptance_test.go` use `REGISTRY_URL`.
- G306: `writeWorkspaceConfig` in `helpers_test.go` and
  `writeVerificationConfig` in `verification_helpers_test.go` write
  configuration with `0o644` permissions.

The issue reports golangci-lint 2.11.4 locally and 2.13.2 in inspected
CI runs. Those CI commands omit the acceptance build tag. Verify the
toolchain and dependency context before drawing conclusions from a
scanner result; record versions with the before/after lint output.

The phase checkpoint also ran `yamllint .`, which scanned YAML in
`vendor/` and an ignored generated behavioral report. MegaLinter
already excludes `vendor/`, but standalone yamllint does not inherit
that setting or `.gitignore`. The contributor manually added the two
specific exclusions to `.yamllint.yml` and requested their inclusion
in this change. This is an explicit input-scope adjustment, not a
change to YAML formatting rules.

## Goals / Non-Goals

**Goals:**

- Resolve exactly the two G704 and two G306 findings.
- Preserve registry preflight behavior in both acceptance profiles.
- Keep generated configuration readable by the test process and CLI.
- Exclude vendored YAML and the generated evaluation report from
  standalone style lint while retaining checks on maintained YAML.

**Non-Goals:**

- New validation profiles, Make targets, or tool-version enforcement.
- Reformatting vendor files or changing the report generator.
- Broader YAML exclusions, pre-commit changes, or validation cleanup.
- CI, security scanner configuration, agent conventions, or workflow
  execution policy.
- General-purpose URL validation or production security changes.

## Decisions

### Limit G704 dispositions to the two test requests

The test runner supplies `REGISTRY_URL` and `TEST_POLICY_ID` through
the acceptance environment. These values select test infrastructure;
they do not come from a public service request. The runner already
controls the test process and its network access.

Confirm that boundary against the Compose configuration. If confirmed,
use a finding-local G704 annotation at each of the two request sites
with a comment explaining the trusted runner configuration and private
test registry access. Keep timeouts, request paths, seed checks, and
existing error behavior intact. Do not suppress G704 package-wide or
change scanner configuration.

Reject a private-address ban: private Compose endpoints are intentional.
Reject a new URL-validation subsystem: it would expand this test-only
lint fix without addressing a demonstrated untrusted-input boundary.
If the runner-controlled boundary is not confirmed, report that fact
before choosing a different fix.

### Use owner-only test configuration permissions

Change the two reported writes to `0o600`. Both helpers create files
in test workspaces, and the test process launches the CLI to read them.
Verify that both acceptance profiles can still read the configuration.
Retain the existing contents, paths, and directory permissions.

Prefer this direct fix over suppressing G306 or changing repository
file-permission conventions. Owner-only test files do not require a
global convention amendment.

### Verify with existing commands and focused assertions

Use the existing acceptance-tagged golangci-lint invocation to compare
the same four locations before and after the fix. Run the lifecycle
and verification acceptance targets already used by CI. Add focused
permission assertions to the affected tests where needed; do not add
validation-profile infrastructure or suppression-policy test machinery.

### Bound standalone YAML exclusions to two inputs

Retain the contributor's top-level `ignore` entries in
`.yamllint.yml`: `vendor/` and the exact path
`governance/reports/evaluation-log.yaml`. Preserve `extends: default`
and every existing rule. Do not exclude the whole governance tree,
all reports, all ignored files, or other first-party YAML.

The report is reproducible output from `make test-behavioral` and is
already Git-ignored. Its absence must not be necessary for lint to
pass: recreating it must leave it outside yamllint's input set.
Deleting a stale local report is workspace cleanup, not a tracked
implementation change or a generator fix.

Verify the exclusion boundaries using disposable YAML fixtures with
the same relative paths and the repository configuration. Malformed
vendored/report fixtures must be ignored; malformed maintained YAML
and a sibling governance YAML file must still fail. Use temporary
fixtures rather than introducing a new validation framework or
committing intentionally invalid YAML.

Reject reformatting vendored files: it adds upstream maintenance debt.
Reject disabling indentation checks: maintained YAML still needs them.
Security scanner and MegaLinter configuration stay unchanged.

## Risks / Trade-offs

- [Scanner versions report different findings] -> Record versions and
  compare like-for-like output; do not call a version difference a
  code regression or introduce unrelated fixes.
- [G704 annotations hide a changed trust boundary] -> Explain the
  runner-controlled inputs at each request site during code review.
- [Owner-only files prevent CLI access] -> Run both acceptance profiles
  and check generated file permissions and readability.
- [YAML exclusions hide maintained files] -> Verify the exact report
  path boundary and that first-party YAML still produces lint errors.

## Migration Plan

Apply the test-only fixes and retain the contributor's two YAML
exclusions. Verify the four reported findings, both acceptance
profiles, and YAML exclusion boundaries. No deployment or data
migration is required. Additional validation failures remain separate
work, not reasons to expand the exclusions or disable lint rules.
