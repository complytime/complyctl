# Tasks: RPM Packaging and CI for Split Repositories

**Input**: Design documents from `/specs/005-rpm-packaging-ci/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Includes exact file paths in descriptions

## Repositories

This feature spans two repositories:

- **complyctl** (this repo): `/home/maburgha/GIT/ProdSec/complyctl/`
- **complytime-providers** (external): `github.com/complytime/complytime-providers`

Tasks for complytime-providers are marked with `[providers-repo]` and must be
executed in a clone of that repository.

---

## Phase 1: Foundational (Blocking Prerequisites)

**Purpose**: Updates that MUST be complete before RPM spec work can begin

- [x] T001 Update man page source with provider terminology in docs/man/complyctl.md -- replace any remaining "plugin" references with "provider" throughout the file. The file uses pandoc man page format (% header lines, # sections). Preserve the pandoc metadata header. Review all sections: NAME, SYNOPSIS, DESCRIPTION, COMMANDS, OPTIONS, FILES, EXAMPLES, SEE ALSO. Ensure references to provider binaries use the `complyctl-provider-<name>` naming convention and discovery paths match `/usr/libexec/complytime/providers/` and `~/.complytime/providers/`.
- [x] T002 Regenerate man page from updated source by running `make man` in the complyctl repository root. This produces docs/man/complyctl.1 from docs/man/complyctl.md using pandoc. Verify the output file is updated. Requires pandoc to be installed. If pandoc is not available, note the dependency for CI but do not block -- the pre-built complyctl.1 in the repo is acceptable for now.

**Checkpoint**: Man page source and generated output are ready. RPM spec work can begin.

---

## Phase 2: User Story 1 - complyctl RPM Builds and Installs Cleanly (Priority: P1)

**Goal**: Fix the broken complyctl RPM spec so it builds only the complyctl binary (no provider sub-packages), includes the man page, and follows Fedora Go packaging guidelines with automatic bundled dependency declarations.

**Independent Test**: Run `rpmbuild -bs complyctl.spec` or `packit build locally` and verify a single `complyctl` binary RPM is produced with no provider sub-packages. Verify `rpm -qlp` shows `/usr/bin/complyctl`, man page, LICENSE, and owned provider directories.

### Implementation for User Story 1

- [x] T003 [US1] Rewrite complyctl.spec to remove provider sub-package and simplify for core-only delivery. Specific changes to complyctl.spec:
  **Preamble**: Keep `%global goipath`, `%global base_url`, `%global app_dir complytime`, `%global debug_package %{nil}`. Evaluate `%global gopath %{_builddir}/go` (line 6 of current spec) -- this overrides the default from `go-rpm-macros`. Keep it only if the build fails without it; otherwise remove it to align with standard Go RPM macro behavior. Update `Summary` to remove "pluggable providers" language (use "provider" if mentioned). Change `Release: 0%{?dist}` to `Release: 1%{?dist}` (standard Fedora convention for released versions). Keep `BuildRequires: golang` and `BuildRequires: go-rpm-macros`. Keep `%gometa -f`.
  **Remove entirely**: The `%package openscap-provider` section, its `Summary`, `Requires`, and `%description openscap-provider` block. Remove `%files openscap-provider` section.
  **%build section**: Remove `cd cmd/openscap-plugin` build step and the `cd ../..` return. Change `go build ... ./cmd/...` to `go build -buildmode=pie -o ${GO_BUILD_BINDIR}/complyctl -ldflags="${GO_LD_EXTRAFLAGS}" ./cmd/complyctl`. Keep the `%set_build_flags`, `GO_LD_EXTRAFLAGS`, `GO111MODULE=on`, and `BUILD_DATE_GO` setup as-is.
  **%install section**: Remove `install ... complyctl-provider-openscap` line. Keep `install ... complyctl` line. Add man page: `install -d %{buildroot}%{_mandir}/man1` and `install -p -m 0644 docs/man/complyctl.1 %{buildroot}%{_mandir}/man1/complyctl.1`. Keep the provider directory creation (`install -d ... providers`). Do NOT manually install `vendor/modules.txt` here -- RPM's `%license` directive in `%files` handles it automatically (same mechanism used for LICENSE today).
  **%check section**: Remove `cd cmd/openscap-plugin && go test ...` and `cd ../..`. Keep `go test -mod=vendor -race -v ./...` (alternatively, use `%gocheck` which is the standard Fedora Go macro and handles `-race` flag availability per architecture automatically -- the `-race` flag requires CGO which may not be available on all build targets).
  **%files section**: Keep `%attr(0755, root, root) %{_bindir}/complyctl`. Add `%{_mandir}/man1/complyctl.1*`. Change `%license LICENSE` to `%license LICENSE vendor/modules.txt`. Add `%doc README.md`. Keep `%dir %{_libexecdir}/%{app_dir}` and `%dir %{_libexecdir}/%{app_dir}/providers`. Remove the `Requires: scap-security-guide` (now in providers spec).
  **%changelog**: Add new entry for the current date.
  **References**: FR-001 through FR-007, FR-025, SC-001, SC-011. See data-model.md for file ownership layout.

- [x] T004 [P] [US1] Verify complyctl .packit.yaml needs no changes for simplified spec in .packit.yaml -- the existing config references `complyctl.spec` and syncs `complyctl.spec` + `.packit.yaml`. This should still be correct after the spec update. Verify `specfile_path`, `files_to_sync`, and all job configurations are valid. No changes expected (FR-016).

**Checkpoint**: complyctl RPM spec builds a single binary package. `rpmlint complyctl.spec` passes. The existing TMT plan (`plans/test-RPM-provide-content.fmf`) and FMF root (`.fmf/version`) are already in place (FR-020, FR-022).

---

## Phase 3: User Story 2 - Provider Sub-Packages Build From complytime-providers (Priority: P1)

**Goal**: Create a complete RPM packaging pipeline in the complytime-providers repository: spec file producing two sub-packages, Packit CI/CD, FMF metadata, and TMT smoke tests.

**Independent Test**: In the complytime-providers repo, run `rpmbuild -bs complytime-providers.spec` and verify two binary sub-packages are produced (`complytime-providers-openscap` and `complytime-providers-ampel`) with no main package. Verify `rpm -qp --requires` shows `complyctl >= X.Y.Z`.

**NOTE**: All tasks in this phase are executed in the `complytime-providers` repository.

### Implementation for User Story 2

- [x] T005 [P] [US2] [providers-repo] Create complytime-providers.spec in the complytime-providers repository root. The spec must follow the Fedora Go packaging guidelines with vendored dependencies. Structure:
  **First line**: `# SPDX-License-Identifier: Apache-2.0` (required by constitution for all source files).
  **Preamble**: `%global goipath github.com/complytime/complytime-providers`, `%global base_url https://%{goipath}`, `%global app_dir complytime`, `%global debug_package %{nil}`. Do NOT include `%global gopath` -- let `go-rpm-macros` manage the build directory via `%goprep`. `Name: complytime-providers`, `Version:` (set to current or `0.0.1`), `Release: 1%{?dist}`, `Summary: Compliance scanning providers for complyctl`, `License: Apache-2.0`, `URL: %{base_url}`, `Source0: %{base_url}/archive/refs/tags/v%{version}.tar.gz`. `BuildRequires: golang` and `BuildRequires: go-rpm-macros`. `%gometa -f`. Main `%description` describes the source package purpose.
  **Sub-packages**: `%package openscap` with `Summary: OpenSCAP scanning provider for complyctl`, `Requires: complyctl >= X.Y.Z` (set X.Y.Z to the first post-spec-004 release version), `Requires: scap-security-guide`. `%description openscap` explains the OpenSCAP provider. `%package ampel` with `Summary: Ampel scanning provider for complyctl`, `Requires: complyctl >= X.Y.Z`. `%description ampel` explains the Ampel provider.
  **%prep**: `%goprep -k` (preserves vendor directory).
  **%build**: `%set_build_flags`, `export GO111MODULE=on`, `GO_BUILD_BINDIR=./bin`, `mkdir -p ${GO_BUILD_BINDIR}`. Build both: `go build -buildmode=pie -o ${GO_BUILD_BINDIR}/complyctl-provider-openscap ./cmd/openscap-provider` and `go build -buildmode=pie -o ${GO_BUILD_BINDIR}/complyctl-provider-ampel ./cmd/ampel-provider`. No version injection LD flags needed (providers have no version package).
  **%install**: `install -d %{buildroot}%{_libexecdir}/%{app_dir}/providers`. Install both binaries with `install -p -m 0755`. Do NOT manually install `vendor/modules.txt` -- RPM's `%license` directive in `%files` handles it automatically.
  **%check**: `go test -mod=vendor -v ./...` (alternatively, use `%gocheck` which is the standard Fedora Go macro and handles `-race` flag availability per architecture automatically).
  **No main %files section** (no main binary RPM produced). `%files openscap`: provider binary, `%license LICENSE vendor/modules.txt`, `%doc README.md`. `%files ampel`: provider binary, `%license LICENSE vendor/modules.txt`, `%doc README.md`.
  **%changelog**: Initial entry.
  **References**: FR-008 through FR-015, SC-002, SC-003. See data-model.md and contracts/packaging-interface.md.

- [x] T006 [P] [US2] [providers-repo] Create .packit.yaml in the complytime-providers repository root. Model it after the complyctl .packit.yaml with these values: `upstream_project_url: https://github.com/complytime/complytime-providers`, `upstream_tag_template: v{version}`, `upstream_package_name: complytime-providers`, `downstream_package_name: complytime-providers`, `specfile_path: complytime-providers.spec`, `files_to_sync: [complytime-providers.spec, .packit.yaml]`. Jobs: `copr_build` (trigger: pull_request, targets: fedora-rawhide-x86_64, fedora-43-x86_64, fedora-42-x86_64, centos-stream-9-x86_64, centos-stream-10-x86_64), `tests` (same trigger and targets), `propose_downstream` (trigger: release, dist_git_branches: rawhide, f43, f42), `koji_build` (trigger: commit, same branches), `bodhi_update` (trigger: commit, dist_git_branches: f43, f42). References: FR-017, FR-018, FR-019.

- [x] T007 [P] [US2] [providers-repo] Create FMF metadata root at .fmf/version in the complytime-providers repository. Create the directory `.fmf/` and write a single file `version` containing just `1` (no trailing newline or extra content). This enables testing-farm to discover TMT plans. Reference: FR-022.

- [x] T008 [P] [US2] [providers-repo] Create TMT test plan at plans/test-RPM-providers.fmf in the complytime-providers repository. Create the `plans/` directory. The plan validates that both provider binaries are installed at the expected path after RPM installation. Content:
  ```
  summary: Validate complytime-providers RPM sub-packages deliver provider binaries

  execute:
      script:
        - test -x /usr/libexec/complytime/providers/complyctl-provider-openscap
        - test -x /usr/libexec/complytime/providers/complyctl-provider-ampel
  ```
  This uses `test -x` to verify files exist and are executable. Matches the inline script pattern used by complyctl's existing TMT plan. Reference: FR-021. See contracts/packaging-interface.md TMT Test Contract section.

**Checkpoint**: complytime-providers repo has a complete packaging pipeline. `rpmlint complytime-providers.spec` passes. Both sub-packages build successfully. TMT plan is discoverable.

---

## Phase 4: User Story 3 - complyctl Installs Without Providers (Priority: P2)

**Goal**: Validate that the complyctl RPM installs and operates correctly on a system with no provider packages installed.

**Independent Test**: Install the complyctl RPM on a clean Fedora system (or mock chroot). Verify `complyctl version` and `complyctl --help` succeed. Verify no provider-related dependency errors occur.

**NOTE**: This story requires no new files. It is validated by the US1 implementation -- the updated complyctl.spec produces a standalone RPM with no provider dependencies.

### Validation for User Story 3

- [x] T009 [US3] Validate complyctl RPM has no provider dependencies by inspecting the built RPM: run `rpm -qp --requires` on the built complyctl RPM and verify no `complytime-providers` or `scap-security-guide` entries appear. Verify the `%files` section does not include any provider binaries. This confirms FR-001, FR-003, SC-004.

**Checkpoint**: complyctl is confirmed independently installable without providers.

---

## Phase 5: User Story 4 - Automated Fedora Package Updates on Release (Priority: P2)

**Goal**: Ensure the release pipeline works end-to-end: GoReleaser produces a clean GitHub release for complyctl, Packit reacts to releases in both repos, and the release process documentation reflects the new split structure.

**Independent Test**: Run `goreleaser check` and `goreleaser build --snapshot --clean` in complyctl repo. Verify no errors referencing `openscap-plugin`. Review RELEASE_PROCESS.md for accuracy.

### Implementation for User Story 4

- [x] T010 [P] [US4] Update .goreleaser.yaml to remove the openscap-plugin build entry. Remove lines 27-31 (the entire `- id: openscap-plugin` build block including `binary`, `dir`, `main`, and `goos` fields). Keep the `- id: complyctl` build block unchanged. Run `goreleaser check` to validate syntax. Run `goreleaser build --snapshot --clean` to verify only the complyctl binary is produced. Reference: FR-023, SC-010.

- [x] T011 [P] [US4] Update docs/RELEASE_PROCESS.md to reflect the split packaging structure. Add a new section or update the existing "Fedora Package" section to note: (1) complyctl and complytime-providers are now independent Fedora packages with separate release cycles. (2) Each repository has its own `.packit.yaml` that automates downstream PRs, Koji builds, and Bodhi updates. (3) The complytime-providers Fedora package requires a one-time Fedora package review before automation is functional. (4) Link to the complytime-providers repository. Keep the existing manual fallback process documentation. Reference: FR-024.

**Checkpoint**: GoReleaser builds cleanly. Release documentation is accurate for the new multi-repo structure.

---

## Phase 6: User Story 5 - Testing-Farm Validates RPMs on Each PR (Priority: P2)

**Goal**: Confirm both repositories have the complete testing-farm integration: FMF metadata root, TMT plans, and Packit `tests` job configuration.

**Independent Test**: Run `tmt lint` in both repos. Verify Packit configs include `tests` jobs. Submit a PR to validate testing-farm triggers.

**NOTE**: This story requires no new files. The testing-farm infrastructure is delivered by US1 (complyctl already has TMT plan + Packit tests job) and US2 (complytime-providers gets TMT plan + Packit tests job).

### Validation for User Story 5

- [x] T012 [US5] Validate testing-farm readiness in both repositories. In complyctl: verify `.fmf/version` exists, `plans/test-RPM-provide-content.fmf` exists and is valid, and `.packit.yaml` includes a `tests` job. In complytime-providers: verify `.fmf/version`, `plans/test-RPM-providers.fmf`, and `.packit.yaml` `tests` job are all present. Run `tmt lint` in both repos if tmt is available. References: FR-020, FR-021, FR-022, SC-005, SC-006.

**Checkpoint**: Both repos are ready for testing-farm CI on PRs.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and cleanup across both repositories

- [x] T013 [P] Run rpmlint on complyctl.spec and fix any warnings or errors. Focus on: license field, summary length, description quality, macro usage, file permissions.
- [x] T014 [P] Run rpmlint on complytime-providers.spec (in providers repo) and fix any warnings or errors.
- [x] T015 Verify Packit configuration validity by running `packit validate-config` in both repos (if packit CLI is available). Confirm all job triggers, targets, and dist-git branches are correct per the CI target matrix in data-model.md.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 1)**: No dependencies -- can start immediately
- **US1 (Phase 2)**: Depends on Foundational (man page must be ready for RPM spec)
- **US2 (Phase 3)**: No dependency on US1 -- can start in parallel (different repo)
- **US3 (Phase 4)**: Depends on US1 completion (validates the complyctl RPM)
- **US4 (Phase 5)**: Can start after Foundational (GoReleaser + docs are independent files)
- **US5 (Phase 6)**: Depends on US1 + US2 (validates infrastructure from both)
- **Polish (Phase 7)**: Depends on US1 + US2 + US4

### User Story Dependencies

- **US1 (P1)**: Depends on Foundational only. Produces: updated complyctl.spec
- **US2 (P1)**: Independent of US1. Produces: complytime-providers.spec, .packit.yaml, .fmf/version, TMT plan
- **US3 (P2)**: Depends on US1. Validation only (no new files)
- **US4 (P2)**: Depends on Foundational only. Produces: updated .goreleaser.yaml, updated RELEASE_PROCESS.md
- **US5 (P2)**: Depends on US1 + US2. Validation only (no new files)

### Parallel Opportunities

- **US1 and US2 can run in parallel** (different repos, no shared files)
- **US1 and US4 can run in parallel** after Foundational (different files in same repo)
- **Within US2**: All 4 tasks (T005-T008) can run in parallel (different files)
- **Within US4**: T010 and T011 can run in parallel (different files)
- **Polish**: T013 and T014 can run in parallel (different repos)

---

## Parallel Example: US1 + US2 + US4 After Foundational

```text
# After Foundational (T001, T002) completes:

# Stream A (complyctl repo - US1):
Task: T003 "Rewrite complyctl.spec"
Task: T004 "Verify .packit.yaml"

# Stream B (complytime-providers repo - US2, all parallel):
Task: T005 "Create complytime-providers.spec"
Task: T006 "Create .packit.yaml"
Task: T007 "Create .fmf/version"
Task: T008 "Create plans/test-RPM-providers.fmf"

# Stream C (complyctl repo - US4, parallel with Stream A):
Task: T010 "Update .goreleaser.yaml"
Task: T011 "Update RELEASE_PROCESS.md"
```

---

## Implementation Strategy

### MVP First (US1 Only)

1. Complete Foundational (T001-T002) -- man page update
2. Complete US1 (T003-T004) -- fix complyctl RPM
3. **STOP and VALIDATE**: `rpmlint complyctl.spec`, test local build
4. Submit PR to complyctl -- Packit/COPR build + testing-farm validates

### Incremental Delivery

1. Foundational → man page ready
2. US1 → complyctl RPM fixed → PR validates via existing CI (MVP!)
3. US2 → complytime-providers has packaging → PR validates via new CI
4. US4 → GoReleaser + docs updated → release pipeline unblocked
5. US3 + US5 → validation confirms everything works together
6. Polish → rpmlint + packit validate-config clean

### Parallel Strategy

With two developers or agents:

1. Both complete Foundational together
2. Once Foundational is done:
   - Agent A: US1 (complyctl spec) + US4 (goreleaser + docs)
   - Agent B: US2 (complytime-providers, all 4 tasks in parallel)
3. Both validate: US3, US5, Polish

---

## Notes

- [P] tasks = different files, no dependencies on incomplete tasks
- [providers-repo] = execute in complytime-providers clone, not complyctl
- US3 and US5 are validation-only (no new implementation files)
- The `Requires: complyctl >= X.Y.Z` version in complytime-providers.spec must be set to the actual first post-spec-004 complyctl release version at implementation time
- Fedora package review for complytime-providers is a manual prerequisite outside this task list
- Commit after each task or logical group
- Stop at any checkpoint to validate independently
