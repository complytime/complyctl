# Feature Specification: Providers Repository Split

**Feature Branch**: `004-providers-repository-split`
**Created**: 2026-04-15
**Status**: Draft
**Input**: User description: "the complyctl repository includes both the complyctl core and two plugins (openscap and ampel). While the core is more stable, plugins can change more often by adopting new plugins, removing old plugins or including plugin features. The goal is to move the plugins and related documentation to complytime-providers (fork already available locally) while making the complyctl repository much cleaner. When moving the plugins lets also update the terminology to use "providers" instead of "plugins" and lets consider the impact in all existing process in complyctl repository. After the move, we must ensure the RPM and testing-farm configurations are updated, but this should be done in a separate spec."

## Overview

The complyctl repository currently co-locates the core compliance engine with two provider implementations — OpenSCAP and AMPEL — that evolve at different cadences than the core. This creates friction: provider changes trigger full core CI runs, core refactors ripple into provider code, and the repository boundary does not reflect the independent lifecycle of provider authors.

This feature extracts the two existing providers (`cmd/openscap-plugin/` and `cmd/ampel-plugin/`) plus all provider-specific documentation into the standalone `complytime-providers` repository. In parallel, all uses of the term "plugin" are replaced with "provider" across complyctl's codebase, configuration, documentation, and process files, so that terminology aligns with the architectural reality that providers are independently authored, discoverable binaries — not plugins embedded in the core.

**Out of scope**: RPM spec (`complyctl.spec`) and testing-farm / Packit configuration updates are deliberately deferred to a separate spec as directed by the user.

---

## Clarifications

### Session 2026-04-15

- Q: Should `pkg/plugin/` be renamed to `pkg/provider/` as part of this feature, or should only the identifiers inside it be renamed while the directory stays named "plugin"? → A: Rename `pkg/plugin/` → `pkg/provider/` in this feature; update all internal imports (Option A).
- Q: Should `complytime-providers` use a single `go.mod` for all providers (monorepo) or a separate `go.mod` per provider? → A: Single `go.mod` at the root of `complytime-providers` covering all providers (Option A).
- Q: After the split, how should the `ci_compliance.yml` workflow in complyctl obtain the `complyctl-provider-ampel` binary? → A: Download a pre-built provider binary from a `complytime-providers` GitHub release artifact (Option B).
- Q: Must a complyctl SDK tagged release be published before provider migration work begins, or can both repositories be developed in parallel? → A: Parallel development is allowed; `complytime-providers` MAY use a temporary `replace` directive during development, which MUST be removed before the final merge (Option B).
- Q: Should Go constant identifiers carrying the hashicorp/go-plugin handshake values be renamed as part of the terminology rename? → A: Rename the constant identifiers (e.g., `PluginHandshake` → `ProviderHandshake`) but preserve the string values (`"COMPLYCTL_PLUGIN"`, UUID) exactly — no runtime impact (Option A).

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Core Contributor Works Without Provider Code (Priority: P1)

A core complyctl contributor clones the repository, builds the project, runs all tests, and contributes a change — without encountering any provider-specific source code, build steps, or documentation.

**Why this priority**: This is the primary motivation for the split. If the core repository remains cluttered with provider code after the feature is complete, the goal has not been achieved. It is independently testable because the core build and test suite can be validated before the providers repository is touched.

**Independent Test**: Clone `complyctl` from a clean state, run `make build test-unit`, and verify the output contains no provider binary artifacts and no compilation steps that reference `openscap` or `ampel` source directories.

**Acceptance Scenarios**:

1. **Given** a clean checkout of complyctl after the split, **When** a contributor runs `make build`, **Then** only the `complyctl` CLI binary is produced — no `complyctl-provider-openscap` or `complyctl-provider-ampel` binaries are built from within the complyctl repository.
2. **Given** the complyctl repository after the split, **When** a contributor searches for references to `cmd/openscap-plugin` or `cmd/ampel-plugin` directories, **Then** no such directories or files exist.
3. **Given** the complyctl repository after the split, **When** `make test-unit` is executed, **Then** all tests pass and no test imports or references from the removed provider directories exist.
4. **Given** the complyctl repository after the split, **When** a contributor reads any documentation file, **Then** no file uses the term "plugin" to refer to providers — only "provider" terminology appears.

---

### User Story 2 — Provider Contributor Works in the Providers Repository (Priority: P1)

A provider author clones `complytime-providers`, builds and tests both providers, and submits a change — entirely within the providers repository, with no dependency on a complyctl source checkout beyond the published module.

**Why this priority**: Equally critical to Story 1. The providers repository must be self-contained and independently buildable. Shared priority with Story 1 because neither is useful without the other.

**Independent Test**: Clone `complytime-providers` from a clean state (without a local `complyctl` checkout), resolve dependencies from published module versions, build both providers, run their unit tests, and verify both provider binaries are produced correctly.

**Acceptance Scenarios**:

1. **Given** a clean checkout of `complytime-providers` with no local `complyctl` source tree, **When** a contributor builds the providers, **Then** both `complyctl-provider-openscap` and `complyctl-provider-ampel` binaries are produced successfully using only published complyctl SDK versions.
2. **Given** the `complytime-providers` repository, **When** a contributor runs the provider test suite, **Then** all tests pass independently of the complyctl source tree.
3. **Given** a new provider author, **When** they read the provider authoring guide in `complytime-providers`, **Then** they find complete documentation including the provider contract, naming convention, discovery mechanism, and variable model — without needing to consult the complyctl repository.
4. **Given** the `complytime-providers` repository after migration, **When** a contributor searches for "plugin" in any file, **Then** no references use "plugin" as terminology for the provider concept (only in historical or third-party contexts such as hashicorp/go-plugin library references are acceptable).

---

### User Story 3 — Operator Installs and Uses Providers Identically to Before (Priority: P2)

An operator who uses complyctl with openscap or ampel providers experiences no change in runtime behavior, binary names, discovery paths, or workspace layout after the repository split.

**Why this priority**: The split must be invisible at runtime. Operators must not need to change how they install or invoke providers. This is lower priority than the migration itself because it is a constraint on the migration rather than a new capability.

**Independent Test**: On a system with `complyctl` and a pre-split provider binary installed, verify that the provider is discovered and functions identically after the split. Can be tested end-to-end using the existing E2E test suite adapted for a separately-built provider binary.

**Acceptance Scenarios**:

1. **Given** a provider binary named `complyctl-provider-openscap` installed to `~/.complytime/providers/`, **When** a user runs `complyctl scan`, **Then** the provider is discovered and invoked identically to how it was before the repository split.
2. **Given** the complyctl `doctor` command, **When** an operator runs `complyctl doctor`, **Then** provider health checks succeed for providers installed from the `complytime-providers` repository.
3. **Given** a workspace created before the split, **When** an operator uses `complyctl generate` and `complyctl scan` with a provider installed from the new repository, **Then** all workspace artifacts (policy, results) are produced at the same paths as before.

---

### User Story 4 — Terminology Is Consistent Across All Surfaces (Priority: P2)

A user or contributor reading any complyctl artifact — CLI output, help text, documentation, configuration, code comments — encounters only the term "provider" (not "plugin") when referring to the extensibility mechanism.

**Why this priority**: Consistent terminology reduces confusion and ensures the repository accurately reflects the architecture. It is a horizontal concern applied across all other stories.

**Independent Test**: Run a full-text search across the complyctl repository for the word "plugin" and verify that all matches are either: (a) references to the `hashicorp/go-plugin` library name (acceptable), (b) historical commit messages, or (c) test fixtures with explicit justification.

**Acceptance Scenarios**:

1. **Given** the complyctl CLI, **When** a user runs `complyctl --help` or any subcommand help, **Then** all output uses "provider" terminology — no mention of "plugin" in user-visible text.
2. **Given** the complyctl source code, **When** a contributor searches for identifiers containing "Plugin" (e.g., `PluginExecutablePrefix`, `SupportedPlugins`), **Then** all such identifiers have been renamed to use "Provider" equivalents (e.g., `ProviderExecutablePrefix`, `SupportedProviders`).
3. **Given** the complyctl CI workflow files, **When** reviewed, **Then** no workflow name, step name, or environment variable uses "plugin" to refer to a provider concept.
4. **Given** the Makefile, **When** reviewed, **Then** all targets and variable names use "provider" terminology (e.g., `build-test-provider` instead of `build-test-plugin`).

---

### Edge Cases

- What happens when a user has a provider binary from the old build path installed — is it still discovered and does it still work after the terminology rename?
- How does the complyctl SDK (`pkg/plugin/`) get consumed by the providers repository if it has not been published as a versioned module — does the migration require a tagged release of complyctl first?
- What happens to the `replace` directive in `cmd/openscap-plugin/go.mod` that currently points to the parent complyctl source tree — how is this removed without breaking local development of providers?
- What happens to CI workflows in complyctl that currently build or reference provider source directories (e.g., `make build` which includes `cd cmd/openscap-plugin`)?
- What happens to the `ci_compliance.yml` daily workflow that uses the `ampel-bp` policy — does it need to install the ampel provider from the new repository?
- What happens if the terminology rename (`Plugin` → `Provider`) in Go identifiers breaks the `hashicorp/go-plugin` handshake? — Resolved: only Go constant *identifiers* are renamed; the handshake string values (`"COMPLYCTL_PLUGIN"`, UUID, protocol version) are preserved byte-for-byte, ensuring existing installed provider binaries remain compatible (FR-013).
- How is the `api/plugin/` protobuf package path handled — the package is named `complyctl.plugin.v1` and is consumed by the proto contract; renaming the directory may break generated code compatibility.

---

## Requirements *(mandatory)*

### Functional Requirements

#### Providers Repository (`complytime-providers`)

- **FR-001**: The `complytime-providers` repository MUST contain the full source code of the OpenSCAP provider (equivalent to current `cmd/openscap-plugin/`) as a Go package within the repository's single root Go module.
- **FR-002**: The `complytime-providers` repository MUST contain the full source code of the AMPEL provider (equivalent to current `cmd/ampel-plugin/`) as a Go package within the repository's single root Go module.
- **FR-003**: The `complytime-providers` repository MUST use a single `go.mod` at the repository root that covers all providers. Each provider MUST be buildable without a local checkout of the complyctl repository — all complyctl SDK dependencies MUST be resolved from a published, versioned module reference. The existing `replace` directive in `cmd/openscap-plugin/go.mod` MUST NOT be carried over.
- **FR-004**: The `complytime-providers` repository MUST include the provider authoring guide (equivalent to current `docs/PLUGIN_GUIDE.md`) updated to use "provider" terminology throughout.
- **FR-005**: The `complytime-providers` repository MUST include a Makefile with targets equivalent to the provider-related targets currently in the complyctl Makefile (build, test, vendor, lint).
- **FR-006**: Provider binaries produced from `complytime-providers` MUST retain the naming convention `complyctl-provider-<name>` so that complyctl's discovery mechanism continues to function without modification.

#### complyctl Core Repository

- **FR-007**: The `cmd/openscap-plugin/` directory MUST be removed from the complyctl repository.
- **FR-008**: The `cmd/ampel-plugin/` directory MUST be removed from the complyctl repository.
- **FR-009**: The complyctl Makefile MUST be updated to remove all targets and build steps that reference the removed provider source directories (e.g., the `cd cmd/openscap-plugin` step in `build` and `vendor`).
- **FR-010**: The complyctl CI workflows MUST be updated to remove all steps that build, test, or reference the removed provider source directories. The `ci_compliance.yml` daily workflow MUST be updated to download the `complyctl-provider-ampel` binary from a published `complytime-providers` GitHub release artifact rather than building it from the complyctl source tree.
- **FR-011**: The `docs/PLUGIN_GUIDE.md` file MUST be removed from the complyctl repository (its content migrated to `complytime-providers`).
- **FR-012**: All man page sources that document provider-specific CLI behavior MUST be reviewed; provider-specific man pages MUST be moved to `complytime-providers` and the Makefile updated accordingly.

#### Terminology Rename ("plugin" → "provider")

- **FR-013**: All Go exported and unexported identifiers in `pkg/plugin/` (to be renamed `pkg/provider/`) that use "Plugin" in their name MUST be renamed to use "Provider". The exception is identifiers whose names are dictated by the `hashicorp/go-plugin` library interface (e.g., method names on `goplugin.Plugin`). Handshake-related constant identifiers (e.g., `PluginHandshake`) SHOULD be renamed to "Provider" equivalents, but this is a convention preference — their renaming carries no functional impact and may be skipped at the implementer's discretion. The string *values* of handshake constants (the magic cookie key `"COMPLYCTL_PLUGIN"` and its UUID value) MUST remain unchanged to preserve runtime compatibility with existing installed provider binaries.
- **FR-014**: The constant `PluginExecutablePrefix` in `internal/complytime/consts.go` MUST be renamed to `ProviderExecutablePrefix`. Its value (`"complyctl-provider-"`) MUST remain unchanged to preserve binary compatibility with existing installed providers.
- **FR-015**: All user-visible CLI output, help text, and error messages that use the word "plugin" to refer to the provider concept MUST be updated to use "provider".
- **FR-016**: All documentation files remaining in the complyctl repository that use "plugin" to refer to the provider concept MUST be updated to use "provider".
- **FR-017**: All CI workflow file names, job names, step names, and environment variables that use "plugin" to refer to the provider concept MUST be updated to use "provider".
- **FR-018**: Makefile target names and variable names that use "plugin" (e.g., `build-test-plugin`, `MAN_OPENSCAP_PLUGIN`) MUST be renamed to use "provider" equivalents.
- **FR-019**: The `api/plugin/` protobuf package directory name and the `complyctl.plugin.v1` package declaration MUST be preserved as-is in this feature. A rename to `complyctl.provider.v1` is deferred to a planned future major version of the proto contract with a formal migration period, ensuring no breaking wire-format changes are introduced now. The decision to defer is documented as an assumption.
- **FR-020**: The `pkg/plugin/` Go package directory MUST be renamed to `pkg/provider/` as part of this feature. All internal complyctl import paths referencing `pkg/plugin` MUST be updated to `pkg/provider`. The published module path for the SDK consumed by providers will reflect this rename in the tagged release that unblocks `complytime-providers`.
- **FR-021**: The complyctl `cmd/test-plugin/` reference implementation MUST be renamed to `cmd/test-provider/` and its Makefile target updated to `build-test-provider`.

### Key Entities

- **Provider**: An independently buildable binary that implements the complyctl provider contract (Describe, Generate, Scan), discoverable via the `complyctl-provider-<name>` naming convention. Formerly called "plugin."
- **Provider SDK**: The set of Go packages in complyctl (`pkg/provider/`, `api/plugin/`) that define the contract and runtime harness providers must implement. Consumed by providers as a published Go module dependency.
- **complytime-providers repository**: The destination repository (`github.com/complytime/complytime-providers`) that hosts all first-party provider implementations and provider authoring documentation. Organized as a monorepo with a single root `go.mod` covering all providers.
- **Discovery path**: The filesystem directories (`~/.complytime/providers/` and `/usr/libexec/complytime/providers/`) where complyctl searches for provider binaries at runtime.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The complyctl repository contains zero source files under `cmd/openscap-plugin/` or `cmd/ampel-plugin/` after the migration.
- **SC-002**: A full-text search of the complyctl repository for the word "plugin" (case-insensitive) in Go source files, Makefile, CI workflow files, and documentation returns zero matches where the term refers to the provider extensibility concept (library names such as `hashicorp/go-plugin` are exempt).
- **SC-003**: Both providers build successfully from the `complytime-providers` repository without a local complyctl source tree present, using only published module versions.
- **SC-004**: All existing complyctl unit tests pass after the migration, with no test failures attributable to the removal of provider code or the terminology rename.
- **SC-005**: All existing complyctl E2E tests pass after the migration, using a provider binary built from `complytime-providers` rather than from `complyctl`.
- **SC-006**: A provider binary built from `complytime-providers` is discovered and invoked correctly by complyctl without any change to the complyctl discovery mechanism.
- **SC-007**: The complyctl `make build` target produces only the `complyctl` CLI binary — no provider binaries — and completes without errors.
- **SC-008**: The complyctl CI pipeline (all existing workflows) passes without modification to any step that previously built or tested provider source directories.

---

## Assumptions

- The `complytime-providers` repository MAY use a temporary `replace` directive pointing to a local complyctl checkout during development. This directive MUST be removed and replaced with a published versioned reference (`require github.com/complytime/complyctl vX.Y.Z`) before any code is merged to the `complytime-providers` main branch. A tagged complyctl release is therefore a merge gate, not a development prerequisite.
- The `hashicorp/go-plugin` handshake string values (magic cookie key `"COMPLYCTL_PLUGIN"`, its UUID value, and protocol version) are treated as a stable wire contract and MUST NOT change as part of the terminology rename. Renaming the Go constant *identifiers* that carry these values (e.g., `PluginHandshake` → `ProviderHandshake`) is a convention preference with no functional impact and is left to the implementer's discretion (FR-013).
- The `api/plugin/` protobuf directory and `complyctl.plugin.v1` package declaration are NOT renamed in this feature. Renaming is explicitly deferred to a future planned major version of the proto contract to avoid introducing breaking wire-format changes for existing providers. This decision was made to preserve wire compatibility (Option C).
- The `cmd/test-plugin/` (to be renamed `cmd/test-provider/`) remains in the complyctl repository as a reference implementation and CI test harness — it is NOT moved to `complytime-providers`.
- RPM spec and testing-farm / Packit configuration updates are explicitly deferred to a separate specification as stated in the user description.
- The `ci_compliance.yml` daily workflow in complyctl that uses the AMPEL provider (`ampel-bp` policy) MUST be updated to download `complyctl-provider-ampel` from a published `complytime-providers` GitHub release artifact (FR-010). This requires `complytime-providers` to publish release binaries before the complyctl CI workflow update can be validated.
- The `pkg/plugin/` package directory is renamed to `pkg/provider/` as part of this feature (FR-020). All internal import paths are updated. The renamed path is reflected in the tagged complyctl release that providers will depend on.

---

## Dependencies

- A tagged release of `github.com/complytime/complyctl` exposing the renamed provider SDK (`pkg/provider/`) MUST be published before the final merge of `complytime-providers`. During development, `complytime-providers` MAY temporarily use a `replace` directive pointing to a local complyctl checkout. The `replace` directive MUST be removed and replaced with a published version reference before any code is merged to the `complytime-providers` main branch.
- The `complytime-providers` repository exists as a stub containing only `LICENSE` and `README.md` — all provider source code migration is new work.
- RPM and testing-farm configuration (deferred spec) depends on this feature being completed first, as the RPM sub-package for openscap-provider will need to reference the new source location.
