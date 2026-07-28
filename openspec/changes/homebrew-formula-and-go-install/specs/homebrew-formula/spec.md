## ADDED Requirements

### Requirement: Formula installs complyctl from source
The release workflow SHALL generate a Homebrew Formula that builds complyctl
from the tagged source tarball using `go build` with the project's standard
ldflags (version, buildDate).

#### Scenario: Successful brew install
- **WHEN** user runs `brew install complytime/tap/complyctl`
- **THEN** Homebrew downloads the source tarball, compiles with Go, and places
  the `complyctl` binary in the Homebrew bin directory

#### Scenario: Version output matches release tag
- **WHEN** user runs `complyctl version` after installing via Homebrew
- **THEN** the Version field MUST contain the release version (e.g., "1.2.3")

### Requirement: Formula installs shell completions
The Formula SHALL generate and install shell completions for Bash, Zsh, and
Fish using `generate_completions_from_executable(bin/"complyctl", "completion")`.

#### Scenario: Zsh completion available
- **WHEN** user installs complyctl via Homebrew on a system with Zsh
- **THEN** `complyctl <TAB>` provides subcommand completions without additional
  configuration

### Requirement: Formula is published automatically on release
The release workflow SHALL push an updated Formula to
`complytime/homebrew-tap` (branch: main, path: `Formula/complyctl.rb`) after
GoReleaser completes successfully.

#### Scenario: First release with Homebrew
- **WHEN** maintainer triggers the release workflow for a new tag
- **THEN** a commit is pushed to `complytime/homebrew-tap` containing
  `Formula/complyctl.rb` with the correct url and sha256 for the tagged source
  tarball

#### Scenario: Subsequent release updates existing Formula
- **WHEN** maintainer triggers the release workflow for a newer tag
- **THEN** the existing `Formula/complyctl.rb` in the tap is overwritten with
  updated url and sha256 values

### Requirement: Formula authentication uses GitHub App
The workflow SHALL mint a short-lived token via `actions/create-github-app-token`
using org secrets `APP_ID` and `PRIVATE_KEY` to push to the tap repository.
The token MUST be scoped to the `homebrew-tap` repository only.

#### Scenario: Token generation
- **WHEN** the release job reaches the "Generate Homebrew tap token" step
- **THEN** a token is created with Contents:write access to `complytime/homebrew-tap`

#### Scenario: Token not available
- **WHEN** the GitHub App does not have access to `homebrew-tap`
- **THEN** the push step fails with a clear authentication error (non-silent
  failure)

### Requirement: No macOS signing or Gatekeeper workaround needed
Because the Formula builds from source on the user's machine, the installed
binary SHALL NOT be subject to Gatekeeper quarantine. No `xattr` removal, code
signing, or notarization is required.

#### Scenario: Unsigned binary runs without warning
- **WHEN** user installs via `brew install complytime/tap/complyctl` and runs
  `complyctl version`
- **THEN** macOS does not display any Gatekeeper warning or quarantine dialog
