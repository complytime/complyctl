# Proposal

## Why

Maintainers cannot preview the generated release notes before the
release workflow creates a tag. They need one local command to see
the actual notes and assess the release delta without building or
publishing artifacts.

## What Changes

- Add `make release-notes-preview` to render Markdown with the
  open-source GoReleaser changelog generator.
- Always discover the latest published stable release in
  `complytime/complyctl` and compare it with the checkout's HEAD.
  Do not expose a baseline override or require a proposed tag.
- Reuse the checkout's `.goreleaser.yaml` changelog settings through
  an isolated temporary configuration that disables builds and
  publishing side effects.
- Resolve changelog requests against canonical upstream even when
  `origin` points to a contributor fork.
- Add focused regression tests and local usage documentation.

## Capabilities

### New Capabilities

- `release-notes-preview`: Preview the upstream release delta using
  the existing release-note renderer without creating a release.

### Modified Capabilities

None.

## Impact

- Add a maintainer helper under `scripts/`, focused tests under
  `tests/`, and preview and test targets in `Makefile`.
- Update `docs/RELEASE_PROCESS.md` and assess the documentation gate
  for `CHANGELOG.md`, `AGENTS.md`, and `README.md`.
- Use installed Git, GitHub CLI, open-source GoReleaser, Python 3,
  and PyYAML. Do not add application or Go module dependencies.
- Do not change the release workflow, release gates, GoReleaser
  publishing configuration, or complyctl's public APIs.
