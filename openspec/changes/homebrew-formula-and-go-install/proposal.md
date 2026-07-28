## Why

macOS users currently have no streamlined way to install complyctl. The only
options are downloading a Linux-only binary from GitHub Releases or building
from source with a full Go toolchain and Make. This creates friction for
developer onboarding—especially for non-Go developers—and is inconsistent with
the project's adoption of Homebrew as the standard macOS package manager for
local tooling (uv, bun, node, opencode, etc.). Issue #713.

## What Changes

- Release pipeline produces macOS (darwin) binaries for amd64 and arm64 in
  addition to existing Linux builds.
- A source-build Homebrew Formula is automatically generated and pushed to the
  `complytime/homebrew-tap` repository on each release, enabling
  `brew install complytime/tap/complyctl`.
- `go install github.com/complytime/complyctl/cmd/complyctl@latest` is
  documented as a zero-infrastructure alternative for Go developers.
- Shell completions (Bash, Zsh, Fish) are installed automatically via the
  Formula.

## Capabilities

### New Capabilities
- `homebrew-formula`: Automated source-build Homebrew Formula published to the
  complytime/homebrew-tap repository on each release. Builds from source using
  Go (avoids Gatekeeper/signing entirely). Installs shell completions.
- `go-install`: Documented `go install` path as a cross-platform alternative
  requiring no tap or external infrastructure.
- `macos-binaries`: Cross-compilation of darwin/amd64 and darwin/arm64 release
  archives alongside existing Linux builds.

### Modified Capabilities
<!-- No existing spec-level behavior changes. The release pipeline gains new
     steps but existing release semantics are preserved. -->

## Impact

- `.goreleaser.yaml`: Build matrix gains darwin + explicit goarch; deprecated
  `format` key updated to `formats`.
- `.github/workflows/release.yml`: New steps for GitHub App token generation
  and Formula publishing to the tap repo.
- `docs/INSTALLATION.md`: Gains Homebrew and `go install` sections.
- `CHANGELOG.md`: New entries under Unreleased.
- `complytime/homebrew-tap` repository: Needs a `Formula/` directory; the
  GitHub App must have Contents:write access to this repo.
- No runtime behavior changes to complyctl itself.
