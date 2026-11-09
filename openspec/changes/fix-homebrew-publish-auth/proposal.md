# Proposal

## Why

Homebrew publishing failed for v1.1.0 and v1.2.0 because the Git
credential helper could not inherit the App token during push.
PR #902 introduced a protected-branch-compatible PR flow but left
that authentication defect unchanged.

## What Changes

- Expose the existing App token as step-scoped `GH_TOKEN` rather
  than `TAP_TOKEN`; remove per-command token assignments.
- Add offline regression coverage for the actual publishing
  step's environment and Git credential-helper invocation.
- Document authentication requirements and recovery of an
  already-published release without moving its tag.
- Preserve the PR flow, permissions, and release gates.

## Capabilities

### New Capabilities

- `homebrew-publishing`: App-authenticated formula branch and
  pull-request publishing after a successful release.

### Modified Capabilities

None. There are no existing OpenSpec capability specifications.

## Impact

- `.github/workflows/release.yml`: publishing-step environment
  and token usage only; no quality or governance gate changes.
- `tests/homebrew_publish_test.sh` and `Makefile`: a separate
  offline publishing regression target, not a replacement for
  the existing formula installation smoke test.
- `docs/RELEASE_PROCESS.md` and `CHANGELOG.md`: fix and recovery
  documentation.
- No Go API changes, new secrets, or production dependencies.
- Live tap mutations and release reruns require separate
  authorization and are not part of implementation.
