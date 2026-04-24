# Feature Specification: RPM Packaging and CI for Split Repositories

**Feature Branch**: `005-rpm-packaging-ci`
**Created**: 2026-04-24
**Status**: Draft
**Input**: User description: "After splitting the repository by moving the openscap and ampel providers to complytime-providers repository, the RPM is naturally broken. The spec file in complyctl will only deliver the complyctl binary. Another single spec file must be created in complytime-providers to generate two sub-packages (OpenSCAP and Ampel providers). Provider RPMs must require complyctl. Testing-farm CI tests for each PR and Packit automation for Fedora Packages on release."

## Overview

The repository split (spec 004) moved the OpenSCAP and Ampel providers from complyctl to the `complytime-providers` repository. The current RPM spec (`complyctl.spec`) still references the removed provider directories (`cmd/openscap-plugin/`), breaking the build and all testing-farm CI checks.

This feature restores a working Fedora packaging pipeline across both repositories by:

1. Simplifying the complyctl RPM spec to deliver only the `complyctl` binary.
2. Creating a new RPM spec in `complytime-providers` that produces two sub-packages: one for the OpenSCAP provider and one for the Ampel provider.
3. Establishing a dependency relationship where provider packages require complyctl, while complyctl can be installed standalone.
4. Adding testing-farm CI validation for each PR in both repositories.
5. Configuring Packit automation so that new releases automatically propagate to Fedora packages.

All RPMs follow the Fedora Packaging Guidelines for Go projects, including bundled dependency declarations.

Integration tests between complyctl and complytime-providers RPMs are explicitly out of scope and deferred to a separate specification.

---

## Clarifications

### Session 2026-04-24

- Q: Should the complytime-providers source RPM produce a main `complytime-providers` binary package alongside the sub-packages? → A: No main package. The source RPM produces only the two sub-packages (`complytime-providers-openscap` and `complytime-providers-ampel`); no `complytime-providers` binary RPM is published (Option A).
- Q: Should provider sub-packages use unversioned, minimum-version, or exact-match dependency on complyctl? → A: Minimum version (`Requires: complyctl >= X.Y.Z`) to ensure provider SDK compatibility; the exact version is determined at implementation time when the post-spec-004 release is tagged (Option B).

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - complyctl RPM Builds and Installs Cleanly (Priority: P1)

A Fedora package maintainer submits a PR to the complyctl repository and the automated CI pipeline successfully builds an RPM containing only the `complyctl` binary, installs it on a test system, and validates the binary is functional.

**Why this priority**: The complyctl RPM is currently broken after the provider split. Restoring a working RPM build is the prerequisite for all other packaging work and for unblocking CI on every PR.

**Independent Test**: Open a PR against complyctl, observe the Packit/COPR build succeeds, and confirm the testing-farm job installs the RPM and runs basic validation commands (`complyctl version`, `complyctl --help`) without errors.

**Acceptance Scenarios**:

1. **Given** the updated complyctl RPM spec, **When** a COPR build is triggered by a PR, **Then** the build succeeds and produces a single `complyctl` binary RPM with no provider sub-packages.
2. **Given** the complyctl RPM installed on a Fedora system, **When** a user runs `complyctl version`, **Then** the command exits successfully and displays the version.
3. **Given** the complyctl RPM installed on a Fedora system, **When** a user runs `complyctl --help`, **Then** the command exits successfully and displays usage information.
4. **Given** the complyctl RPM spec, **When** reviewed against Fedora Packaging Guidelines for Go projects, **Then** the spec includes proper bundled dependency declarations, license metadata, and follows the standard Go RPM macros.

---

### User Story 2 - Provider Sub-Packages Build From complytime-providers (Priority: P1)

A package maintainer submits a PR to the `complytime-providers` repository and the CI pipeline builds a source RPM that produces two binary sub-packages: `complytime-providers-openscap` and `complytime-providers-ampel`. Each sub-package contains its respective provider binary installed to the standard provider discovery path.

**Why this priority**: Equally critical to Story 1. Without working provider RPMs, Fedora users cannot install providers through the package manager. The provider packages depend on the complyctl package being available.

**Independent Test**: Open a PR against complytime-providers, observe the Packit/COPR build succeeds, and confirm the testing-farm job installs each provider sub-package along with its complyctl dependency and validates the provider binaries are present at the expected filesystem locations.

**Acceptance Scenarios**:

1. **Given** the complytime-providers RPM spec, **When** a COPR build is triggered, **Then** two binary sub-packages are produced: `complytime-providers-openscap` and `complytime-providers-ampel`.
2. **Given** the `complytime-providers-openscap` RPM installed on a Fedora system, **When** a user checks the provider binary path, **Then** `complyctl-provider-openscap` is present at the standard provider directory.
3. **Given** the `complytime-providers-ampel` RPM installed on a Fedora system, **When** a user checks the provider binary path, **Then** `complyctl-provider-ampel` is present at the standard provider directory.
4. **Given** either provider sub-package, **When** installed via `dnf`, **Then** the `complyctl` package is automatically pulled in as a dependency.
5. **Given** the complytime-providers RPM spec, **When** reviewed against Fedora Packaging Guidelines for Go projects, **Then** the spec includes proper bundled dependency declarations, license metadata, and follows the standard Go RPM macros.

---

### User Story 3 - complyctl Installs Without Providers (Priority: P2)

A Fedora user installs the `complyctl` package without any provider packages. The installation succeeds and the CLI is functional for operations that do not require a provider (e.g., version display, help, workspace initialization).

**Why this priority**: This validates the decoupled dependency model. complyctl must be independently installable since some users may only need the core tool or may install providers separately.

**Independent Test**: Install `complyctl` RPM on a clean Fedora system without any provider packages and verify the binary runs, displays help, and exits cleanly.

**Acceptance Scenarios**:

1. **Given** a clean Fedora system with no provider packages installed, **When** a user installs only the `complyctl` RPM, **Then** the installation succeeds without dependency errors.
2. **Given** a Fedora system with only `complyctl` installed, **When** a user runs `complyctl version`, **Then** the command succeeds and shows version information.
3. **Given** a Fedora system with only `complyctl` installed, **When** a user attempts a scan operation that requires a provider, **Then** the CLI produces a clear message indicating no providers are available.

---

### User Story 4 - Automated Fedora Package Updates on Release (Priority: P2)

When a new version is tagged and released in either repository, Packit automatically proposes a downstream PR to the corresponding Fedora dist-git repository, submits Koji builds, and triggers Bodhi updates for released Fedora versions.

**Why this priority**: Automation is essential for sustainable maintenance but is not blocking the initial RPM fix. Once the specs and CI are working (Stories 1-3), the release automation completes the pipeline.

**Independent Test**: Create a release tag in the repository, observe that Packit creates a PR in the corresponding Fedora dist-git, and after merge, a Koji build and Bodhi update are triggered.

**Acceptance Scenarios**:

1. **Given** a new version tag pushed to the complyctl repository, **When** the Packit `propose_downstream` job triggers, **Then** a PR is created in the Fedora dist-git complyctl repository for each configured branch.
2. **Given** a new version tag pushed to the complytime-providers repository, **When** the Packit `propose_downstream` job triggers, **Then** a PR is created in the Fedora dist-git complytime-providers repository for each configured branch.
3. **Given** a merged PR in Fedora dist-git for either package, **When** the Packit `koji_build` job triggers, **Then** a Koji build is submitted for the corresponding branches.
4. **Given** a successful Koji build for a released Fedora version, **When** the Packit `bodhi_update` job triggers, **Then** a Bodhi update is created for the corresponding package.

---

### User Story 5 - Testing-Farm Validates RPMs on Each PR (Priority: P2)

Each PR submitted to either repository triggers testing-farm CI that builds the RPM from the PR source, installs it on a test system, and runs basic validation tests to catch packaging regressions before merge.

**Why this priority**: CI validation ensures packaging quality over time. It prevents regressions where a code change inadvertently breaks the RPM build or the installed binary.

**Independent Test**: Submit a PR with a trivial change and verify that testing-farm jobs appear in the PR checks, execute successfully, and their results are visible in the PR.

**Acceptance Scenarios**:

1. **Given** a PR submitted to the complyctl repository, **When** the Packit tests job triggers, **Then** a testing-farm job runs that installs the built RPM and validates the `complyctl` binary.
2. **Given** a PR submitted to the complytime-providers repository, **When** the Packit tests job triggers, **Then** a testing-farm job runs that installs both provider sub-packages and validates the provider binaries are present.
3. **Given** a testing-farm test run for either repository, **When** the test completes, **Then** the pass/fail result is reported back to the PR as a status check.

---

### Edge Cases

- What happens when a provider RPM is installed but the complyctl package is not yet available in the target Fedora version? The provider package declares a `Requires: complyctl` dependency, so `dnf` will refuse the installation if complyctl is not available, giving the user a clear dependency error.
- What happens when the complyctl and complytime-providers packages are at mismatched versions? Since integration tests are deferred, version compatibility is managed by the `Requires` dependency specifying a minimum compatible complyctl version.
- What happens when a COPR build succeeds but the testing-farm test fails? The PR status check reflects the failure, blocking merge until the packaging issue is resolved.
- What happens when the Fedora dist-git package for complytime-providers does not yet exist? The package must be registered in Fedora before Packit can propose downstream PRs. This is a one-time prerequisite handled by the package maintainer through the Fedora package review process.

---

## Requirements *(mandatory)*

### Functional Requirements

#### complyctl RPM Spec Update

- **FR-001**: The complyctl RPM spec MUST be updated to remove the `openscap-provider` sub-package section and all references to building or installing provider binaries.
- **FR-002**: The complyctl RPM spec MUST build only the `complyctl` binary from `cmd/complyctl/`.
- **FR-003**: The complyctl RPM spec MUST install the `complyctl` binary to `%{_bindir}` and the LICENSE file.
- **FR-004**: The complyctl RPM spec MUST retain the provider directory structure (`%{_libexecdir}/complytime/providers/`) as an owned directory so that provider packages can install into it.
- **FR-005**: The complyctl RPM spec MUST follow the Fedora Packaging Guidelines for Go projects, including proper use of Go RPM macros and bundled dependency declarations.
- **FR-006**: The complyctl RPM spec MUST declare all bundled Go dependencies using `Provides: bundled(golang(...))` entries, as required by Fedora for Go projects that vendor dependencies.
- **FR-007**: The complyctl RPM spec `%check` section MUST run the project's unit tests during the build.

#### complytime-providers RPM Spec (New)

- **FR-008**: A new RPM spec file MUST be created in the `complytime-providers` repository that builds both provider binaries from a single source package.
- **FR-009**: The spec MUST define two sub-packages: `complytime-providers-openscap` and `complytime-providers-ampel`. No main `complytime-providers` binary RPM is produced; the source RPM yields only these two sub-packages.
- **FR-010**: Each sub-package MUST install its respective provider binary (`complyctl-provider-openscap` or `complyctl-provider-ampel`) to the standard provider directory (`%{_libexecdir}/complytime/providers/`).
- **FR-011**: Each sub-package MUST declare a dependency on the `complyctl` package using `Requires: complyctl >= X.Y.Z`, where `X.Y.Z` is the first complyctl release containing the provider SDK (`pkg/provider/`). The exact version is determined at implementation time.
- **FR-012**: The `complytime-providers-openscap` sub-package MUST declare a dependency on `scap-security-guide` since the OpenSCAP provider requires SCAP content to function.
- **FR-013**: The complytime-providers RPM spec MUST follow the Fedora Packaging Guidelines for Go projects, including proper use of Go RPM macros and bundled dependency declarations.
- **FR-014**: The complytime-providers RPM spec MUST declare all bundled Go dependencies using `Provides: bundled(golang(...))` entries.
- **FR-015**: The complytime-providers RPM spec `%check` section MUST run the project's unit tests during the build.

#### Packit Configuration

- **FR-016**: The complyctl `.packit.yaml` MUST be updated to reflect the simplified spec (no provider-related file syncing changes expected, but the file list MUST match actual deliverables).
- **FR-017**: A `.packit.yaml` MUST be created in the `complytime-providers` repository with COPR build, testing-farm tests, propose-downstream, Koji build, and Bodhi update jobs.
- **FR-018**: Both Packit configurations MUST target the same set of Fedora and CentOS Stream versions for COPR builds and testing-farm tests (currently: Fedora rawhide, Fedora 43, Fedora 42, CentOS Stream 9, CentOS Stream 10).
- **FR-019**: Both Packit configurations MUST include `propose_downstream`, `koji_build`, and `bodhi_update` jobs for release automation against the appropriate dist-git branches.

#### Testing-Farm / TMT Plans

- **FR-020**: The complyctl repository MUST have a TMT test plan that validates the installed `complyctl` RPM binary is functional (at minimum: `complyctl version` and `complyctl --help` succeed).
- **FR-021**: The `complytime-providers` repository MUST have TMT test plan(s) that validate each installed provider sub-package binary is present at the expected location and executable.
- **FR-022**: Both repositories MUST have the FMF metadata root (`.fmf/version` file) so that testing-farm can discover and execute TMT plans.

#### Release Process and Documentation

- **FR-023**: The complyctl `.goreleaser.yaml` MUST be updated to remove the `openscap-plugin` build entry, since `cmd/openscap-plugin/` no longer exists after the repository split. Only the `complyctl` binary MUST be built by GoReleaser.
- **FR-024**: The `docs/RELEASE_PROCESS.md` MUST be updated to reflect that providers are now a separate Fedora package (`complytime-providers`) with an independent release cycle and Packit automation.
- **FR-025**: The complyctl RPM spec MUST install the `complyctl.1` man page to the standard man page directory. The man page source (`docs/man/complyctl.md`) MUST be updated with provider terminology before regeneration via `make man`.
- **FR-026**: The `docs/man/complyctl.md` man page source MUST use "provider" terminology consistently (replacing any remaining "plugin" references) and be regenerated into `docs/man/complyctl.1` before the RPM source tarball is produced.

#### Scope Boundaries

- **FR-027**: Integration tests that validate complyctl and providers work together as installed RPMs are explicitly out of scope and MUST NOT be included in this feature. They are deferred to a separate specification.

### Key Entities

- **complyctl RPM**: The Fedora package that delivers the `complyctl` CLI binary. Installable standalone. Owns the provider directory structure.
- **complytime-providers source RPM**: A single source package in Fedora that builds from the `complytime-providers` upstream repository and produces only two binary sub-packages (no main binary RPM is published).
- **complytime-providers-openscap**: A binary sub-package containing the OpenSCAP provider binary. Requires complyctl and scap-security-guide.
- **complytime-providers-ampel**: A binary sub-package containing the Ampel provider binary. Requires complyctl.
- **TMT plan**: A test metadata tree (tmt) definition that testing-farm executes to validate installed RPMs. Defined as `.fmf` files in the `plans/` directory.
- **Packit configuration**: A `.packit.yaml` file that drives the CI/CD pipeline: COPR builds on PRs, testing-farm tests on PRs, and downstream Fedora automation on releases.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The complyctl RPM builds successfully from the updated spec, producing a single binary package with no provider sub-packages.
- **SC-002**: The complytime-providers RPM builds successfully, producing two binary sub-packages (`complytime-providers-openscap` and `complytime-providers-ampel`).
- **SC-003**: Installing a provider sub-package via `dnf` automatically resolves and installs the `complyctl` dependency.
- **SC-004**: Installing the `complyctl` RPM on a clean system without providers succeeds, and `complyctl version` exits with code 0.
- **SC-005**: PRs to the complyctl repository trigger COPR build and testing-farm jobs that pass, validating the RPM is functional.
- **SC-006**: PRs to the complytime-providers repository trigger COPR build and testing-farm jobs that pass, validating both provider sub-packages are functional.
- **SC-007**: A release tag on the complyctl repository triggers Packit to propose a downstream PR to the Fedora dist-git complyctl package.
- **SC-008**: A release tag on the complytime-providers repository triggers Packit to propose a downstream PR to the Fedora dist-git complytime-providers package.
- **SC-009**: Both RPM specs pass Fedora package review criteria for Go projects, including bundled dependency declarations and proper license fields.
- **SC-010**: The complyctl GoReleaser build (`goreleaser check` and `goreleaser build --snapshot`) succeeds without referencing the removed openscap-plugin directory.
- **SC-011**: The `complyctl.1` man page is present in the installed complyctl RPM.

---

## Assumptions

- The `complytime-providers` Fedora package does not yet exist in Fedora dist-git. A Fedora package review request must be filed and approved before the `propose_downstream` and `koji_build` Packit jobs can function. This is a manual, one-time prerequisite outside the scope of this specification.
- The complyctl Fedora package already exists in Fedora dist-git and the existing Packit integration continues to work after the spec update.
- Both repositories use Go module vendoring (dependencies committed under `vendor/`), which is the standard approach for Fedora Go RPM packaging with bundled dependencies.
- The `complyctl` provider directory (`/usr/libexec/complytime/providers/`) is owned by the complyctl RPM. Provider sub-packages install files into this directory without owning it.
- The Packit service has access to both the `complytime/complyctl` and `complytime/complytime-providers` GitHub repositories.
- Testing-farm tests perform basic smoke tests only (binary presence, version output, help output). They do not validate provider-complyctl interaction. Integration tests are explicitly deferred.
- The minimum `complyctl` version required by providers corresponds to the first release that includes the provider SDK rename from spec 004 (`pkg/provider/`). The exact version number is determined at implementation time when the release is tagged.

---

## Dependencies

- **Spec 004 (Providers Repository Split)**: Must be completed before this feature can be fully implemented. The complyctl codebase must have providers removed and the `complytime-providers` repository must contain the provider source code.
- **complyctl tagged release**: A tagged release of complyctl with the provider SDK (`pkg/provider/`) must be published before the complytime-providers spec can declare a versioned `Requires: complyctl >= X.Y.Z`.
- **Fedora package review for complytime-providers**: A new package review request must be filed and approved in Fedora before downstream automation can function. This is a manual prerequisite.
- **Packit access to complytime-providers**: The Packit service must be enabled on the `complytime/complytime-providers` GitHub repository.
