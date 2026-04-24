# Implementation Plan: RPM Packaging and CI for Split Repositories

**Branch**: `005-rpm-packaging-ci` | **Date**: 2026-04-24 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/005-rpm-packaging-ci/spec.md`

## Summary

Restore Fedora RPM packaging for complyctl and create new packaging for
complytime-providers after the repository split (spec 004). The complyctl spec
is simplified to deliver only the CLI binary and man page. A new spec is created
in complytime-providers producing two sub-packages (openscap and ampel
providers) with no main binary RPM. Both repositories get Packit CI/CD
automation and TMT smoke tests. Release process documentation and GoReleaser
configuration are updated to reflect the new packaging structure.

## Technical Context

**Language/Version**: Go 1.25
**Primary Dependencies**: go-rpm-macros, Packit, Testing Farm (TMT/FMF)
**Storage**: N/A
**Testing**: `go test` (unit, in `%check`), TMT inline scripts (RPM smoke)
**Target Platform**: Fedora rawhide/43/42, CentOS Stream 9/10 (x86_64)
**Project Type**: CLI + provider binaries (RPM packaging)
**Performance Goals**: N/A
**Constraints**: Fedora Packaging Guidelines for Go projects
**Scale/Scope**: 2 repositories, 3 binary RPMs, release automation for both

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle                          | Status | Notes                                                                     |
|------------------------------------|--------|---------------------------------------------------------------------------|
| I. Single Source of Truth          | PASS   | Provider path centralized via `%global app_dir`; reused across both specs |
| II. Simplicity & Isolation         | PASS   | Each spec has one concern; TMT plans are minimal smoke tests              |
| III. Incremental Improvement       | PASS   | Packaging-only; integration tests deferred to separate spec               |
| IV. Readability First              | PASS   | Specs follow established Fedora patterns with clear comments              |
| V. Do Not Reinvent the Wheel       | PASS   | Standard Go RPM macros, Packit, TMT -- all established Fedora tools       |
| VI. Composability                  | PASS   | Independent RPM packages compose via standard `Requires` dependency       |
| VII. Convention Over Configuration | PASS   | Follows Fedora Go packaging conventions; Packit with standard job configs |

No violations detected. No complexity tracking needed.

## Project Structure

### Documentation (this feature)

```text
specs/005-rpm-packaging-ci/
├── plan.md                         # This file
├── research.md                     # Phase 0 output
├── data-model.md                   # Phase 1 output
├── quickstart.md                   # Phase 1 output
├── contracts/
│   └── packaging-interface.md      # Phase 1 output
└── tasks.md                        # Phase 2 output (/speckit.tasks)
```

### Source Code (both repositories)

```text
# complyctl repository (this repo) -- files to UPDATE
complyctl.spec                          # UPDATE: remove provider sub-package, add man page
.goreleaser.yaml                        # UPDATE: remove openscap-plugin build entry
.packit.yaml                            # KEEP: already correct
.fmf/version                           # EXISTS: FMF metadata root
plans/
  test-RPM-provide-content.fmf         # KEEP: existing smoke test
docs/
  man/complyctl.md                      # UPDATE: plugin→provider terminology
  man/complyctl.1                       # REGENERATE: via make man
  RELEASE_PROCESS.md                    # UPDATE: reflect split packaging structure
  TESTING_FARM.md                       # KEEP: generic, no provider-specific refs
  INSTALLATION.md                       # KEEP: no provider-specific refs

# complytime-providers repository (github.com/complytime/complytime-providers)
complytime-providers.spec               # NEW: source RPM → two sub-packages only
.packit.yaml                            # NEW: full Packit CI/CD config
.fmf/
  version                              # NEW: FMF metadata root (contains "1")
plans/
  test-RPM-providers.fmf               # NEW: provider binary presence + permissions
```

**Structure Decision**: This feature modifies packaging and CI configuration
files in two repositories. No application source code changes are required.
The complyctl repo changes are in-tree; the complytime-providers changes require
work in the external repository at `github.com/complytime/complytime-providers`.

## Key Research Decisions

Full details in [research.md](research.md).

1. **Bundled provides**: Automatic generation via `vendor/modules.txt` installed
   as `%license`; no manual `Provides: bundled(golang(...))` list needed.

2. **Build scope**: complyctl spec changes from `go build ./cmd/...` to
   `go build ./cmd/complyctl/` (avoids building dev-only binaries in RPM).

3. **No main package**: complytime-providers source RPM omits `%files` for the
   main package name; only sub-packages are produced.

4. **TMT format**: Inline `execute.script` in `.fmf` files (matches existing
   complyctl pattern).

5. **Man page**: Install pre-built `docs/man/complyctl.1` from source tarball;
   no pandoc BuildRequires. Source `complyctl.md` updated for terminology, then
   regenerated via `make man` before release.

6. **GoReleaser**: Remove `openscap-plugin` build entry from `.goreleaser.yaml`
   (tracked by spec 004 FR-010, prerequisite for release automation).

7. **Release docs**: Update `RELEASE_PROCESS.md` to reflect that providers are
   a separate Fedora package with independent release and Packit automation.
