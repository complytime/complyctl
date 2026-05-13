# Changelog

## Unreleased

### Added

- Cross-repo integration test infrastructure validating the complyctl + Ampel
  provider pipeline end-to-end (`tests/cross-repo/`, `make test-cross-repo`).
- CI workflow `ci_cross_repo_integration.yml` that builds complyctl from the PR
  branch and complytime-providers from main, then runs the full get → generate →
  scan pipeline with real snappy and ampel binaries.
- Minimal test Gemara policy (`policies/test-branch-protection`) seeded in the
  mock OCI registry for integration testing.
