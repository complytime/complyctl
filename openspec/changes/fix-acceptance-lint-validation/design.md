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

## Goals / Non-Goals

**Goals:**

- Resolve exactly the two G704 and two G306 findings.
- Preserve registry preflight behavior in both acceptance profiles.
- Keep generated configuration readable by the test process and CLI.

**Non-Goals:**

- New validation profiles, Make targets, or tool-version enforcement.
- YAML, pre-commit, vendor, or generated-report cleanup.
- CI, scanner configuration, agent conventions, or workflow gates.
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

## Risks / Trade-offs

- [Scanner versions report different findings] -> Record versions and
  compare like-for-like output; do not call a version difference a
  code regression or introduce unrelated fixes.
- [G704 annotations hide a changed trust boundary] -> Explain the
  runner-controlled inputs at each request site during code review.
- [Owner-only files prevent CLI access] -> Run both acceptance profiles
  and check generated file permissions and readability.

## Migration Plan

Apply the test-only fixes and verify the four reported findings and
both acceptance profiles. No deployment or data migration is required.
Any additional validation failures remain separate work, not reasons
to expand this change or weaken a gate.
