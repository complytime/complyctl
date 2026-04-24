# Packaging Interface Contract

**Feature**: [../spec.md](../spec.md) | **Plan**: [../plan.md](../plan.md)
**Date**: 2026-04-24

## Package Ownership Contract

The `complyctl` RPM **owns** the provider directory tree:

- `/usr/libexec/complytime/` (directory)
- `/usr/libexec/complytime/providers/` (directory)

Provider sub-packages install files **into** this directory without owning it.
This ensures `complyctl` must be installed first, which is enforced by the
`Requires: complyctl >= X.Y.Z` dependency on each provider sub-package.

If the `complyctl` RPM is removed, `dnf` will also remove provider packages
(they depend on it). The provider directories are cleaned up with the
complyctl package removal.

## Binary Naming Convention

Provider binaries MUST follow the pattern `complyctl-provider-<name>`. This is
how the `complyctl` discovery mechanism finds providers at runtime.

The convention is defined in
`internal/complytime/consts.go:ProviderExecutablePrefix = "complyctl-provider-"`.

The discovery searches two locations:
1. `~/.complytime/providers/` (user-local)
2. `/usr/libexec/complytime/providers/` (system-wide, RPM install path)

## Version Compatibility

Provider sub-packages declare `Requires: complyctl >= X.Y.Z` where `X.Y.Z` is
the first complyctl release containing the `pkg/provider/` SDK (renamed from
`pkg/plugin/` in spec 004).

This ensures:
- Users cannot install providers against a complyctl version that predates the
  provider SDK rename (which would be incompatible).
- Newer complyctl versions remain compatible (the minimum version constraint
  does not prevent upgrades).
- The exact version is determined at implementation time when the post-spec-004
  release is tagged.

## Packit Configuration Contract

Both repositories use identical Packit job structure:

| Job                  | Trigger       | Purpose                                    |
|----------------------|---------------|--------------------------------------------|
| `copr_build`         | pull_request  | Build RPMs from PR source in COPR          |
| `tests`              | pull_request  | Run TMT tests via Testing Farm             |
| `propose_downstream` | release       | Create dist-git PRs for Fedora branches    |
| `koji_build`         | commit        | Submit Koji builds after dist-git merge    |
| `bodhi_update`       | commit        | Create Bodhi updates for released versions |

The `tests` job depends on `copr_build` completing first. Built packages are
automatically installed in the testing environment by Testing Farm.

## TMT Test Contract

### complyctl tests (`plans/test-RPM-provide-content.fmf`)

Validates:
- `complyctl version` exits with code 0
- `complyctl --help` exits with code 0

### complytime-providers tests (`plans/test-RPM-providers.fmf`)

Validates:
- `complyctl-provider-openscap` binary exists at expected path
- `complyctl-provider-ampel` binary exists at expected path
- Both binaries have executable permissions

Provider binaries are gRPC servers and cannot be invoked standalone for
functional testing. File presence and permissions are the appropriate smoke
test level. Integration testing is deferred.
