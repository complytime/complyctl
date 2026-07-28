## ADDED Requirements

### Requirement: macOS binaries included in release archives
GoReleaser SHALL produce release archives for darwin/amd64 and darwin/arm64
in addition to existing linux builds. Archives MUST use tar.gz format.

#### Scenario: Release contains macOS archives
- **WHEN** maintainer triggers the release workflow
- **THEN** the GitHub Release page includes downloadable archives named
  `complyctl_darwin_x86_64.tar.gz` and `complyctl_darwin_arm64.tar.gz`

#### Scenario: macOS binary is functional
- **WHEN** user downloads and extracts the darwin/arm64 archive on an Apple
  Silicon Mac
- **THEN** the `complyctl` binary executes and `complyctl version` outputs
  version information

### Requirement: CGO disabled for cross-compilation
All builds (including darwin) SHALL set `CGO_ENABLED=0` to ensure static
linking and reliable cross-compilation without a macOS SDK.

#### Scenario: Static binary
- **WHEN** GoReleaser builds the darwin/arm64 target on an ubuntu runner
- **THEN** the resulting binary has no dynamic library dependencies and runs
  on macOS without additional shared libraries

### Requirement: Archive naming follows uname convention
Archive names SHALL use the existing name template that maps architecture
identifiers to `uname`-compatible names (amd64 → x86_64).

#### Scenario: Consistent naming
- **WHEN** a release is created
- **THEN** the darwin/amd64 archive is named `complyctl_darwin_x86_64.tar.gz`
  (not `complyctl_darwin_amd64.tar.gz`)
