# Research: RPM Packaging and CI for Split Repositories

**Feature**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)
**Date**: 2026-04-24

## Decision 1: RPM Spec Macros for Go Projects

**Decision**: Use `%gometa -f`, `%goprep -k`, manual `go build` with
`%set_build_flags` and `-buildmode=pie`, and `go test -mod=vendor` in `%check`.

**Rationale**: The existing complyctl.spec already uses this pattern. The `-f`
flag on `%gometa` uses `golang_arches_future` (excludes i686). The `-k` flag on
`%goprep` preserves the `vendor/` directory. Manual `go build` (instead of
`%gobuild`) is needed because complyctl injects custom version info via
`-ldflags -X`. For complytime-providers, manual `go build` is also appropriate
since two separate binaries need specific output names.

**Alternatives considered**:
- `%gobuild` macro: Wraps `go build` with Fedora hardening flags but does not
  easily support custom LD flags or multiple output binaries with specific
  names. Would require workarounds.

## Decision 2: Bundled Dependency Declaration

**Decision**: Install `vendor/modules.txt` as a `%license` file. The
`go_mod_vendor.prov` RPM generator (part of `go-rpm-macros`) automatically
parses it and emits `Provides: bundled(golang(...)) = <version>` entries at
build time.

**Rationale**: Eliminates manual maintenance of a bundled provides list. The
generator auto-produces accurate provides from `vendor/modules.txt`. This is the
recommended modern Fedora approach for Go projects with vendored dependencies.

**Alternatives considered**:
- Manual `Provides: bundled(golang(...))` lines in the spec: Requires updating
  on every dependency change. Error-prone and labor-intensive.
- `gobundled.prov`: Older mechanism for `-devel` subpackages. Not applicable
  to binary-only packages.

## Decision 3: Sub-packages Without Main Package

**Decision**: Omit the `%files` section for the main `complytime-providers`
package name. RPM produces only the two sub-packages.

**Rationale**: The source RPM produces two independent provider binaries with
no shared runtime files. A main package would be either empty or a meta-package.
Per clarification session (Option A), no main binary RPM is published.

**Alternatives considered**:
- Empty meta-package requiring both providers: Rejected (user wants individual
  provider installation choice).
- Shared-files package with LICENSE/docs: Rejected (each sub-package includes
  its own LICENSE).

## Decision 4: TMT Test Plan Structure

**Decision**: Use inline `execute.script` in `.fmf` files for smoke tests.

**Rationale**: Matches the existing `plans/test-RPM-provide-content.fmf` pattern
in complyctl. Simple enough that separate test script files add no value.
Testing Farm executes the script commands after COPR-built RPMs are installed.

**Alternatives considered**:
- Separate `tests/` directory with `main.fmf` + shell scripts: Appropriate for
  complex test suites but overkill for binary presence and version checks.

## Decision 5: complyctl Build Scope in RPM

**Decision**: Change build command from `go build ./cmd/...` to
`go build ./cmd/complyctl/` in the RPM spec.

**Rationale**: After the split, `cmd/` contains 4 entries: `complyctl`,
`behavioral-report`, `mock-oci-registry`, `test-provider`. Only `complyctl`
should be in the RPM. Building all 4 wastes build time and may cause build
failures if dev-only `cmd/` entries have dependencies not available in the
RPM build root.

**Alternatives considered**:
- Keep `./cmd/...` and install only `complyctl`: Functional but wasteful and
  risks build failures from dev-only dependencies.

## Decision 6: Man Page Packaging

**Decision**: Install the pre-built `docs/man/complyctl.1` from the source
tarball. No pandoc BuildRequires needed.

**Rationale**: The man page source (`complyctl.md`) is maintained by developers
and converted to `complyctl.1` via `make man` (uses pandoc) as part of the
development workflow. The pre-built `.1` file is committed to the repo and
included in the release tarball. The RPM spec simply installs it. This avoids
adding pandoc as a BuildRequires (which may not be available on all build
targets like CentOS Stream).

**Alternatives considered**:
- Build from source during RPM build: Requires `BuildRequires: pandoc`. Adds
  build complexity and a dependency that may not be universally available.

## Decision 7: Release Process and GoReleaser Updates

**Decision**: Update `.goreleaser.yaml` to remove the `openscap-plugin` build
entry. Update `docs/RELEASE_PROCESS.md` to reflect the new multi-repository
packaging structure.

**Rationale**: The `.goreleaser.yaml` currently defines two builds: `complyctl`
(from `./cmd/complyctl/`) and `openscap-plugin` (from `./cmd/openscap-plugin`).
After the repository split, `cmd/openscap-plugin` no longer exists, so the
GoReleaser release will fail. The `RELEASE_PROCESS.md` references the Fedora
package automation but does not mention that providers are now a separate
package. These updates are prerequisites for the Packit release automation to
function correctly.

The `.goreleaser.yaml` update is tracked by spec 004 FR-010 ("CI workflows MUST
be updated to remove all steps that reference the removed provider source
directories"). It is included in this plan because the release process directly
gates the Packit `propose_downstream` automation that this feature enables.

**Alternatives considered**:
- Defer to spec 004: The GoReleaser fix is technically in spec 004 scope, but
  release automation for this feature cannot be validated without it. Including
  it here avoids a circular dependency.

## Decision 8: Provider Binary Test Approach

**Decision**: TMT tests for complytime-providers validate file presence at the
expected path and executable permissions. No runtime invocation of provider
binaries.

**Rationale**: Provider binaries are gRPC servers invoked by complyctl via
the hashicorp/go-plugin protocol. They do not support standalone `--help` or
`--version` flags. Verifying they are installed at the correct location with
correct permissions is the appropriate level for smoke testing. Integration
testing (complyctl invoking providers) is explicitly deferred to a separate
specification.

**Alternatives considered**:
- Invoke provider binary standalone: Provider binaries are gRPC servers and will
  exit with an error when invoked without a parent complyctl process. Not useful
  for smoke testing.
