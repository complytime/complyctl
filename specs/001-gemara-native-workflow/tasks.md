# Tasks: Gemara-Native Decoupled Workflow

**Branch**: `001-gemara-native-workflow` | **Date**: 2026-02-14 (regenerated 2026-02-26e)
**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Data Model**: [data-model.md](./data-model.md)

## Status Summary

| Phase | Scope | Status |
| :--- | :--- | :--- |
| 1 | Setup — Legacy removal, dependency updates, proto codegen | **COMPLETE** |
| 2 | Foundational — Cache, registry, policy resolution, plugin SDK | **COMPLETE** |
| 3 | US1 — Init, Get, Doctor, List | **COMPLETE** |
| 4 | US2 — Generate, Scan, Execution Plan | **COMPLETE** |
| 5 | US3 — Output Formatters, Scan Summary | **COMPLETE** |
| 6 | Polish — Behavioral tests, spinner, pack data model, PolicyEntry | **COMPLETE** |
| 7 | Documentation — quickstart.md alignment | **COMPLETE** |
| 8 | Doctor Redesign — Version Comparison + Per-Provider Config (R55) | **COMPLETE** |
| 9 | 002-comply-packs — Pack CLI + Doctor Integration | **DEFERRED** |
| 10 | Fix — List table: effective ID, source, status, usage hint | **COMPLETE** |
| 11 | Unit Tests — Critical package coverage (R56) | **COMPLETE** |
| 12 | API Rename — HealthCheck → Describe + ValidateGlobalVars removal | **COMPLETE** |
| 13 | Terminal Output Redesign — Plain default, log relocation (R57) | **COMPLETE** |
| 14 | Output Path Split + Discovery Simplification (R58) | **COMPLETE** |
| 15 | UX Refresh — Lipgloss Default, Scan Table, Execution Plan Collapse (R59) | **COMPLETE** |
| 16 | Bug Fixes — Error Consistency, Get Timeout, Resolver Error Surfacing | **COMPLETE** |
| 17 | Plain Text Only — Lipgloss Removal, Generate Readout, Dry-Run Removal (Session 2026-02-26e) | **COMPLETE** |
| 18 | Init Hardening + OpenSCAP Debug Command (Session 2026-03-02) | **COMPLETE** |

---

## Phase 1: Setup — Legacy Removal + Dependencies (COMPLETE)

**Purpose**: Remove old C2P/OSCAL code paths, update dependencies, establish proto contract.

- [x] T001 Remove `info` command from `cmd/complyctl/cli/` and `root.go` registration
- [x] T002 Remove old `plan` command from `cmd/complyctl/cli/` and `root.go` registration
- [x] T003 Remove C2P framework imports (`compliance-to-policy-go/v2/framework`) from all source files
- [x] T004 Remove `oscal-sdk-go` dependency from `go.mod` and all imports
- [x] T005 Gut `internal/complytime/` — remove C2P-specific logic (`Config`, `Plugins`, `ActionsContextFromPlan`, `LoadFrameworks`, `FindComponentDefinitions`, `LoadProfile`, `LoadCatalogSource`, `WriteAssessmentResults`)
- [x] T006 Remove old OSCAL workflow: `loadPlan`, `actions.AggregateResults`, `actions.Report`, `actions.GeneratePolicy`, `WriteAssessmentResults`, profile/catalog resolution
- [x] T007 Remove `policytype` package from `cmd/openscap-plugin/policytype/`
- [x] T008 Replace `oscalPolicy` references in `cmd/openscap-plugin/` with `[]plugin.AssessmentConfiguration`
- [x] T009 Define proto contract in `specs/001-gemara-native-workflow/contracts/plugin.proto` — `EvaluatorService` with `Generate`, `Scan`, `HealthCheck` RPCs. `ConfidenceLevel` enum (NOT_SET, UNDETERMINED, LOW, MEDIUM, HIGH). `Result` enum (UNSPECIFIED, PASSED, FAILED, SKIPPED, ERROR). `ScanRequest` receives targets only (no `requirement_ids`, R47). `HealthCheckResponse` includes `required_global_variables` and `required_target_variables` (R51)
- [x] T010 Run `buf generate` to produce Go stubs from `plugin.proto`
- [x] T011 Run `buf lint` and `buf breaking` to verify proto quality
- [x] T012 Add Gemara media type constants to `internal/complytime/consts.go` — `MediaTypeCatalog`, `MediaTypeGuidance`, `MediaTypePolicy` (R27)
- [x] T013 Add scan output constants to `internal/complytime/consts.go` — `ScanOutputDir = ".complytime/scan"` (R42), emoji status constants (R43)
- [x] T014 Run `go mod tidy` and `go mod vendor` to clean dependencies
- [x] T015 Run `go build ./...`, `go vet ./...`, `gofmt -l .` — verify clean baseline

**Checkpoint**: Legacy code removed. Proto contract defined. Constants centralized. Clean build.

---

## Phase 2: Foundational — Cache, Registry, Policy, Plugin SDK (COMPLETE)

**Purpose**: Build shared infrastructure that all user stories depend on. No user-facing CLI changes yet.

### 2a: OCI Cache + State Tracking

- [x] T016 [P] Implement `internal/cache/cache.go` — OCI Layout store wrapper using `oras-go/v2/content/oci`. `NewStore(policyID)` creates per-policy OCI Layout directory under `~/.complytime/policies/{id}/`
- [x] T017 [P] Implement `internal/cache/state.go` — `CacheState` struct with `LastSync` and `Policies map[string]PolicyState`. `LoadState()`, `SaveState()` for `~/.complytime/state.json`
- [x] T018 Implement `internal/cache/sync.go` — `SyncPolicy()` using `oras.Copy()` from remote registry to local OCI Layout store. Atomic: failure rolls back to previous state (FR-006)
- [x] T019 Implement `internal/cache/source.go` — `PolicySource` interface + `RegistrySource` implementation
- [x] T020 [P] Implement `internal/cache/cachetest/mock_source.go` — `MockPolicySource` for tests

### 2b: Registry Client + Auth

- [x] T021 [P] Implement `internal/registry/auth.go` — `NewCredentialFunc()` using `oras-credentials-go` `credentials.NewStoreFromDocker()` (R6, R24). Zero custom auth code
- [x] T022 Implement `internal/registry/client.go` — OCI registry client wrapping `oras-go/v2` `remote.Repository`. `PlainHTTP` support for local registries
- [x] T023 [P] Implement `internal/registry/fetcher.go` — `Fetcher` interface for testability. Also implements `internal/registry/resolver.go` — `GetDefinitions`, `DefinitionVersion` resolver methods (FR-002)

### 2c: Policy Resolution

- [x] T024 Implement `internal/policy/loader.go` — OCI Layout → layer extraction by media type matching (`MediaTypeCatalog`, `MediaTypeGuidance`, `MediaTypePolicy`). Returns typed content per layer (R25, R27)
- [x] T025 Implement `internal/policy/resolver.go` — `ResolvePolicyGraph()` builds `DependencyGraph` (controls, guidelines, assessments). `parsePolicyLayer` accepts only `gemara.Policy` with `adherence.assessment-plans` (R39)
- [x] T026 Implement `internal/policy/assessment.go` — `ExtractAssessmentConfigs()`, `GroupByEvaluator()` (per-plan `executor.id`, R32), `ValidateGlobalVars()` (R48)
- [x] T027 Implement `internal/policy/generation_state.go` — `GenerationState`, `SaveState()`, `LoadState()`, `IsFresh(currentDigest)` for digest-based freshness detection (R37)

### 2d: Plugin SDK (gRPC Scanning Interface)

- [x] T028 Implement `pkg/plugin/client.go` — gRPC client wrapper + domain types (`AssessmentConfiguration`, `AssessmentLog`, `ConfidenceLevel` enum). Public SDK for scanning provider authors
- [x] T029 [P] Implement `pkg/plugin/server.go` — gRPC server adapter mapping proto messages to domain types. Public SDK for scanning provider authors
- [x] T030 Implement `pkg/plugin/plugin.go` — `Handshake`, `GRPCEvaluatorPlugin`, `Serve()` using `hashicorp/go-plugin`
- [x] T031 Implement `pkg/plugin/manager.go` — Scanning provider lifecycle: load, health check, route Generate/Scan RPCs by evaluator ID
- [x] T032 Implement `pkg/plugin/discovery.go` — Filesystem discovery of `complyctl-provider-*` executables in `~/.complytime/providers/` (FR-029)
- [x] T033 Implement `pkg/plugin/initialization.go` — `NewClient(path, logger)` simplified (R21, no manifests, no checksums)
- [x] T034 [P] Implement `pkg/plugin/export_test.go` — `RegisterPluginForTest` test helper

### 2e: Workspace + Config

- [x] T035 Implement `internal/complytime/workspace.go` — `NewWorkspace()`, `Exists()`, `Path()`, `EnsureDir()`, `Save()` (R50)
- [x] T036 Implement `internal/complytime/config.go` — `WorkspaceConfig` struct with `Policies []PolicyEntry`, `Variables map[string]string`, `Targets []TargetConfig`. `PolicyEntry` struct (`URL`, `ID`, `EffectiveID()` method). `LoadFrom()`, `SaveTo()`, `Validate()`, `FindPolicy()`, `PolicyIDs()`, `UniqueRegistries()`, `ParsePolicyRef()`, `ValidateTargetPolicyVersions()`, `ResolveEnvVars()` (Session 2026-02-25d)
- [x] T037 [P] Implement `internal/complytime/config_test.go` — unit tests for `PolicyEntry`, `EffectiveID`, `FindPolicy`, `Validate`, `ValidateTargetPolicyVersions`, `UniqueRegistries`, `PolicyIDs`, `ParsePolicyRef`, `ResolveEnvVars`

### 2f: Terminal Helpers

- [x] T038 [P] Implement `internal/terminal/spinner.go` — charmbracelet/bubbles spinner wrapper (Constitution V)
- [x] T039 [P] Implement `internal/terminal/table.go` — reusable charmbracelet table helpers: `Model`, `ShowPlainTable` (R38)

### 2g: Verification

- [x] T040 Run `go build ./...` — all Phase 2 code compiles
- [x] T041 Run `go test ./internal/...` — all unit tests pass
- [x] T042 Run `go vet ./...` and `gofmt -l .` — clean

**Checkpoint**: All shared infrastructure built. Cache, registry, policy resolution, plugin SDK, config, terminal helpers. Ready for CLI commands.

---

## Phase 3: User Story 1 — Initialize, Fetch, Validate (COMPLETE)

**Story**: A system administrator needs to set up complyctl, fetch policies, and validate the environment.

**Goal**: `complyctl init` → `complyctl get` → `complyctl doctor` → `complyctl list` delivers a working policy cache.

**Independent Test**: Run `complyctl init` (creates config), `complyctl get` (fetches policies), `complyctl doctor` (validates), `complyctl list` (shows cached policies).

### 3a: `complyctl init` (FR-003)

- [x] T043 [US1] Implement `cmd/complyctl/cli/init.go` — config-creation-only. Error if `complytime.yaml` exists. Interactive prompts: `promptPolicies()` asks for policy URLs + optional IDs (builds `[]PolicyEntry`), `promptTargets()` shows available effective IDs and collects target policies. Builds `WorkspaceConfig`, calls `workspace.Save()`, prints status to stderr, exits. No `get`, no `doctor` (Session 2026-02-25d)
- [x] T044 [US1] Register `initCmd` in `cmd/complyctl/cli/root.go`

### 3b: `complyctl get` (FR-002, FR-004, FR-005, FR-006)

- [x] T045 [US1] Implement `cmd/complyctl/cli/get.go` — load config, iterate `cfg.Policies` as `[]PolicyEntry`, create per-registry OCI clients via `UniqueRegistries()`. For each policy: `ParsePolicyRef(entry.URL)`, resolve version, `oras.Copy()` to local OCI Layout store. Per-policy progress to stderr (Tier 1 output, FR-035). Atomic sync with rollback on failure (FR-006)
- [x] T046 [US1] Register `getCmd` in `cmd/complyctl/cli/root.go`

### 3c: `complyctl doctor` (FR-039)

- [x] T047 [US1] Implement `internal/doctor/doctor.go` — `Run()` function. Checks: (1) config validation (`LoadFrom` + `Validate` + `ValidateTargetPolicyVersions`), (2) provider discovery + HealthCheck, (3) registry reachability (non-blocking warning), (4) HealthCheck-declared variable validation — global keys against `config.variables`, target keys against relevant `config.targets[].policies` using policy → evaluator → target mapping from cache (R51). Emoji + message output. Exit 0 if all blocking checks pass
- [x] T048 [US1] Implement `cmd/complyctl/cli/doctor.go` — CLI entrypoint. Load config, pass `cfg.Policies` to `UniqueRegistries()` for registry probes. Invoke `doctor.Run()`
- [x] T049 [US1] Register `doctorCmd` in `cmd/complyctl/cli/root.go`

### 3d: `complyctl list` (FR-031)

- [x] T050 [US1] Implement `cmd/complyctl/cli/list.go` — load config, iterate `cfg.Policies` as `[]PolicyEntry`. For each: `ParsePolicyRef(entry.URL)`, check cache status, display `entry.EffectiveID()` + version + cache status. Charmbracelet table + `--plain` flag (R38)
- [x] T051 [US1] Register `listCmd` in `cmd/complyctl/cli/root.go`

### 3e: `complyctl providers` (FR-032)

- [x] T052 [US1] Implement `cmd/complyctl/cli/providers.go` — discover plugins in `~/.complytime/providers/`, display evaluator ID + path + health + version. Charmbracelet table + `--plain` flag (R38)
- [x] T053 [US1] Register `providersCmd` in `cmd/complyctl/cli/root.go`

### 3f: Verification

- [x] T054 Run `go build ./...` — all Phase 3 code compiles
- [x] T055 Run `go test ./...` — all tests pass
- [x] T056 Run `go vet ./...` and `gofmt -l .` — clean

**Checkpoint**: Admin can init workspace, fetch policies, validate environment, list policies and providers. US1 complete.

---

## Phase 4: User Story 2 — Generate + Scan (COMPLETE)

**Story**: A system administrator needs to generate policy artifacts and execute compliance scans.

**Goal**: `complyctl generate` → `complyctl scan` (or just `complyctl scan` with auto-generate) produces EvaluationLog and terminal summary.

**Independent Test**: `complyctl scan --policy-id <ID>` auto-generates and scans. `complyctl generate --policy-id <ID>` followed by `complyctl scan --policy-id <ID>` reuses artifacts. `complyctl scan --policy-id <ID> --dry-run` outputs execution plan without scanning.

### 4a: `complyctl generate` (FR-007, FR-008, FR-009)

- [x] T057 [US2] Implement `cmd/complyctl/cli/generate.go` — load config, `FindPolicy()` returns `*PolicyEntry`, `ParsePolicyRef(entry.URL)` for graph resolution. `ResolvePolicyGraph()` → `ExtractAssessmentConfigs()` → `GroupByEvaluator()` → `ValidateGlobalVars()` → `RouteGenerate()` (global vars + test vars via Generate RPC). Persist `GenerationState`. Output `FormatExecutionPlan()` (charmbracelet tables)
- [x] T058 [US2] Register `generateCmd` in `cmd/complyctl/cli/root.go`

### 4b: `complyctl scan` (FR-024, FR-012, FR-033, FR-034)

- [x] T059 [US2] Implement `cmd/complyctl/cli/scan.go` — load config, `FindPolicy()` returns `*PolicyEntry`, check `GenerationState.IsFresh()`: fresh → reuse, stale → warn + auto-generate, missing → auto-generate. Print brief one-line summary (FR-034). `RouteScan()` with targets only (R47). Build `EvaluationLog` from `AssessmentLog[]`. Write to `{ScanOutputDir}/evaluation-log.yaml`. Display `ScanSummary` (FR-037). `--dry-run` flag: generate + execution plan + exit (FR-033). `--format` flag: oscal/pretty/sarif (Phase 5). `--policy-id` required
- [x] T060 [US2] Register `scanCmd` in `cmd/complyctl/cli/root.go`

### 4c: Execution Plan Output (FR-033, R36)

- [x] T061 [P] [US2] Implement `internal/output/execution_plan.go` — `FormatExecutionPlan()`. Two charmbracelet tables: (1) Evaluator Routing (evaluator ID, requirement count, plugin path, status), (2) Target Scope (target ID, policy ID, evaluator IDs). Unmatched evaluators show ERROR status (R36, R38)

### 4d: EvaluationLog Builder (FR-012)

- [x] T062 [P] [US2] Implement `internal/output/evaluator.go` — builds `*gemara.EvaluationLog` from `[]AssessmentLog`. Maps `AssessmentLog` → `gemara.ControlEvaluation` + `gemara.AssessmentLog`. Result aggregation via go-gemara (R45). YAML output via `goccy/go-yaml`

### 4e: OpenSCAP Provider Updates

- [x] T063 [US2] Update `cmd/openscap-plugin/server/server.go` — implement `Generate` and `Scan` RPCs using `[]plugin.AssessmentConfiguration` (R30). Scan evaluates all requirements from Generate-time state (R47)
- [x] T064 [US2] Update `cmd/openscap-plugin/xccdf/tailoring.go` — accept `[]plugin.AssessmentConfiguration` instead of `oscalPolicy` (R30)
- [x] T065 [US2] Update `cmd/openscap-plugin/xccdf/datastream.go` — use assessment configuration parameters
- [x] T066 [US2] Run `go mod vendor` in `cmd/openscap-plugin/` to sync vendored dependencies

### 4f: Test Plugin

- [x] T067 [P] [US2] Implement `cmd/test-plugin/` — E2E test scanning provider binary. Implements Generate, Scan, HealthCheck RPCs. Returns deterministic AssessmentLog entries for test verification. NOT referenced by production code

### 4g: Verification

- [x] T068 Run `go build ./...` — all Phase 4 code compiles
- [x] T069 Run `go test ./...` — all tests pass
- [x] T070 Run `go vet ./...` and `gofmt -l .` — clean

**Checkpoint**: Admin can generate artifacts and scan. Auto-generate, reuse, stale-detect all work. Dry-run outputs execution plan. US2 complete.

---

## Phase 5: User Story 3 — Output Formats + Scan Summary (COMPLETE)

**Story**: A system administrator needs scan results in multiple formats and a clear terminal summary.

**Goal**: `complyctl scan --format <oscal|pretty|sarif>` produces formatted reports. Terminal always shows ActionError-style summary.

**Independent Test**: `complyctl scan --policy-id <ID> --format oscal` produces OSCAL JSON. `--format pretty` produces Markdown + EvaluationLog. `--format sarif` produces SARIF JSON. Default (no `--format`) produces EvaluationLog only. Terminal always shows emoji + message summary + totals table.

### 5a: Output Formatters (FR-014, FR-025, FR-026, FR-027)

- [x] T071 [P] [US3] Implement `internal/output/oscal.go` — OSCAL export using `go-oscal` types (`AssessmentResults`, `Finding`, `Observation`). Maps `EvaluationLog` → OSCAL (FR-014)
- [x] T072 [P] [US3] Implement `internal/output/markdown.go` — Markdown report. Optionally embeds EvaluationLog when `--format pretty` (FR-025, FR-027)
- [x] T073 [P] [US3] Implement `internal/output/sarif.go` — SARIF export using `go-gemara/gemaraconv` SARIF conversion (FR-026)

### 5b: Scan Summary (FR-037, R45)

- [x] T074 [US3] Implement `internal/output/scan_summary.go` — `FormatScanSummary()`. Non-passing results: emoji + message per failure/error/skip (no requirement ID). Sort by severity: failed → error → skipped. Single-row charmbracelet totals table. Result aggregation via go-gemara. Message from `Steps[].Message` (first match)

### 5c: Wire Formatters into Scan

- [x] T075 [US3] Update `cmd/complyctl/cli/scan.go` — wire `--format` flag dispatch: `oscal` → `oscal.go`, `pretty` → `markdown.go`, `sarif` → `sarif.go`. Default: EvaluationLog only (FR-028). Always display `ScanSummary` in terminal (FR-037)

### 5d: Verification

- [x] T076 Run `go build ./...` — all Phase 5 code compiles
- [x] T077 Run `go test ./...` — all tests pass (including output formatter tests)
- [x] T078 Run `go vet ./...` and `gofmt -l .` — clean

**Checkpoint**: All output formats work. Scan summary displays after every scan. US3 complete.

---

## Phase 6: Polish — Behavioral Tests, Pack Data Model, PolicyEntry (COMPLETE)

**Purpose**: Cross-cutting concerns, governance tests, pack data model for 002, and final config refactoring.

### 6a: Behavioral Assessment Tests

- [x] T079 [P] Implement `tests/behavioral/reusable_steps.go` — shared test infrastructure for behavioral assessment. Config YAML uses `PolicyEntry` format
- [x] T080 [P] Implement `tests/behavioral/transport_security.go` — TLS transport security assessment
- [x] T081 [P] Implement `tests/behavioral/log_security.go` — log credential redaction assessment
- [x] T082 [P] Implement `tests/behavioral/credential_protection.go` — credential protection assessment
- [x] T083 Implement `.github/workflows/behavioral_assessment.yml` — CI workflow for behavioral tests

### 6b: Pack Data Model (types only, CLI deferred to 002)

- [x] T084 [P] Implement `internal/complytime/pack.go` — `PackManifest`, `PlatformConfig`, `PackPolicyEntry`, `PackProviderEntry`, `SystemDependency` structs. `LoadPackManifest()`, `ValidatePackManifest()`, `PackManifestExists()`, `PackPolicyIDs()` (R53)
- [x] T085 [P] Implement `internal/complytime/pack_test.go` — pack manifest validation, loading, YAML parsing, duplicate detection
- [x] T086 Add `PackManifestFile` constant to `internal/complytime/consts.go` (R53)

### 6c: E2E + Integration Test Updates

- [x] T087 Update `tests/e2e/e2e_test.go` — embedded YAML configs use `PolicyEntry` format (url + id, not registry + policy_ids)
- [x] T088 Update `tests/e2e/helpers_test.go` — `writeWorkspaceConfig()` generates `PolicyEntry` format
- [x] T089 Update `tests/integration_test.sh` — complytime.yaml content uses `PolicyEntry` format
- [x] T090 Update `cmd/mock-oci-registry/testdata/sample-complytime.yaml` — `PolicyEntry` format

### 6d: Version Command

- [x] T091 Update `cmd/complyctl/cli/version.go` — clean version output, no log file created (FR-035)

### 6e: CLI Options + Root

- [x] T092 Implement `cmd/complyctl/cli/options.go` — shared CLI option types
- [x] T093 Update `cmd/complyctl/cli/root.go` — register all commands (init, get, list, providers, generate, scan, doctor, version). No `pack` command (deferred to 002)

### 6f: Verification

- [x] T094 Run `go build ./...` — full build green
- [x] T095 Run `go test ./...` — all tests pass
- [x] T096 Run `go vet ./...` and `gofmt -l .` — clean
- [x] T097 Verify no stale references — search for `Pack ` (deliberate space), `RegistryConfig`, `policy_ids` in Go source (except `pack.go` data model); zero matches expected

**Checkpoint**: Behavioral tests, pack data model, PolicyEntry propagated. Full build green. All 001 implementation complete.

---

## Phase 7: Documentation Alignment (COMPLETE)

**Purpose**: Align documentation artifacts with the implemented `PolicyEntry` model. quickstart.md still references the superseded dual-mode config (`pack` field, `policy_ids`).

### 7a: quickstart.md Update

- [x] T098 Update `specs/001-gemara-native-workflow/quickstart.md` Section 1 — replace pack-mode `init` example with `PolicyEntry` prompts (policy URLs + optional IDs). Replace YAML showing `pack:` field with `policies:` list of `url:` + `id:` entries. Replace `policy_ids:` with `policies:` in targets
- [x] T099 Update `specs/001-gemara-native-workflow/quickstart.md` Section 1b — replace standalone-mode YAML showing `registry:` + `policies:` (id/version format) with `PolicyEntry` format. Replace `policy_ids:` with `policies:` in targets
- [x] T100 Update `specs/001-gemara-native-workflow/quickstart.md` — remove all references to dual-mode config, pack-mode init, and `complyctl pack init`. Add note that pack CLI is deferred to 002
- [x] T101 Review remaining sections of quickstart.md (workflow steps 2-5) for stale `policy_ids` or `registry` references and update to match current implementation

### 7b: Verification

- [x] T102 Grep quickstart.md for `policy_ids`, `pack:`, `registry:` — zero matches expected (except historical context or 002 forward-references)

**Checkpoint**: All documentation aligned with `PolicyEntry` implementation.

---

## Phase 8: Doctor Redesign — Version Comparison + Per-Provider Config (COMPLETE)

**Purpose**: Replace doctor's registry reachability probe with per-policy version comparison. Add per-provider configuration summary with `--verbose` drill-down. Implements R55 (Session 2026-02-25e).

**Dependencies**: All 001 phases complete (Phases 1-7). Uses existing `DefinitionVersion()` from `internal/registry/resolver.go`, `PolicyState` from `internal/cache/state.go`, and `ProviderHealth` from `internal/doctor/doctor.go`.

**Independent Test**: `complyctl doctor` shows per-policy version status (latest vs stale) and per-provider config summary (resolved/missing counts). `complyctl doctor --verbose` expands provider variable detail to per-key status. Unreachable registries produce per-registry warning, skip version check for those policies.

### 8a: Version Comparison Check (replaces CheckRegistries)

- [x] T103 [US1] Implement `CheckPolicyVersions()` in `internal/doctor/doctor.go` — accepts `*WorkspaceConfig`, `cacheDir string`, registry resolver. For each policy in config: load `PolicyState` from `state.json` (cached version/digest), group policies by registry via `UniqueRegistries()`. Per registry: attempt `DefinitionVersion()` query for latest version. If unreachable → emit non-blocking warning per registry (e.g., `⚠️ registry/X: unreachable — version check skipped`), skip all policies from that registry. Per reachable policy: compare cached version against remote latest — stale → warning with both versions + remediation (`run complyctl get`), up-to-date → pass. Returns `[]CheckResult`
- [x] T104 [US1] Update `Run()` in `internal/doctor/doctor.go` — replace `CheckRegistries(registries)` call with `CheckPolicyVersions(cfg, cacheDir, versionResolver)`. Add `VersionResolver` interface parameter (satisfied by `internal/registry/resolver.go`) for testability. Keep `CheckCache()` before `CheckPolicyVersions()` (version check needs cache to exist)
- [x] T105 [P] [US1] Implement `VersionResolver` interface in `internal/doctor/doctor.go` — `ResolveLatestVersion(registry, repository string) (string, error)`. Wraps `internal/registry/resolver.go` `DefinitionVersion()`. Interface enables mock injection for tests

### 8b: Per-Provider Configuration Summary (replaces failures-only output)

- [x] T106 [US1] Refactor `CheckVariables()` in `internal/doctor/doctor.go` — add `verbose bool` parameter. Default mode: per-provider summary line with resolved count + missing count (e.g., `✅ provider/openscap: 3/3 global vars, 2/2 target vars` or `❌ provider/kube-eval: 1/2 global vars — missing workspace`). Verbose mode: append per-key status lines below each provider summary (e.g., `global: workspace ✅, output_dir ✅`). Keep existing global + target variable validation logic (R51). Returns `[]CheckResult` — one per provider in default mode, additional detail results in verbose mode
- [x] T107 [US1] Update `Run()` signature in `internal/doctor/doctor.go` — add `verbose bool` parameter. Pass through to `CheckVariables()`. All other checks unaffected by verbose flag

### 8c: `--verbose` CLI Flag

- [x] T108 [US1] Add `--verbose` bool flag to `doctorCmd` in `cmd/complyctl/cli/doctor.go` — cobra `BoolVar`. Pass to `doctor.Run()`. Short flag: `-v`
- [x] T109 [US1] Update `runDoctor()` in `cmd/complyctl/cli/doctor.go` — pass verbose flag to `doctor.Run()`. Pass version resolver (registry client wrapper). Load cache state for version comparison

### 8d: Registry Resolver Adapter

- [x] T110 [P] [US1] Implement version resolver adapter in `cmd/complyctl/cli/doctor.go` or `internal/doctor/` — wraps `internal/registry/client.go` to satisfy `VersionResolver` interface. Creates per-registry clients dynamically (same pattern as `get` command). Handles auth via `NewCredentialFunc()`

### 8e: Tests

- [x] T111 [P] [US1] Add unit tests for `CheckPolicyVersions()` in `internal/doctor/doctor_test.go` — test cases: (1) all policies at latest → all pass, (2) one stale policy → one warning with version info, (3) unreachable registry → per-registry warning + no staleness lines for its policies, (4) mixed registries — one reachable + one unreachable, (5) empty policies list → no checks
- [x] T112 [P] [US1] Add unit tests for refactored `CheckVariables()` in `internal/doctor/doctor_test.go` — test cases: (1) default mode → per-provider summary counts, (2) verbose mode → per-key status lines, (3) all vars present → pass with counts, (4) missing global var → fail with count + missing name, (5) missing target var → fail with count + missing name + target ID
- [x] T113 [P] [US1] Add integration test for `complyctl doctor --verbose` in `tests/e2e/` or `tests/integration_test.sh` — verify verbose output contains per-key lines, default output shows counts only

### 8f: Verification

- [x] T114 Run `go build ./...` — all Phase 8 code compiles
- [x] T115 Run `go test ./internal/doctor/...` — all doctor tests pass
- [x] T116 Run `go test ./...` — full test suite passes
- [x] T117 Run `go vet ./...` and `gofmt -l .` — clean
- [x] T118 Verify `CheckRegistries()` is no longer called — search `internal/doctor/` for `CheckRegistries`; zero matches expected (function removed or renamed)

**Checkpoint**: Doctor shows per-policy version staleness, per-provider config summary with counts, `--verbose` for key detail. Unreachable registries handled gracefully. R55 complete.

---

## Phase 9: 002-comply-packs — Pack CLI + Doctor Integration (DEFERRED)

**Purpose**: Implement pack CLI subcommands, doctor dual-file validation, pack build/push/pull, and provider directory override. Deferred to `002-comply-packs` branch. Pack manifest types available from Phase 6b.

**Dependencies**: All 001 phases complete (Phases 1-8).

### 9a: Doctor Dual-File Mode

- [ ] T119 Update `internal/doctor/doctor.go` `Run()` — detect `complypack.yaml` presence using `PackManifestExists()`; if found, load and validate pack manifest (schema, provider binaries, cache digests); if `complytime.yaml` absent but `complypack.yaml` present, report pack OK + config missing with remediation guidance (R53)
- [ ] T120 Add `CheckPackManifest()` to `internal/doctor/doctor.go` — validate manifest schema, verify provider binaries exist in `./bin/`, verify cached policy OCI layouts exist in `./policies/`
- [ ] T121 Add `CheckSystemDeps()` to `internal/doctor/doctor.go` — run each `system-dependencies[].check` command, report pass/fail per dependency
- [ ] T122 Update `cmd/complyctl/cli/doctor.go` — pass `complypack.yaml` path to `doctor.Run()` when present

### 9b: Pack Build

- [ ] T123 Implement `internal/pack/build.go` — `Build()` function: fetch policies via `get` logic, copy provider binaries to `bin/`, generate `complytime.yaml.example`, assemble tarball
- [ ] T124 Add `GenerateExampleConfig()` — create `complytime.yaml.example` from pack manifest policies + empty targets

### 9c: Pack CLI Subcommands

- [ ] T125 Create `cmd/complyctl/cli/pack.go` — `packCmd` command group (`Use: "pack"`, `Short: "Manage comply-packs"`)
- [ ] T126 Add `pack init` subcommand — prompts for pack id, version, registry URL, policy IDs, provider IDs. Creates `complypack.yaml`
- [ ] T127 Add `pack doctor` subcommand — validates manifest for buildability (registry reachable, policies exist, profiles valid)
- [ ] T128 Add `pack build` subcommand — invokes `internal/pack/build.go`
- [ ] T129 Add `pack push` subcommand — pushes tarball as OCI artifact to registry
- [ ] T130 Add `pack pull` subcommand — retrieves pack tarball from registry
- [ ] T131 Register `packCmd` in `cmd/complyctl/cli/root.go`

### 9d: Config Validation — Pack Context

- [ ] T132 Update doctor `CheckConfig` — if in pack context, validate that target policy references exist in pack's policy list
- [ ] T133 Update `Validate()` — optional `registry.url` when in pack context (policies pre-cached)

### 9e: Provider Directory Override

- [ ] T134 Add `COMPLYTIME_PROVIDER_DIR` env var support to `pkg/plugin/discovery.go` — overrides default `~/.complytime/providers/`

### 9f: Verification

- [ ] T135 Run `go build ./...` — all Phase 9 code compiles
- [ ] T136 Run `go test ./...` — all tests pass
- [ ] T137 Run `go vet ./...` and `gofmt -l .` — clean
- [ ] T138 E2E test: Fedora comply-pack workflow (build → push → pull → doctor → scan)
- [ ] T139 Verify no field overlap between `PackManifest` and `WorkspaceConfig`

**Checkpoint**: Pack CLI complete. Doctor validates both files. Build/push/pull works. Provider dir override works.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies. Foundation for everything
- **Phase 2 (Foundational)**: Depends on Phase 1. All shared infrastructure
- **Phase 3 (US1)**: Depends on Phase 2. Init/get/doctor/list/providers CLI commands
- **Phase 4 (US2)**: Depends on Phase 2. Can run in parallel with Phase 3 (different CLI files). Generate/scan depend on cache infrastructure from Phase 2
- **Phase 5 (US3)**: Depends on Phase 4 (output formatters wire into scan). Can run 5a in parallel with Phase 4 (different files)
- **Phase 6 (Polish)**: Depends on Phases 3-5. Cross-cutting cleanup
- **Phase 7 (Docs)**: Depends on Phase 6. Documentation alignment only
- **Phase 8 (Doctor Redesign)**: Depends on Phases 1-7. Modifies `internal/doctor/doctor.go` and `cmd/complyctl/cli/doctor.go`. Uses existing `internal/registry/resolver.go` and `internal/cache/state.go`
- **Phase 9 (002-comply-packs)**: Depends on all 001 phases (1-8). Separate branch

### Parallel Opportunities

**Within Phase 2**: T016+T017+T020 parallel (different files). T021+T023 parallel. T028+T029+T034 parallel. T035+T038+T039 parallel.

**Phase 3 vs Phase 4**: T043 (init.go) parallel with T057 (generate.go) — different CLI files. T047 (doctor.go) parallel with T059 (scan.go). T050 (list.go) parallel with T061 (execution_plan.go).

**Within Phase 5**: T071+T072+T073 fully parallel (oscal.go, markdown.go, sarif.go — independent formatters).

**Within Phase 6**: T079-T082 fully parallel (different behavioral test files). T084+T085+T086 parallel with T087-T090 (pack model vs test configs — different files).

**Within Phase 8**: T105 (VersionResolver interface) parallel with T106 (CheckVariables refactor) — different functions. T110 (resolver adapter) parallel with T111+T112 (unit tests) after T103-T107 complete. T111+T112+T113 fully parallel (different test files/functions).

---

## Implementation Strategy

### MVP (Phases 1-3) — COMPLETE

Setup + foundational infrastructure + US1 (init, get, doctor, list, providers). Delivers a working policy cache that can be fetched and validated.

### Core Value (Phase 4) — COMPLETE

US2 (generate, scan). Delivers the primary compliance scanning capability.

### Full Feature (Phase 5) — COMPLETE

US3 (output formatters, scan summary). Delivers multi-format output and terminal UX.

### Polish (Phase 6) — COMPLETE

Behavioral tests, pack data model, PolicyEntry refactoring.

### Documentation (Phase 7) — COMPLETE

quickstart.md aligned with PolicyEntry model.

### Doctor Redesign (Phase 8) — COMPLETE

Per-policy version comparison, per-provider config summary, `--verbose` flag. R55.

### Comply-Packs (Phase 9) — DEFERRED to 002

Full pack CLI lifecycle. Separate feature branch.

### List Table Fix (Phase 10) — COMPLETE

Fix effective ID/source/status display disconnect. 5 tasks (T140-T144).

### Unit Test Coverage (Phase 11) — COMPLETE

Critical package unit tests per R56. 16 tasks (T145-T160).

### API Rename (Phase 12) — COMPLETE

HealthCheck → Describe + ValidateGlobalVars removal. 17 tasks (T161-T177).

### Terminal Output Redesign (Phase 13) — COMPLETE

Plain default, log relocation, scan totals, list simplification. R57. 19 tasks (T178-T196).

### Output Path Split (Phase 14) — COMPLETE

Remove `--pretty` from discovery commands. Formatted reports to CWD. R58. 9 tasks (T197-T205).

### UX Refresh (Phase 15) — COMPLETE

Lipgloss universal default, scan results 4-column table, execution plan collapsed to single table, TTY auto-detection. R59. 20 tasks (T206-T225).

### Bug Fixes (Phase 16) — COMPLETE

Error message consistency ("complytime" → "complyctl"/file name), `get --timeout`, resolver error surfacing. 16 tasks (T226-T241).

### Comply-Packs (Phase 9) — DEFERRED to 002 (detail)

Full pack CLI lifecycle. Separate feature branch.

---

## Phase 10: Fix — List Table Effective ID + Source + Status (COMPLETE)

**Purpose**: `complyctl list` currently displays OCI repository paths from the cache (e.g., `policies/nist-800-53-r5`), but `generate` and `scan` require the effective policy ID (e.g., `nist-800-53-r5`) via `--policy-id`. Admins cannot copy-paste from `list` output into `scan --policy-id`. Fix `list` to show effective IDs, source URLs, cache status, and a usage hint.

**Bug analysis**: `loader.ListCachedPolicies()` returns `map[string][]string` keyed by OCI repo path. `list.go` displays these keys directly. `scan.go` and `generate.go` use `FindPolicy(cfg.Policies, policyID)` which matches by `EffectiveID()` — the last URL path segment or explicit `id` field. The disconnect: cache keys are repo paths (e.g., `policies/nist-800-53-r5`), effective IDs are shortnames (e.g., `nist-800-53-r5`).

**Checkpoint**: `complyctl list` displays effective IDs that can be passed directly to `complyctl scan --policy-id <ID>` and `complyctl generate --policy-id <ID>`. Table includes source URL, cache status, and footer message.

- [x] T140 [US3] Refactor `run()` in `cmd/complyctl/cli/list.go` to build rows from `cfg.Policies` as the primary data source instead of raw cache keys — iterate `PolicyEntry` objects, resolve each to cache state via `loader.GetCachedVersions(ref.Repository)`, produce rows keyed by `EffectiveID()`
- [x] T141 [US3] Update `printGemaraPolicyTable()` in `cmd/complyctl/cli/list.go` to accept 4-column rows: Effective ID, Source (registry/repository from `ParsePolicyRef`), Versions, Status (cached version string or `StatusError` + "not cached"). Add column definitions: `Effective ID`, `Source`, `Versions`, `Status`
- [x] T142 [US3] Add footer message after the table in `cmd/complyctl/cli/list.go` — print `fmt.Fprintf(w, "\nUse the Effective ID with: complyctl generate --policy-id <ID> | complyctl scan --policy-id <ID>\n")` after table output in both styled and `--plain` modes
- [x] T143 [US3] Update `--policy-id` filter logic in `list.go` `run()` to filter by effective ID instead of raw cache key — match `o.policyID` against `PolicyEntry.EffectiveID()` before building the row set
- [x] T144 [US3] Update `cmd/complyctl/cli/list.go` not-cached status — when a policy from config has no cached versions, show its effective ID with status `⚠️ not cached — run complyctl get` instead of the current `"(not cached — " + EffectiveID + ")"` version string hack

---

## Phase 11: Unit Tests — Critical Package Coverage, R56 (COMPLETE)

**Purpose**: Add unit tests for `internal/policy/` (dependency resolution, policy parsing, generation state), `pkg/plugin/discovery.go` (plugin discovery), per Session 2026-02-26 clarifications. All exported functions get positive + negative cases. See R56 in research.md.

**Checkpoint**: `go test ./internal/policy/... ./pkg/plugin/...` passes with all new test files. Each test file covers the exported functions of its corresponding source file.

### Resolver Tests (PolicyLoader interface + resolver_test.go)

- [x] T145 [P] Extract `PolicyLoader` interface from `Loader` in `internal/policy/resolver.go` — methods: `LoadLayerByMediaType(policyID, version, mediaType string) ([]byte, error)`, `PolicyExists(policyID, version string) bool`, `ResolveVersion(policyID, configVersion string) (string, error)`. Update `Resolver` struct to accept the interface. Production `Loader` satisfies it without code changes
- [x] T146 Create `internal/policy/resolver_test.go` — mock `PolicyLoader` implementation. Test `ResolvePolicyGraph`: empty policy ID returns error, empty version returns error, policy not in cache returns error, valid policy with all 3 layers returns populated `DependencyGraph`, policy with missing optional layers (catalog, guidance) returns partial graph without error
- [x] T147 Add `parsePolicyLayer` tests in `internal/policy/resolver_test.go` — invalid YAML returns error, valid YAML missing `adherence.assessment-plans` returns error, valid synthetic Gemara Policy YAML with one assessment plan returns correct `policyLayerResult`, valid YAML with multiple assessment plans and distinct `executor.id` per plan returns multi-evaluator result
- [x] T148 Add `extractFromGemaraPolicy` tests in `internal/policy/resolver_test.go` — single evaluator across all plans sets `EvaluatorID` on result, mixed evaluators across plans leaves `EvaluatorID` empty with per-assessment IDs set, policy with `implementation-plan` timeline fields populates `PolicyTimeline`

### Assessment Tests

- [x] T149 [P] Create `internal/policy/assessment_test.go` — test `ExtractAssessmentConfigs`: empty graph returns empty slice, graph with 3 assessments returns 3 configs with correct `PlanID`/`RequirementID`/`EvaluatorID` mapping
- [x] T150 Add `GroupByEvaluator` tests in `internal/policy/assessment_test.go` — single-evaluator shortcut (graph `EvaluatorID` set) returns one group with all configs, multi-evaluator (graph `EvaluatorID` empty, per-config IDs differ) returns N groups, empty configs returns empty map
- [x] T151 Add `ValidateGlobalVars` tests in `internal/policy/assessment_test.go` — non-empty global vars returns nil, empty global vars with named evaluator returns error naming the evaluator, empty global vars with only broadcast (empty evaluator ID) group returns nil

### Generation State Tests

- [x] T152 [P] Create `internal/policy/generation_state_test.go` — test `SaveGenerationState` + `LoadGenerationState` round-trip: save to temp dir, load back, verify all fields match. Test `LoadGenerationState` with missing file returns nil (no error). Test `LoadGenerationState` with corrupt JSON returns error
- [x] T153 Add `IsFresh` tests in `internal/policy/generation_state_test.go` — matching digest returns true, mismatched digest returns false, empty digest returns false
- [x] T154 Add `NewGenerationState` test in `internal/policy/generation_state_test.go` — verify fields populated, `GeneratedAt` is valid RFC3339, evaluator IDs preserved

### Loader Tests

- [x] T155 [P] Create `internal/policy/loader_test.go` — test `PolicyExists` with nonexistent policy returns false. Test `ResolveVersion` with empty cache returns error with "not in cache" message. Test `GetCachedVersions` with nonexistent policy returns empty slice
- [x] T156 Add `LoadLayerByMediaType` tests in `internal/policy/loader_test.go` — empty policy ID returns error, empty version returns error, empty media type returns error, nonexistent policy returns error

### Discovery Tests

- [x] T157 [P] Create `pkg/plugin/discovery_test.go` — test `DiscoverPlugins` with empty temp dir returns empty slice. Test with temp dir containing non-prefixed executables returns empty slice. Test with temp dir containing `complyctl-provider-mock` (chmod +x) returns one `PluginInfo` with `EvaluatorID: "mock"` and correct path
- [x] T158 Add `scanDir` tests in `pkg/plugin/discovery_test.go` — nonexistent directory returns nil (no error). Test non-executable file with correct prefix is skipped. Test directory entries are skipped. Test multiple valid executables returns all with correct evaluator IDs
- [x] T159 Add `expandPath` tests in `pkg/plugin/discovery_test.go` — `~/foo` expands to `$HOME/foo`. `/absolute/path` unchanged. `relative/path` unchanged
- [x] T160 Add user-dir precedence test in `pkg/plugin/discovery_test.go` — when same evaluator ID exists in both user dir and system dir, only user-dir entry appears in results (requires overriding `SystemProviderDir` or testing via `scanDir` directly)

---

## Phase 12: API Rename — HealthCheck → Describe + ValidateGlobalVars Removal (COMPLETE)

**Purpose**: Rename the `HealthCheck` RPC to `Describe` for clearer semantics (the endpoint reports capabilities, not just health). Remove the incorrect `ValidateGlobalVars` blanket check that caused `scan`/`generate` to fail when global variables were empty even though the plugin did not require them. Session 2026-02-26b.

**Bug**: `complyctl doctor` passes (validates against plugin-declared `RequiredGlobalVariables`, which openscap declares as empty) but `scan`/`generate` fails with "evaluator requires global variables" (hardcoded check in `ValidateGlobalVars` fails whenever a named evaluator exists and global variables section is empty, regardless of plugin requirements).

**Checkpoint**: `complyctl scan` succeeds when the plugin does not declare global variables. `Describe` is the canonical RPC name across proto, SDK, CLI, tests, and vendored copies. `ValidateGlobalVars` function and calls removed.

### 12a: ValidateGlobalVars Removal

- [x] T161 Remove `ValidateGlobalVars` function from `internal/policy/assessment.go` and unused `fmt` import
- [x] T162 Remove `policy.ValidateGlobalVars` calls from `cmd/complyctl/cli/scan.go` and `cmd/complyctl/cli/generate.go`
- [x] T163 Remove `ValidateGlobalVars` tests from `internal/policy/assessment_test.go` (`TestValidateGlobalVars_NonEmptyVars`, `TestValidateGlobalVars_EmptyVarsNamedEvaluator`, `TestValidateGlobalVars_BroadcastOnlyNoError`)

### 12b: HealthCheck → Describe Rename

- [x] T164 Rename `HealthCheck` RPC → `Describe` in `api/plugin/plugin.proto` (messages: `DescribeRequest`, `DescribeResponse`)
- [x] T165 [P] Rename in `specs/001-gemara-native-workflow/contracts/plugin.proto` — match canonical proto
- [x] T166 Run `buf generate` — regenerate `api/plugin/plugin.pb.go` and `api/plugin/plugin_grpc.pb.go`
- [x] T167 Update `pkg/plugin/client.go` — rename method and types to Describe
- [x] T168 [P] Update `pkg/plugin/server.go` — rename gRPC adapter method
- [x] T169 Update `pkg/plugin/manager.go` — `Plugin` interface: `Describe` replaces `HealthCheck`. `LoadPlugins` call site updated
- [x] T170 Update `internal/doctor/doctor.go` — rename `lp.Client.HealthCheck` → `lp.Client.Describe`, variable names (`hcErr` → `descErr`), comments
- [x] T171 [P] Update `cmd/complyctl/cli/providers.go` — rename `lp.Client.HealthCheck` → `lp.Client.Describe`
- [x] T172 Update `cmd/openscap-plugin/server/server.go` — rename `HealthCheck` method to `Describe`
- [x] T173 [P] Update `cmd/test-plugin/main.go` — rename method to `Describe`
- [x] T174 Update tests: `pkg/plugin/mock_client_test.go`, `pkg/plugin/client_test.go` (method names), `tests/e2e/e2e_test.go` (`TestE2E_MockPluginHealthCheck` → `TestE2E_MockPluginDescribe`)
- [x] T175 Copy updated source files to `cmd/openscap-plugin/vendor/github.com/complytime/complyctl/` — `pkg/plugin/`, `api/plugin/` vendored copies
- [x] T176 Update error message in `cmd/complyctl/cli/scan.go` — `"HealthCheck may have failed"` → `"Describe may have failed"`

### 12c: Verification

- [x] T177 Run `go build ./...`, `go test ./...`, `go vet ./...` — verify clean build and all tests pass

---

## Phase 13: Terminal Output Redesign — Plain Default + Log Relocation, R57 (COMPLETE)

**Purpose**: Make plain aligned text (podman-style) the default for all tabular CLI outputs. Add `--pretty` flag for lipgloss-rendered tables. Move log file to `.complytime/complyctl.log`. Compact scan summary totals to inline format. Simplify `list` to two columns. Remove `charmbracelet/bubbles/table` from vendor. Implements R57 (Session 2026-02-26b).

**Dependencies**: All 001 phases complete (Phases 1-12). Uses existing `ShowPlainTable` and `RenderTable` from `internal/terminal/table.go`. No new dependencies.

**Independent Test**: `complyctl list` shows two-column plain table. `complyctl providers` defaults to plain, `--pretty` renders with lipgloss borders. `complyctl scan` totals line is compact inline. Log file created at `.complytime/complyctl.log`. `--plain` flag no longer accepted.

### 13a: Constants + Log Relocation

- [X] T178 Add `LogFileName = "complyctl.log"` constant to `internal/complytime/consts.go` (FR-038, Constitution I)
- [X] T179 Update `cmd/complyctl/cli/root.go` — replace hardcoded `logFileName` with path constructed from `complytime.WorkspaceDir` + `complytime.LogFileName`. Ensure `.complytime/` directory is created before opening the log file in `lazyLogWriter.Write`. Remove the local `logFileName` const

### 13b: `complyctl list` — Two Columns (FR-031, R57)

- [X] T180 Update `cmd/complyctl/cli/list.go` — reduce to two columns: `POLICY ID` + `VERSION`. Remove `source` and `status` columns. Version column shows resolved version from cache (or `-` if not cached). Replace `--plain` flag with `--pretty` flag. Make `ShowPlainTable` the default renderer, `RenderTable` when `--pretty`. Remove `--plain` from Example string. Update `printGemaraPolicyTable` signature and headers

### 13c: `complyctl providers` — Pretty Flag (FR-032, R57)

- [X] T181 Update `cmd/complyctl/cli/providers.go` — replace `--plain` bool with `--pretty` bool. Invert rendering logic: `ShowPlainTable` is default, `RenderTable` when `--pretty`. Update flag definition (`BoolVarP` name and help text)

### 13d: Execution Plan — Pretty Flag (FR-033, R57)

- [X] T182 Update `internal/output/execution_plan.go` — add `pretty bool` parameter to `FormatExecutionPlan`. Use `terminal.ShowPlainTable` (writing to `strings.Builder`) for default. Use `terminal.RenderTable` when `pretty=true`. Update `SectionLabel` usage — bold labels for `--pretty`, plain text labels for default
- [X] T183 [P] Update `cmd/complyctl/cli/generate.go` — add `--pretty` flag to `generateOptions`. Pass to `output.FormatExecutionPlan`
- [X] T184 [P] Update `cmd/complyctl/cli/scan.go` — add `--pretty` flag to `scanOptions`. Pass to `output.FormatExecutionPlan` for `--dry-run` path

### 13e: Scan Summary — Compact Inline Totals (FR-037, R57)

- [X] T185 Update `internal/output/scan_summary.go` — replace `terminal.RenderTable` totals table with compact inline `fmt.Fprintf`: `"%d ✅  %d ❌  %d ⏭️  %d ⚠️\n"`. Remove `terminal` import (no longer needed). Non-passing lines remain as-is (emoji + message per line)

### 13f: Dependency Cleanup

- [X] T186 [P] Run `go mod tidy` and `go mod vendor` — clean unused `charmbracelet/bubbles` sub-packages (`help`, `key`, `table`, `viewport`) from vendor. Verify `bubbles/spinner` remains (used by `internal/terminal/spinner.go`)
- [X] T187 [P] Update `vendor/modules.txt` — verify `charmbracelet/bubbles` sub-packages reflect only `spinner` (no `table`, `help`, `key`, `viewport`)

### 13g: Documentation Alignment

- [X] T188 Update `specs/001-gemara-native-workflow/quickstart.md` Section "Troubleshooting" — change log path from `complyctl.log` to `.complytime/complyctl.log`
- [X] T189 [P] Update `specs/001-gemara-native-workflow/quickstart.md` Section 8 scan output — replace charmbracelet totals table with compact inline: `44 ✅  2 ❌  1 ⏭️  1 ⚠️`
- [X] T190 [P] Update `specs/001-gemara-native-workflow/quickstart.md` "Scanning Provider Development" — rename `HealthCheck` to `Describe` in method signatures, interface descriptions, and code example

### 13h: Verification

- [X] T191 Run `go build ./...` — all Phase 13 code compiles
- [X] T192 Run `go test ./...` — all tests pass
- [X] T193 Run `go vet ./...` and `gofmt -l .` — clean
- [X] T194 Verify: grep for `--plain` in `cmd/complyctl/cli/` Go source — zero matches expected (replaced by `--pretty`)
- [X] T195 Verify: grep for `bubbles/table` in non-vendor Go source — zero imports expected
- [X] T196 Verify: grep for `"complyctl.log"` in `cmd/complyctl/cli/root.go` — zero hardcoded matches expected (uses `complytime.LogFileName`)

**Checkpoint**: All tabular outputs default to plain aligned text. `--pretty` enables lipgloss tables. Log file at `.complytime/complyctl.log`. Scan totals compact inline. List shows two columns. `bubbles/table` vendor files removed.

---

## Phase 14: Output Path Split + Discovery Simplification, R58 (COMPLETE)

**Purpose**: Remove `--pretty` from discovery commands (`list`, `providers`). Write formatted scan reports (`--format`) to CWD instead of hidden `.complytime/scan/` directory. EvaluationLog stays in hidden dir. Implements R58 (Session 2026-02-26c).

**Dependencies**: Phase 13 complete (uses `ShowPlainTable`/`RenderTable` from `internal/terminal/table.go`). Modifies `list.go`, `providers.go`, `scan.go`.

**Independent Test**: `complyctl list` has no `--pretty` flag (rejects `--pretty`). `complyctl providers` has no `--pretty` flag. `complyctl scan --policy-id <ID> --format oscal` writes OSCAL report to CWD (`./oscal-assessment-results.json`), EvaluationLog to `.complytime/scan/evaluation-log.yaml`. EvaluationLog path always printed.

### 14a: Remove `--pretty` from Discovery Commands (FR-031, FR-032)

- [x] T197 [P] Remove `--pretty` from `cmd/complyctl/cli/list.go` — delete `pretty bool` field from `listOptions` struct, remove `cmd.Flags().BoolVar(&o.pretty, ...)` registration, update `printGemaraPolicyTable` to remove `pretty bool` parameter (always call `terminal.ShowPlainTable`, remove `terminal.RenderTable` branch). Update `Example` string if it references `--pretty`
- [x] T198 [P] Remove `--pretty` from `cmd/complyctl/cli/providers.go` — delete `pretty bool` field from `providersOptions` struct, remove `cmd.Flags().BoolVar(&o.pretty, ...)` registration, remove `if o.pretty` branch (always call `terminal.ShowPlainTable`). Remove `terminal.RenderTable` import if no longer needed

### 14b: Formatted Reports to CWD (FR-028, R58)

- [x] T199 Update `cmd/complyctl/cli/scan.go` `run()` — add `reportDir := "."` constant for formatted report output. Change `md.Write(outDir)` → `md.Write(reportDir)`, `output.ToSARIF(gemaraLog, "file:///scan", outDir)` → `output.ToSARIF(gemaraLog, "file:///scan", reportDir)`, `output.ToOSCAL(gemaraLog, outDir)` → `output.ToOSCAL(gemaraLog, reportDir)`. EvaluationLog path (`eval.Write(outDir)`) stays unchanged — diagnostic artifact stays in `.complytime/scan/`. `fmt.Printf("Evaluation log written: ...")` already exists and satisfies the "always print" requirement

### 14c: Verification

- [x] T200 Run `go build ./...` — all Phase 14 code compiles
- [x] T201 Run `go test ./...` — all tests pass
- [x] T202 Run `go vet ./...` and `gofmt -l .` — clean
- [x] T203 Verify: grep `cmd/complyctl/cli/list.go` and `cmd/complyctl/cli/providers.go` for `pretty` — zero matches expected (field, flag, and render branch all removed)
- [x] T204 Verify: grep `cmd/complyctl/cli/scan.go` for `md.Write(outDir)` — zero matches expected (should be `md.Write(reportDir)`)
- [x] T205 Verify: grep `cmd/complyctl/cli/scan.go` for `ToSARIF.*outDir` and `ToOSCAL.*outDir` — zero matches expected (should use `reportDir`)

**Checkpoint**: Discovery commands use plain output only (no `--pretty`). Formatted reports written to CWD. EvaluationLog in hidden dir. EvaluationLog path always printed. R58 complete.

---

## Phase 15: UX Refresh — Lipgloss Default, Scan Table, Execution Plan Collapse, R59 (COMPLETE)

**Purpose**: Make lipgloss-rendered tables the universal default (no `--pretty` flag). Redesign scan summary as a 4-column table (Requirement ID, Control ID, Status, Message). Collapse execution plan from two tables to one. Add TTY detection for automatic plain fallback on piped output. All tabular output follows report-style layout: intro text → subtle table → conclusion text. Implements R59 (Session 2026-02-26d).

**Dependencies**: Phase 14 complete. Modifies `internal/terminal/table.go`, `internal/output/scan_summary.go`, `internal/output/execution_plan.go`, `cmd/complyctl/cli/scan.go`, `cmd/complyctl/cli/generate.go`, `cmd/complyctl/cli/list.go`, `cmd/complyctl/cli/providers.go`.

**Independent Test**: `complyctl list` renders lipgloss table in terminal, plain when piped. `complyctl scan --policy-id <ID>` shows report-style summary with 4-column table of non-passing results. `complyctl generate --policy-id <ID>` shows single-table execution plan. No `--pretty` flag accepted by any command.

### 15a: Terminal Package — TTY Detection + Report Renderer

- [x] T206 [P] Add `IsTTY()` function to `internal/terminal/table.go` — uses `term.IsTerminal(os.Stdout.Fd())` from `charmbracelet/x/term` (already vendored for `TerminalWidth()`). Returns `bool`. Centralized TTY detection for all rendering decisions
- [x] T207 Add `RenderReport(w io.Writer, intro string, headers []string, rows [][]string, conclusion string)` function to `internal/terminal/table.go` — dispatches to `RenderTable` (lipgloss) when `IsTTY()` is true, `ShowPlainTable` when false. Writes intro line before table, conclusion line after table. Handles empty rows gracefully (prints intro + conclusion only)
- [x] T208 [P] Add `RenderTableString(headers []string, rows [][]string) string` convenience function to `internal/terminal/table.go` — returns lipgloss table as string (wraps existing `RenderTable`). Pairs with `ShowPlainTableString` for non-TTY path

### 15b: Scan Summary Redesign (FR-037, R59)

- [x] T209 Refactor `internal/output/scan_summary.go` — change `FormatScanSummary` signature to `FormatScanSummary(assessments []plugin.AssessmentLog, reqToControl map[string]string, policyID string, targetIDs []string) string`. Add `reqToControl` map parameter for control ID lookup. Build 4-column table rows: Requirement ID (`a.RequirementId`), Control ID (from `reqToControl[a.RequirementId]`, fallback `"-"`), Status (emoji), Message (`matchingStepMessage`). Non-passing only, ordered by severity. Return report-style string: intro line (`Scan: {policyID} | Target: {targets} | {total} requirements`), table rows via `terminal.RenderReport`, conclusion (compact inline totals + newline)
- [x] T210 Update `nonPassingEntry` struct in `internal/output/scan_summary.go` — add `requirementID string` and `controlID string` fields. Populate in the switch cases from `AssessmentLog.RequirementId` and `reqToControl` map
- [x] T211 Update `cmd/complyctl/cli/scan.go` — pass `reqToControl` map (already computed as `reqToControl := extractReqToControlMap(graph)`), `eid` (effective policy ID), and `targetIDs` to `output.FormatScanSummary`. Move EvaluationLog path and formatted report path printing into the conclusion string passed to the summary function, or print them after the summary call

### 15c: Execution Plan Collapse (FR-033, R59)

- [x] T212 Replace `ProviderRoute` and `TargetScope` structs with `ExecutionPlanRow` in `internal/output/execution_plan.go` — fields: `TargetID string`, `ProviderID string`, `RequirementCount int`, `Status string`. Remove `ProviderRoute` and `TargetScope` types
- [x] T213 Rewrite `FormatExecutionPlan` in `internal/output/execution_plan.go` — new signature: `FormatExecutionPlan(effectiveID string, rows []ExecutionPlanRow) string`. Remove `pretty bool` parameter. Build single table with headers: `Target`, `Provider`, `Requirements`, `Status`. Use `terminal.RenderReport` with intro (`Execution Plan: {effectiveID}`) and conclusion (`Generation completed.`)
- [x] T214 Update `cmd/complyctl/cli/generate.go` — remove `pretty bool` field from `generateOptions`. Remove `--pretty` flag registration. Build `[]output.ExecutionPlanRow` by iterating groups and `policyTargets` (one row per target-provider combination). Call `output.FormatExecutionPlan(eid, rows)`
- [x] T215 Update `cmd/complyctl/cli/scan.go` — remove `pretty bool` field from `scanOptions`. Remove `--pretty` flag registration. For `--dry-run` path: build `[]output.ExecutionPlanRow` and call `output.FormatExecutionPlan`. Remove `o.pretty` references

### 15d: Discovery Commands — Lipgloss Default

- [x] T216 [P] ~~Update `cmd/complyctl/cli/list.go` — RenderReport~~ **REVERTED**: `list` uses `terminal.ShowPlainTable` (podman-style plain output, no lipgloss)
- [x] T217 [P] ~~Update `cmd/complyctl/cli/providers.go` — RenderReport~~ **REVERTED**: `providers` uses `terminal.ShowPlainTable` with uppercase headers (podman-style plain output, no lipgloss)

### 15e: Cleanup

- [x] T218 Remove `OutputFormatPretty` usage check — verify `OutputFormatPretty` constant in `internal/complytime/consts.go` is only used for `--format pretty` (Markdown report format), not rendering style. No change needed if correct
- [x] T219 [P] Update `internal/output/scan_summary.go` — remove `aggregateResultFromSteps` if it duplicates go-gemara's built-in aggregation. Use `gemara.AggregateResults()` or equivalent. If no go-gemara aggregation function exists, keep the existing implementation with a TODO comment

### 15f: Verification

- [x] T220 Run `go build ./...` — all Phase 15 code compiles
- [x] T221 Run `go test ./...` — all tests pass
- [x] T222 Run `go vet ./...` and `gofmt -l .` — clean
- [x] T223 Verify: grep `cmd/complyctl/cli/scan.go` and `cmd/complyctl/cli/generate.go` for `pretty` — zero matches expected (field, flag, and render branch all removed)
- [x] T224 Verify: grep `internal/output/execution_plan.go` for `ProviderRoute\|TargetScope` — zero matches expected (replaced by `ExecutionPlanRow`)
- [x] T225 Verify: grep `internal/output/scan_summary.go` for `Requirement ID` column header present in table construction

**Checkpoint**: All tabular output uses lipgloss by default with TTY auto-detection. Scan summary is a 4-column table with report-style layout. Execution plan is a single table. No `--pretty` flag on any command. R59 complete.

---

## Phase 16: Bug Fixes — Error Consistency, Get Timeout, Resolver Error Surfacing (COMPLETE)

**Purpose**: Three independent bug fixes: (1) Error messages referencing "complytime" when they should reference "complyctl" or the specific config file. (2) `complyctl get` has no `--timeout` flag unlike `scan` and `generate`. (3) `ResolvePolicyGraph` silently discards errors when catalog/guidance layers fail to load or parse — invalid Gemara content or incompatible versions are not surfaced.

**Dependencies**: None — all three fixes are independent of Phase 15 and each other. Can be implemented in parallel.

**Independent Test**: (1) `complyctl scan` with invalid config shows error mentioning "complytime.yaml" not bare "complytime". (2) `complyctl get --timeout 10s` respects the timeout. (3) `complyctl scan` with a corrupted catalog layer in cache returns an error describing the parse failure instead of silently producing an empty graph.

### 16a: Error Message Consistency — "complyctl" in CLI Errors

- [x] T226 [P] Update error messages in `cmd/complyctl/cli/scan.go` — change `"failed to load complytime: %w"` → `"failed to load workspace config: %w"` (line 107). Change `"No targets configured in complytime"` → `"no targets configured in complytime.yaml"` (line 114). Change `"no targets in complytime config (add targets with policies)"` → `"no targets in complytime.yaml (add targets with policies)"` (line 115)
- [x] T227 [P] Update error messages in `cmd/complyctl/cli/generate.go` — change `"failed to load complytime: %w"` → `"failed to load workspace config: %w"` (line 80)
- [x] T228 [P] Update error messages in `cmd/complyctl/cli/list.go` — change `"failed to load complytime: %w"` → `"failed to load workspace config: %w"` (line 74)
- [x] T229 [P] Update error messages in `cmd/complyctl/cli/get.go` — change `"failed to load complytime: %w"` → `"failed to load workspace config: %w"` (line 62)
- [x] T230 [P] Update error messages in `cmd/complyctl/cli/init.go` — change `"failed to create complytime directory: %w"` → `"failed to create .complytime directory: %w"` (line 48, references the artifact directory, not the config file)
- [x] T231 [P] Update error messages in `internal/complytime/config.go` — change `"failed to read complytime file %s: %w"` → `"failed to read config file %s: %w"` (line 130). Change `"failed to marshal complytime: %w"` → `"failed to marshal workspace config: %w"` (line 185). Change `"failed to write complytime file %s: %w"` → `"failed to write config file %s: %w"` (line 189)

### 16b: `complyctl get` Timeout

- [x] T232 Add `timeout` field and `--timeout` flag to `cmd/complyctl/cli/get.go` — add `timeout time.Duration` to `getOptions` struct. Register `cmd.Flags().DurationVarP(&o.timeout, "timeout", "t", complytime.DefaultCommandTimeout, "Maximum time for the get operation (e.g. 5m, 10m, 1h)")`. In `run()`, wrap context with `context.WithTimeout(ctx, o.timeout)` and `defer cancel()` — same pattern as `scan.go` and `generate.go`

### 16c: Resolver Error Surfacing — Collect and Report Layer Errors

- [x] T233 Refactor `ResolvePolicyGraph` in `internal/policy/resolver.go` — collect errors from `LoadLayerByMediaType` calls for catalog and guidance layers instead of silently discarding them. When `LoadLayerByMediaType` returns an error for catalog or guidance, append to a `[]error` collector. After all layers are attempted: if the policy layer (Layer 3) failed to load, return error immediately (already done). If catalog or guidance layers failed to load AND the graph has zero controls and zero guidelines, return a combined error: `"policy %s@%s: no valid layers loaded: %s"` listing each layer failure. If some layers loaded but others failed, log warnings but proceed (partial graph is valid when optional layers are missing from the manifest)
- [x] T234 Refactor parse error handling in `ResolvePolicyGraph` in `internal/policy/resolver.go` — when `parseControlCatalog` or `parseGuidanceCatalog` returns an error (layer loaded successfully but content is not valid Gemara YAML or is an incompatible version), return an error instead of silently adding the unparsed layer: `"policy %s: catalog layer is not valid Gemara: %w"` or `"policy %s: guidance layer is not valid Gemara: %w"`. This ensures corrupted or incompatible-version content is surfaced to the admin rather than producing a graph with nil `Parsed` fields
- [x] T235 [P] Add resolver error surfacing tests in `internal/policy/resolver_test.go` — test: mock loader returns catalog data that is invalid YAML → `ResolvePolicyGraph` returns error mentioning "not valid Gemara". Test: mock loader returns guidance data that is invalid YAML → error surfaced. Test: mock loader returns error for catalog `LoadLayerByMediaType` (layer not in manifest) → graph built without catalog (no error, partial graph). Test: all three layers fail to load → error returned listing failures

### 16d: Verification

- [x] T236 Run `go build ./...` — all Phase 16 code compiles
- [x] T237 Run `go test ./...` — all tests pass (including new resolver error tests)
- [x] T238 Run `go vet ./...` and `gofmt -l .` — clean
- [x] T239 Verify: grep for `"failed to load complytime"` in `cmd/complyctl/cli/` Go source — zero matches expected
- [x] T240 Verify: grep `cmd/complyctl/cli/get.go` for `timeout` — confirms flag exists
- [x] T241 Verify: grep `internal/policy/resolver.go` for `parseErr == nil` — zero matches expected (parse errors now surfaced, not silently swallowed)

**Checkpoint**: All CLI error messages reference "complyctl" or the specific file name. `complyctl get` has `--timeout`. Policy resolver surfaces invalid Gemara content and incompatible version errors. Bug fixes complete.

---

## Phase 17: Plain Text Only — Lipgloss Removal, Generate Readout, Dry-Run Removal (Session 2026-02-26e)

**Purpose**: Remove `lipgloss/table` rendering entirely — persistent emoji alignment issues (double-width glyphs measured as single-width) broke column alignment across terminals. `ShowPlainTable` becomes the sole table renderer. Rewrite `generate` execution plan as a structured plain-text block (indented labeled lines per target-provider pair). `scan --dry-run` already absent from code (never implemented as a flag); this phase removes remaining spec/code references and cleans up the rendering pipeline.

**Dependencies**: Phase 15 and 16 complete. Modifies `internal/terminal/table.go`, `internal/output/execution_plan.go`, `internal/output/scan_summary.go`, `vendor/`.

**Independent Test**: `complyctl generate --policy-id <ID>` outputs structured plain-text block (no table borders). `complyctl scan --policy-id <ID>` shows plain aligned text table for non-passing results. `complyctl list` and `complyctl providers` unchanged (already plain). No lipgloss imports in `internal/terminal/table.go`.

### 17a: Terminal Package — Remove Lipgloss Table Rendering

- [x] T242 Remove lipgloss rendering functions and styles from `internal/terminal/table.go` — delete functions: `RenderTable`, `RenderTableString`, `SectionLabel`, `RenderReport`, `RenderReportString`, `TerminalWidth`. Delete style variables: `borderColor`, `headerStyle`, `cellStyle`, `borderStyle`, `sectionLabel`. Delete `defaultTermWidth` constant (only used by `TerminalWidth`). Remove imports: `"github.com/charmbracelet/lipgloss"`, `ltable "github.com/charmbracelet/lipgloss/table"`, `"strings"`. Keep: `IsTTY()` (retained for future use), `ShowPlainTable`, `"fmt"`, `"io"`, `"os"`, `"github.com/charmbracelet/x/term"` imports

### 17b: Execution Plan — Structured Plain-Text Block

- [x] T243 [P] Rewrite `FormatExecutionPlan` in `internal/output/execution_plan.go` — replace `terminal.RenderReportString` call with structured plain-text block using `fmt.Fprintf` to a `strings.Builder`. Format: intro line (`Execution Plan: {effectiveID}`), then for each row: indented stanza (`\n  Target: {targetID}\n    Provider: {providerID} ({statusEmoji} {status})\n    Requirements: {count}\n`), then conclusion line (`Generation completed.`). Map status to emoji: `"healthy"` → `complytime.StatusPassed`, `"ERROR"` → `complytime.StatusFailed`. Remove `terminal` import. Add `complytime` import for status emoji constants. Keep `ExecutionPlanRow` struct and `FormatPreScanSummary` unchanged. Update comment from "single-table report-style" to "structured plain-text block"

### 17c: Scan Summary — Plain Table Only

- [x] T244 [P] Update `FormatScanSummary` in `internal/output/scan_summary.go` — replace `terminal.RenderReportString(intro, headers, rows, conclusion)` call with inline logic using `strings.Builder`: `fmt.Fprintln(&b, intro)`, then `terminal.ShowPlainTable(&b, headers, rows)` if rows non-empty, then `fmt.Fprintln(&b, conclusion)`. Return `b.String()`. Update function comment from "lipgloss table" to "plain aligned text table"

### 17d: Vendor Cleanup

- [x] T245 Remove `vendor/github.com/charmbracelet/lipgloss/table/` directory (contains `table.go`, `resizing.go`, and any other files). Update `vendor/modules.txt` — remove the `github.com/charmbracelet/lipgloss/table` package line. Core `lipgloss` package stays in vendor (used by `pkg/log/log.go` and `charmbracelet/bubbles/spinner`). Run `go mod vendor` to verify consistency

### 17e: Verification

- [x] T246 Run `go build ./...` — all Phase 17 code compiles
- [x] T247 Run `go test ./...` — all tests pass
- [x] T248 Run `go vet ./...` and `gofmt -l .` — clean
- [x] T249 Verify: grep `internal/terminal/table.go` for `lipgloss\|ltable\|RenderTable\|RenderReport\|SectionLabel\|TerminalWidth` — zero matches expected
- [x] T250 Verify: grep `internal/output/execution_plan.go` for `RenderReport\|terminal\.` — zero matches expected (terminal import removed)
- [x] T251 Verify: grep `internal/output/scan_summary.go` for `RenderReportString` — zero matches expected (replaced with inline ShowPlainTable)
- [x] T252 Verify: `ls vendor/github.com/charmbracelet/lipgloss/table/` — directory should not exist

**Checkpoint**: All lipgloss table rendering removed. `ShowPlainTable` is the sole table renderer. Generate outputs structured plain-text block. Scan summary uses plain aligned table. No `lipgloss/table` in vendor. Session 2026-02-26e complete.

---

## Dependencies

### Phase 17 (Plain Text Only) — Depends on Phase 15

Modifies `internal/terminal/table.go`, `internal/output/execution_plan.go`, `internal/output/scan_summary.go`. T242 (terminal cleanup) MUST complete before T243 and T244 (consumers reference removed functions). T243 and T244 are fully parallel (different files). T245 (vendor) parallel with T243+T244. T246-T252 verification after all code tasks.

### Phase 15 (UX Refresh) — Depends on Phase 14

Modifies `internal/terminal/table.go`, `internal/output/scan_summary.go`, `internal/output/execution_plan.go`, and CLI files (`scan.go`, `generate.go`, `list.go`, `providers.go`). T206-T208 (terminal package) MUST complete before T209-T217 (consumers). T212-T213 (execution plan) parallel with T209-T211 (scan summary) — different files. T216+T217 (discovery) parallel with each other and with T209-T215.

### Phase 16 (Bug Fixes) — Independent of Phase 15

All three sub-tasks are independent: T226-T231 (error messages), T232 (get timeout), T233-T235 (resolver errors). Can be implemented in parallel with Phase 15. T226-T231 fully parallel (different files). T233-T234 sequential (same function). T235 depends on T233-T234.

### Phase 10 (List Fix) — Independent

No dependencies on Phase 11. Can be implemented immediately. T140-T141 are sequential (T141 depends on T140's row structure). T142-T144 are independent of each other but depend on T140.

### Phase 11 (Unit Tests) — Partially Parallel

- T145 (PolicyLoader interface) MUST complete before T146-T148 (resolver tests need the mock interface)
- T149-T151 (assessment), T152-T154 (generation state), T155-T156 (loader), T157-T160 (discovery) are all independent of each other — safe to parallelize
- No dependency on Phase 10

### Phase 12 (API Rename) — COMPLETE

Depends on Phases 1-11. Proto rename + code propagation. Self-contained — no Phase 13 dependency.

### Phase 13 (Terminal Output Redesign) — Depends on Phase 12

Modifies CLI files touched by Phase 12 (`scan.go`, `generate.go`, `providers.go`). Must run after Phase 12 is merged. T178-T179 (constants + log) are independent of T180-T185 (table changes). T186-T187 (vendor cleanup) depend on T185 (last `terminal.RenderTable` removal from scan_summary). T188-T190 (docs) independent of code tasks.

### Phase 14 (Output Path Split) — Depends on Phase 13

Modifies `list.go`, `providers.go`, `scan.go` — all touched by Phase 13. T197+T198 are fully parallel (different files). T199 is independent of T197+T198 (different file). T200-T205 verification runs after all code tasks.

### Parallel Execution

```text
Phase 10 (independent):         T140 → T141 → [T142, T143, T144]
Phase 11 group A (after T145):  T146, T149, T152, T155, T157
Phase 11 group B (after A):     T147, T148, T150, T151, T153, T154, T156, T158, T159, T160
Phase 12 (sequential):          T161-T163 → T164-T176 → T177
Phase 13 group A (parallel):    T178, T180, T181, T185, T188
Phase 13 group B (after A):     T179, T182 → [T183, T184], T186, T187, T189, T190
Phase 13 verification:          T191-T196 (after all code tasks)
Phase 14 (all parallel):        [T197, T198, T199] → T200-T205 (verification)
Phase 15 foundation:            [T206, T208] → T207
Phase 15 consumers (after 207): [T209+T210, T212+T213, T216, T217] → [T211, T214, T215] → [T218, T219]
Phase 15 verification:          T220-T225 (after all code tasks)
Phase 16 (all parallel):        [T226-T231] | T232 | [T233 → T234 → T235]
Phase 16 verification:          T236-T241 (after all code tasks)
Phase 17 foundation:            T242 (terminal cleanup)
Phase 17 consumers (after 242): [T243, T244, T245] (all parallel — different files)
Phase 17 verification:          T246-T252 (after all code tasks)
```

---

## Key Implementation Notes

- **PolicyEntry model (Session 2026-02-25d)**: `Policies []PolicyEntry` where each entry has `url` (full OCI reference) and optional `id` (shortname). `EffectiveID()` returns explicit `id` or derives from last URL path segment. No `pack` field, no `RegistryConfig` in `WorkspaceConfig`. Targets use `policies` (list of effective IDs). `FindPolicy()` matches by effective ID → full URL → repo path. `UniqueRegistries()` extracts distinct registries for per-registry client creation
- **Three-tier variables (R48)**: Global variables flow via `GenerateRequest.global_variables`. Test variables flow via `AssessmentConfiguration.parameters`. Target variables flow via `Target.variables`. Do not conflate
- **Targets-only Scan RPC (R47)**: No `requirement_ids`. Providers evaluate all requirements from Generate-time state
- **Multi-evaluator (R32)**: `GroupByEvaluator()` groups by per-plan executor ID. Execution plan shows unmatched evaluators as ERROR rows
- **Zero custom auth (R6, R24)**: All credential resolution via `oras-credentials-go`. No `Authenticator` struct
- **Result aggregation (R45)**: Delegates to go-gemara. No custom aggregation in complyctl
- **Describe variable declaration (R51)**: Providers declare required variable *names* via `DescribeResponse`. Doctor validates presence in config. Valid *values* remain provider's responsibility. `HealthCheck` renamed to `Describe` (Phase 12)
- **Pack separation (Session 2026-02-25d)**: Pack builder is a separate tool. All pack CLI commands deferred to 002. Pack manifest types remain in `internal/complytime/pack.go` as data model
- **Plain text only (Session 2026-02-26e)**: Supersedes R59. `lipgloss/table` removed entirely — persistent emoji alignment issues. `ShowPlainTable` is the sole renderer for all tabular output (scan results, `list`, `providers`). `generate` uses structured plain-text block (indented labeled lines), not a table. `scan --dry-run` removed — `generate` is the only execution plan preview. No `--pretty`, no `--plain`, no TTY-gated rendering dispatch. `RenderTable`, `RenderReport`, `RenderReportString`, `SectionLabel` removed from `internal/terminal`. Core `lipgloss` stays (log/spinner dependency)
- **Log file (R57)**: Written to `{WorkspaceDir}/{LogFileName}` (`.complytime/complyctl.log`). `LogFileName` constant in `internal/complytime/consts.go`
- **parsePolicyLayer error contract (R39)**: Only `gemara.Policy` with `adherence.assessment-plans` accepted. No fallback
- **Terminal output tiers (R40)**: Tier 1 (`init`, `get`) writes progress to stderr. Tier 2 (`list`, `providers`, `generate`, `scan`, `doctor`) writes summaries to stdout. No log file for `version`, `list`, `init`, `get`
- **Convention over configuration (R50)**: `complytime.yaml` is a static convention. No `--config` flag. `--policy-id` always required
- **Output path split (R58)**: Formatted reports (OSCAL, SARIF, Markdown) write to CWD — user-facing deliverables. EvaluationLog stays in `.complytime/scan/` — diagnostic artifact. EvaluationLog path always printed to terminal. `--pretty` removed from all commands (R59 supersedes R58's "retain on scan/generate")
- **Doctor version comparison (R55)**: `CheckPolicyVersions()` replaces `CheckRegistries()`. Compares cached vs remote latest per-policy. Non-blocking warnings for stale policies and unreachable registries. `--verbose` expands per-provider variable detail only. Uses existing `DefinitionVersion()` and `PolicyState` — no new infrastructure

---

## Phase 18: Init Hardening + OpenSCAP Debug Command (Session 2026-03-02)

**Purpose**: Implement PR #381 feedback (OCI reference validation, duplicate handling, empty-policies default) and surface the `oscap` command in provider logs so admins can manually reproduce and debug hanging scans. The OpenSCAP provider currently logs the command at Debug level (`hclog.Debug`), which is filtered out by complyctl's go-plugin log filtering (WARN and above). On timeout, the error message says "command timed out or was cancelled" without including the command that was run — admins cannot reproduce the failure.

**Dependencies**: Phase 3 complete (T043 `init.go` exists). OpenSCAP plugin code exists in `cmd/openscap-plugin/`.

**Independent Test**: `complyctl init` rejects non-OCI-reference input, warns on duplicate URLs, and creates a commented template when no policies are provided. When an `oscap` scan times out, the error message includes the full shell-ready command string. `complyctl scan` log file (`.complytime/complyctl.log`) contains the `oscap` command at Info level.

### 18a: OCI Reference Validation

- [x] T253 [US1] Add OCI reference format validation to `promptPolicies()` in `cmd/complyctl/cli/init.go` — reject inputs not matching OCI reference format (registry/repo:tag or @digest). Display validation error and re-prompt. Also validate in `ParsePolicyRef()` or `Validate()` in `internal/complytime/config.go` for non-interactive paths (CI/CD commits `complytime.yaml` directly)
- [x] T254 [P] [US1] Add unit tests for OCI reference validation — valid refs accepted (`registry.com/repo:tag`, `registry.com/repo@sha256:abc`), invalid refs rejected (bare words, shell injection like `ls;pwd`, empty string, whitespace-only)

### 18b: Duplicate Policy Handling

- [x] T255 [US1] Update `promptPolicies()` in `cmd/complyctl/cli/init.go` — detect duplicate policy URLs during interactive entry. Log warning (e.g., `⚠️ duplicate policy URL skipped: <url>`), skip the duplicate entry, continue prompting
- [x] T256 [P] [US1] Add unit test for duplicate detection — two identical URLs produce one `PolicyEntry` + warning log

### 18c: Empty Policies Default

- [x] T257 [US1] Update config creation in `cmd/complyctl/cli/init.go` — when no policies are provided, write `complytime.yaml` with an empty `policies: []` list and a YAML comment showing the `url` + `id` structure as an example. No fake URL in actual config data
- [x] T258 [P] [US1] Add unit test for empty policies path — verify generated YAML contains empty policies list and comment block

### 18d: OpenSCAP Debug Command Visibility

- [x] T259 [US2] Update `executeCommand()` in `cmd/openscap-plugin/oscap/oscap.go` — promote command logging from `hclog.Debug` to `hclog.Info` so the full command appears in `.complytime/complyctl.log` (go-plugin output is filtered to WARN+ but provider's own Info-level messages pass through). Format command as a shell-ready string (`strings.Join(command, " ")`) instead of Go slice representation. On context timeout (line 27), include the shell-ready command in the error message: `"command timed out after deadline: %s\n\nTo debug, run manually:\n  %s"` with `ctx.Err()` and the joined command string. Allows admins to copy-paste and reproduce the hang
- [x] T260 [P] [US2] Add unit test for timeout error format in `cmd/openscap-plugin/oscap/oscap_test.go` — verify timeout error message contains the command string. Verify non-timeout errors do not include the command

### 18e: Verification

- [x] T261 Run `go build ./...` — all Phase 18 code compiles (including `cmd/openscap-plugin/`)
- [x] T262 Run `go test ./...` — all tests pass
- [x] T263 Run `go vet ./...` and `gofmt -l .` — clean

**Checkpoint**: Init validates OCI references, handles duplicates gracefully, produces useful empty config. OpenSCAP timeout errors include the full command for manual debugging. Session 2026-03-02 complete.

---

## Notes

- [P] tasks = different files, no dependencies — safe to parallelize
- [USn] label maps task to specific user story
- Each phase is independently testable at its checkpoint
- Phases 1-8 (T001-T118) are COMPLETE — core 001 implementation
- Phase 9 (T119-T139) is deferred to `002-comply-packs` branch
- Phase 10 (T140-T144) is the list table fix — COMPLETE
- Phase 11 (T145-T160) is unit test coverage — R56, COMPLETE
- Phase 12 (T161-T177) is API rename + bug fix — HealthCheck → Describe, ValidateGlobalVars removal, COMPLETE
- Phase 13 (T178-T196) is terminal output redesign — R57, Session 2026-02-26b, COMPLETE
- Phase 14 (T197-T205) is output path split + discovery simplification — R58, Session 2026-02-26c, COMPLETE
- Phase 15 (T206-T225) is UX refresh — R59, Session 2026-02-26d, lipgloss default + scan table + execution plan collapse
- Phase 16 (T226-T241) is bug fixes — error consistency + get timeout + resolver error surfacing
- Phase 17 (T242-T252) is plain text only — Session 2026-02-26e, lipgloss removal + generate readout + dry-run removal
- Phase 18 (T253-T263) is init hardening + OpenSCAP debug command — Session 2026-03-02, OCI validation + duplicate handling + empty default + oscap command in timeout errors
