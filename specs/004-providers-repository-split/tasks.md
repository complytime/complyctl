# Tasks: Providers Repository Split

**Input**: Design documents from `specs/004-providers-repository-split/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/provider-sdk.md

> **Sequencing gate**: This feature MUST be implemented AFTER PR #463 (`Export` RPC) and
> PR #479 (`ResolveVersion`) have merged to `main`. The complyctl SDK tagged release that
> unblocks `complytime-providers` must include both.

---

## Phase 1: Foundation — complyctl SDK Rename (complyctl repo)

**Purpose**: Rename `pkg/plugin/` to `pkg/provider/` and update all internal imports.
This is the blocking prerequisite for every other task — the published SDK path is what
`complytime-providers` will depend on.

**⚠️ CRITICAL**: No provider migration or CI updates can be finalized until this phase
produces a tagged complyctl SDK release.

- [x] T001 [US2] Rename directory `pkg/plugin/` → `pkg/provider/` in complyctl
- [x] T002 [US2] Update package declaration in all files under `pkg/provider/` from `package plugin` to `package provider`
- [x] T003 [US2] Rename `pkg/provider/manager.go`: `type Plugin interface` → `type Provider interface`; `LoadedPlugin` → `LoadedProvider`; `GetPlugin` → `GetProvider`; `ListPlugins` → `ListProviders`; `LoadPlugins` → `LoadProviders`; update all internal references
- [x] T004 [US2] Rename `pkg/provider/discovery.go`: `type PluginInfo struct` → `type ProviderInfo struct`; `PluginID` field → `ProviderID`; `pluginDir` field → `providerDir`; `DiscoverPlugins` → `DiscoverProviders`
- [x] T005 [US2] Rename `pkg/provider/plugin.go`: `SupportedPlugins` → `SupportedProviders` (SHOULD); `GRPCEvaluatorPlugin.Impl` field type from `Plugin` → `Provider`; update `Serve(impl Plugin)` → `Serve(impl Provider)`
- [x] T006 [US2] Update all complyctl import paths from `github.com/complytime/complyctl/pkg/plugin` → `github.com/complytime/complyctl/pkg/provider` across the entire complyctl module (grep for `pkg/plugin`)
- [x] T007 [US4] Rename `internal/complytime/consts.go`: `PluginExecutablePrefix` → `ProviderExecutablePrefix` (value `"complyctl-provider-"` unchanged); update all references inside `pkg/provider/discovery.go` and any other callers
- [x] T008 [US4] Update `cmd/complyctl/cli/providers.go`: rename all user-visible strings, help text, and log messages from "plugin" → "provider"
- [x] T009 [US4] Update `cmd/complyctl/cli/scan.go`: rename all user-visible strings from "plugin" → "provider"
- [x] T010 [US4] Update `cmd/complyctl/cli/generate.go`: rename all user-visible strings from "plugin" → "provider"
- [x] T011 [US4] Update `cmd/complyctl/cli/doctor.go`: rename all user-visible strings from "plugin" → "provider"
- [x] T012 [US1] Run `go build ./...` from the complyctl root to verify the rename compiles cleanly
- [x] T013 [US1] Run `go test ./...` from the complyctl root to verify all unit tests pass after the rename

**Checkpoint**: complyctl SDK renames complete and tests pass. Tag a release (`vX.Y.Z`) and
publish to GitHub. The tag is required before `complytime-providers` can remove its `replace`
directive.

---

## Phase 2: cmd/test-plugin Rename (complyctl repo)

**Purpose**: Rename the test provider reference implementation (stays in complyctl).

- [x] T014 [US1][US4] Rename directory `cmd/test-plugin/` → `cmd/test-provider/` in complyctl; update any internal imports inside the package
- [x] T015 [US4] Update `Makefile`: rename target `build-test-plugin` → `build-test-provider`; update binary output path accordingly
- [x] T016 [US1] Verify `make build-test-provider` builds successfully

---

## Phase 3: Documentation Cleanup (complyctl repo)

**Purpose**: Remove provider-specific docs from complyctl; update remaining docs to use "provider" terminology.

- [x] T017 [US4] Delete `docs/PLUGIN_GUIDE.md` from complyctl (content migrated to `complytime-providers` in Phase 4)
- [x] T018 [US4] Delete `docs/man/complyctl-openscap-plugin.md` from complyctl (or confirm it doesn't exist at this path); update Makefile variable `MAN_OPENSCAP_PLUGIN` → `MAN_OPENSCAP_PROVIDER` and remove the man page build step for the removed provider
- [x] T019 [P] [US4] Search complyctl for any remaining "plugin" references (excluding `hashicorp/go-plugin` library refs, proto package `complyctl.plugin.v1`, and `api/plugin/`) and update them to "provider"

---

## Phase 4: complytime-providers Repository Setup

**Purpose**: Initialize the providers monorepo with a single root `go.mod` and scaffold both provider directories.

- [x] T020 [US2] Create root `go.mod` in `complytime-providers/` with module `github.com/complytime/complytime-providers`; set Go version to match complyctl root (currently `go 1.25`); add `require github.com/complytime/complyctl vX.Y.Z` (use `replace` directive pointing to local complyctl during development — MUST be removed before merge)
- [x] T021 [US2] Create `Makefile` in `complytime-providers/` with targets: `build` (builds both providers), `build-openscap-provider`, `build-ampel-provider`, `test`, `vendor`, `lint`
- [x] T022 [US2] Create `docs/` directory in `complytime-providers/`; copy and update `docs/PLUGIN_GUIDE.md` from complyctl → `docs/provider-guide.md`; rename all "plugin" terminology to "provider" throughout

---

## Phase 5: OpenSCAP Provider Migration

**Purpose**: Migrate `cmd/openscap-plugin/` from complyctl to `cmd/openscap-provider/` in `complytime-providers`.

- [x] T023 [US2] Copy `cmd/openscap-plugin/` source tree (excluding `vendor/` and `go.mod`) to `cmd/openscap-provider/` in `complytime-providers`
- [x] T024 [US2] Update all `import` paths in `cmd/openscap-provider/` that referenced `github.com/complytime/complyctl/pkg/plugin` → `github.com/complytime/complyctl/pkg/provider`
- [x] T025 [US2] Rename all "plugin" identifiers/strings in `cmd/openscap-provider/` source to "provider" terminology; verify no provider-concept "plugin" references remain
- [x] T026 [US2] Ensure `cmd/openscap-provider/server/server.go` includes the `Export` stub (from PR #463); verify struct implements `provider.Provider` interface
- [x] T027 [US2] Delete `cmd/openscap-plugin/` directory from complyctl (`FR-007`)
- [x] T028 [US1] Run `go build ./cmd/openscap-provider/...` in `complytime-providers` to verify the provider compiles
- [x] T029 [US2][US3] Run unit tests for the openscap provider: `go test ./cmd/openscap-provider/...`

---

## Phase 6: AMPEL Provider Migration

**Purpose**: Migrate `cmd/ampel-plugin/` from complyctl to `cmd/ampel-provider/` in `complytime-providers`.

- [x] T030 [US2] Copy `cmd/ampel-plugin/` source tree to `cmd/ampel-provider/` in `complytime-providers` (no separate `go.mod` exists; it already uses the root module)
- [x] T031 [US2] Update all `import` paths in `cmd/ampel-provider/` that referenced `github.com/complytime/complyctl/pkg/plugin` → `github.com/complytime/complyctl/pkg/provider`
- [x] T032 [US2] Rename all "plugin" identifiers/strings in `cmd/ampel-provider/` source to "provider" terminology
- [x] T033 [US2] Ensure `cmd/ampel-provider/server/server.go` includes the `Export` stub (from PR #463); verify struct implements `provider.Provider` interface
- [x] T034 [US2] Delete `cmd/ampel-plugin/` directory from complyctl (`FR-008`)
- [x] T035 [US1] Run `go build ./cmd/ampel-provider/...` in `complytime-providers` to verify the provider compiles
- [x] T036 [US2][US3] Run unit tests for the ampel provider: `go test ./cmd/ampel-provider/...`

---

## Phase 7: complytime-providers Vendoring & Release Artifacts

**Purpose**: Vendor dependencies in `complytime-providers` and prepare CI for publishing release artifacts.

- [x] T037 [US2] Run `go mod vendor` in `complytime-providers/` to create the `vendor/` tree
- [ ] T038 [US2] Remove `replace` directive from `complytime-providers/go.mod`; update `require github.com/complytime/complyctl` to the tagged release version from Phase 1 checkpoint; re-run `go mod vendor`
- [x] T039 [US2] Create `.github/workflows/ci.yml` in `complytime-providers/` with jobs: build both providers, run tests, and publish release artifacts (`complyctl-provider-openscap` and `complyctl-provider-ampel`) on tag push
- [x] T040 [US1][US3] Verify `make build` in `complytime-providers/` produces both `complyctl-provider-openscap` and `complyctl-provider-ampel` binaries with no local complyctl source tree present

---

## Phase 8: complyctl CI & Makefile Cleanup

**Purpose**: Remove provider build steps from complyctl Makefile and CI; update `ci_compliance.yml`
to download provider binary from `complytime-providers` release artifact.

- [x] T041 [US1] Update complyctl `Makefile`: remove `cd cmd/openscap-plugin` build step from the `build` target; remove `cd cmd/openscap-plugin` vendor step from the `vendor` target; remove `MAN_OPENSCAP_PLUGIN` variable and associated man page build step
- [x] T042 [US1] Verify `make build` in complyctl produces only the `complyctl` CLI binary and completes without errors (`SC-007`)
- [x] T043 [US4] Update `.github/workflows/behavioral_assessment.yml`: rename `build-test-plugin` step → `build-test-provider`
- [x] T044 [US4] Update `.github/workflows/e2e_test.yml`: rename `build-test-plugin` step → `build-test-provider`; update E2E tests to use a provider binary obtained from `complytime-providers` rather than built from complyctl source
- [x] T045 [US1] Update `.github/workflows/ci_compliance.yml`: add step to download `complyctl-provider-ampel` from a published `complytime-providers` GitHub release artifact (note: this step may need to go through `complytime/org-infra` reusable workflow — flag for org-infra maintainers if so)
- [x] T046 [US4] Search `.github/workflows/` for any remaining "plugin" references (excluding library names) and update to "provider"

---

## Phase 9: Acceptance Verification

**Purpose**: Verify all success criteria from spec.md are met before PR submission.

- [x] T047 [US1] Confirm `SC-001`: complyctl repository has zero source files under `cmd/openscap-plugin/` or `cmd/ampel-plugin/`
- [x] T048 [US4] Confirm `SC-002`: full-text search of complyctl for "plugin" (case-insensitive) in Go sources, Makefile, CI workflows, and docs returns zero provider-concept matches (library name `hashicorp/go-plugin` and proto package `complyctl.plugin.v1` are exempt)
- [x] T049 [US2] Confirm `SC-003`: both providers build from `complytime-providers` without a local complyctl source tree
- [x] T050 [US1] Confirm `SC-004`: all complyctl unit tests pass (`go test ./...`)
- [ ] T051 [US3] Confirm `SC-005`: complyctl E2E tests pass using a provider binary built from `complytime-providers`
- [ ] T052 [US3] Confirm `SC-006`: a binary built from `complytime-providers` is discovered and invoked by `complyctl scan` correctly (binary name and discovery paths unchanged)
- [x] T053 [US1] Confirm `SC-007`: `make build` in complyctl produces only the `complyctl` CLI binary
- [ ] T054 [US1] Confirm `SC-008`: complyctl CI pipeline passes without modification to any step that previously built or tested provider source directories

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1** (SDK Rename): No dependencies — start immediately after PR #463 and PR #479 merge
- **Phase 2** (test-plugin rename): Can run in parallel with Phase 1
- **Phase 3** (docs cleanup): Can run in parallel with Phase 1 and 2
- **Phase 4** (complytime-providers setup): Requires Phase 1 to be started (needs `pkg/provider/` to exist for local `replace` dev); can proceed in parallel with Phases 2–3
- **Phase 5** (OpenSCAP migration): Requires Phase 4 complete; can run in parallel with Phase 6
- **Phase 6** (AMPEL migration): Requires Phase 4 complete; can run in parallel with Phase 5
- **Phase 7** (vendoring & release): Requires Phases 5 and 6 complete AND Phase 1 tagged release published
- **Phase 8** (CI & Makefile): Requires Phases 5, 6, and 7 complete (complytime-providers must publish a release before ci_compliance.yml update can be validated)
- **Phase 9** (verification): Requires all prior phases complete

### Within Each Phase

- Tasks marked `[P]` have no intra-phase file conflicts and can run in parallel
- Compile verification tasks (`go build`) depend on all rename tasks within the phase completing first

### SDK Release Gate

The complyctl SDK tagged release (end of Phase 1) is the critical coordination point:
- Required by Phase 7 (T038) before the `replace` directive can be removed
- Must include `pkg/provider/`, `Export` RPC, and `ProviderExecutablePrefix`
- Should be the first tag after both PR #463 and PR #479 merge

---

## Notes

- `[P]` = different files, no intra-task dependencies — safe to parallelize
- `[USn]` = traces task to user story for acceptance verification
- Wire values (`"COMPLYCTL_PLUGIN"`, UUID, protocol version 1) MUST NOT change at any task
- `api/plugin/` directory and `complyctl.plugin.v1` proto package are NOT renamed (FR-019)
- `LoadedPlugin` → `LoadedProvider` rename (T003) is optional per data-model.md but included for consistency; confirm with reviewers before implementing
- Commit after each phase checkpoint; open complyctl PR and complytime-providers PR separately
