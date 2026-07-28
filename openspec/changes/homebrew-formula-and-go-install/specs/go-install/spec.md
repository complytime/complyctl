## ADDED Requirements

### Requirement: go install produces working binary
The module path `github.com/complytime/complyctl/cmd/complyctl` SHALL be
installable via `go install ...@latest` and produce a functional complyctl
binary.

#### Scenario: Install latest
- **WHEN** user runs `go install github.com/complytime/complyctl/cmd/complyctl@latest`
- **THEN** a `complyctl` binary is placed in `$GOPATH/bin` (or `$GOBIN`) and
  runs successfully

#### Scenario: Install specific version
- **WHEN** user runs `go install github.com/complytime/complyctl/cmd/complyctl@v1.2.3`
- **THEN** the installed binary corresponds to the tagged version

### Requirement: Version output indicates devel for go install
When installed via `go install` without custom ldflags, the version output
SHALL indicate a development build rather than showing empty or misleading
version information.

#### Scenario: Default go install version string
- **WHEN** user installs via `go install ...@latest` without passing ldflags
- **THEN** `complyctl version` shows `(devel)` or the module version from
  Go's build info (not an empty string)

### Requirement: go install is documented
The `docs/INSTALLATION.md` file SHALL include a section documenting the
`go install` command and noting that version information will show `(devel)`.

#### Scenario: Documentation present
- **WHEN** user opens `docs/INSTALLATION.md`
- **THEN** there is a "go install" section with the exact command and a note
  about version behavior
