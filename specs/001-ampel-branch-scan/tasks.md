# Tasks: AMPEL Branch Protection Scanning Plugin

**Input**: Design documents from `specs/001-ampel-branch-scan/`
**Prerequisites**: plan.md (required), spec.md (required),
research.md, data-model.md, contracts/

**Tests**: Included. Constitution principle III (Test-Required)
mandates tests for all exported functionality. User explicitly
requested mocked fixtures for verifying assessment plan ↔ AMPEL
policy linkage.

**Organization**: Tasks grouped by user story. Each story is
independently testable after its phase completes.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story (US1, US2, US3, US4)
- All file paths are relative to `cmd/ampel-plugin/`

---

## Phase 1: Setup

**Purpose**: Project initialization, directory structure, entry
point, and sample files.

- [x] T001 Create package directory structure under
  `cmd/ampel-plugin/` with subdirectories: `config/`, `server/`,
  `convert/`, `convert/testdata/`, `toolcheck/`, `targets/`,
  `targets/testdata/`, `scan/`, `results/`,
  `results/testdata/`, `docs/samples/`

- [x] T002 [P] Implement plugin entry point in
  `cmd/ampel-plugin/main.go`. Initialize hclog logger with
  name "ampel-plugin", Debug level, JSON format, stderr output.
  Create PluginServer via `server.New()`, wrap in
  `plugin.PVPPlugin{Impl: server}`, register with
  `plugin.Register()`. Follow the exact pattern from
  `cmd/openscap-plugin/main.go`. Import
  `compliance-to-policy-go/v2/plugin` and
  `hashicorp/go-plugin`.

- [x] T003 [P] Create plugin manifest sample at
  `cmd/ampel-plugin/docs/samples/c2p-ampel-manifest.json` with
  metadata (id: "ampel", types: ["pvp"]), executablePath
  "ampel-plugin", and configuration options: workspace
  (required), profile (required), policy_dir (optional,
  default "policy"), results_dir (optional, default "results"),
  targets_file (optional, default "ampel-targets.yaml"). Follow
  schema in `contracts/plugin-manifest-schema.md`.

- [x] T004 [P] Create sample target config at
  `cmd/ampel-plugin/docs/samples/ampel-targets.yaml` with two
  example repositories (one GitHub, one GitLab), each with
  branches list. Follow schema in
  `contracts/target-config-schema.yaml`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types, configuration, and server skeleton that
ALL user stories depend on.

**CRITICAL**: No user story work can begin until this phase
completes.

- [x] T005 [P] Define AMPEL policy data structures in
  `cmd/ampel-plugin/convert/types.go`. Implement Go structs for:
  AmpelPolicy (ID, Meta, Tenets), AmpelMeta (Runtime,
  Description, AssertMode, Version, Controls, Enforce),
  AmpelTenet (ID, Title, Predicates, Code, Outputs), Control
  (Source, ID), PredicateSpec (Types), Output (Code, Value).
  Use JSON struct tags with snake_case field names
  (e.g., `json:"assert_mode"`). See data-model.md AmpelPolicy
  entity and research.md R-002 for exact field definitions.

- [x] T006 [P] Implement PluginConfig in
  `cmd/ampel-plugin/config/config.go`. Define Config struct with
  fields: Workspace, Profile, PolicyDir, ResultsDir,
  TargetsFile (see data-model.md PluginConfig entity). Implement
  `NewConfig() *Config` constructor. Implement
  `LoadSettings(configMap map[string]string) error` that maps
  config keys to struct fields, applies defaults
  (PolicyDir="policy", ResultsDir="results",
  TargetsFile="ampel-targets.yaml"), and creates workspace directories. All optional path fields
  (PolicyDir, ResultsDir, TargetsFile) store their default
  values as-is (relative strings); path resolution is deferred
  to T032. Implement `Validate() error` to check required
  fields. Follow input sanitization patterns from
  `cmd/openscap-plugin/config/config.go`. Define package
  constants: PluginDir="ampel", PolicyDir="policy",
  ResultsDir="results".

- [x] T007 Implement server skeleton in
  `cmd/ampel-plugin/server/server.go`. Define PluginServer
  struct holding `*config.Config`. Implement `New() PluginServer`
  constructor. Add compile-time interface check:
  `var _ policy.Provider = (*PluginServer)(nil)`. Implement
  stub methods Configure (delegates to config.LoadSettings),
  Generate (returns nil), GetResults (returns empty PVPResult).
  Import `compliance-to-policy-go/v2/policy`. Depends on T005
  and T006.

**Checkpoint**: Foundation ready. Plugin compiles and registers
with complyctl (no-op for Generate/GetResults).

---

## Phase 3: User Story 1 - Generate AMPEL Policies (P1)

**Goal**: Translate OSCAL assessment plan rules into AMPEL policy
JSON files. Honor complyctl policy scope over any existing AMPEL
policy.

**Independent Test**: Run `complyctl generate` with an assessment
plan containing branch protection rules. Verify AMPEL policy
files appear in `{workspace}/ampel/policy/` with exactly the
controls from the assessment plan.

### Test Fixtures for User Story 1

- [x] T008 [P] [US1] Create mock OSCAL assessment plan fixture
  at `cmd/ampel-plugin/convert/testdata/assessment-plan-full.json`.
  Build a `policy.Policy` ([]extensions.RuleSet) JSON with 3-5
  branch protection rules, each having checks with IDs and
  descriptions, and parameters with selected values. Reference
  research.md R-003 for mapping structure. Include rules for:
  PR requirement, minimum approvals, force push blocking, code
  owner review, stale review dismissal.

- [x] T009 [P] [US1] Create subset assessment plan fixture at
  `cmd/ampel-plugin/convert/testdata/assessment-plan-subset.json`.
  Same structure as T008 but with only 2 rules (PR requirement
  and force push blocking). This tests scope filtering per
  FR-003.

- [x] T010 [P] [US1] Create expected AMPEL policy output fixtures:
  `cmd/ampel-plugin/convert/testdata/ampel-policy-expected-full.json`
  (matches full plan from T008) and
  `cmd/ampel-plugin/convert/testdata/ampel-policy-expected-subset.json`
  (matches subset plan from T009). Each must be valid AMPEL
  Policy API v1 JSON with correct tenets, CEL expressions,
  predicate type
  `http://github.com/carabiner-dev/snappy/specs/branch-rules.yaml`,
  and `assert_mode: "AND"`. See research.md R-002 for format.

- [x] T011 [P] [US1] Create broader pre-existing AMPEL policy
  fixture at
  `cmd/ampel-plugin/convert/testdata/ampel-policy-existing-broader.json`.
  Contains 7+ tenets covering more controls than either
  assessment plan fixture. Used to test that Generate() ignores
  existing broader policy and generates only what the assessment
  plan requires (FR-003, US1 AS2).

### Tests for User Story 1

- [x] T012 [US1] Write unit tests in
  `cmd/ampel-plugin/convert/convert_test.go`. Use table-driven
  tests with testify/require. Test cases:
  (1) Full plan → full expected policy (compare against fixture
  from T010, assert tenet count, IDs, and CEL content).
  (2) Subset plan → subset expected policy (fewer tenets).
  (3) Empty policy input → nil/empty output, no error.
  (4) Nil policy input → appropriate error.
  (5) Rule with no checks → skipped gracefully.
  (6) Rule with empty check ID → returns error.
  (7) Verify changing assessment plan changes output (modify one
  rule parameter, confirm tenet CEL expression differs).
  Load fixtures from testdata/ using os.ReadFile. Depends on
  T008-T011.

### Implementation for User Story 1

- [x] T013 [US1] ~~(Superseded)~~ Originally: Implement
  `PolicyToAmpel` with CEL generation. **Replaced by T040-T042**:
  Implement granular policy matching approach with
  `LoadGranularPolicies`, `MatchPolicies`, and `MergeToBundle`
  functions in `cmd/ampel-plugin/convert/convert.go`. See
  research.md R-003 (updated) for rationale.

- [x] T014 [US1] ~~(Superseded)~~ Originally: Implement
  `WritePolicy` with filename `branch-protection-policy.json`.
  **Updated**: Bundle is now written to
  `complytime-ampel-policy.json` by `MergeToBundle`.

- [x] T015 [US1] Implement Generate() method body in
  `cmd/ampel-plugin/server/server.go`. Call
  `convert.PolicyToAmpel(policy, convertConfig)` then
  `convert.WritePolicy(ampelPolicy, config.PolicyDir)`. Log
  info messages for "Generating AMPEL policy" and "AMPEL policy
  written to {path}". Handle and propagate errors with context.
  Return nil for empty policy case. Depends on T013, T014.

- [x] T016 [US1] Write unit tests for Generate() in
  `cmd/ampel-plugin/server/server_test.go`. Test cases:
  (1) Generate with valid policy creates JSON file in temp dir.
  (2) Generate with empty policy produces no file, no error.
  (3) Generate overwrites existing policy file.
  (4) Generate with invalid config path returns error.
  Use testify/require. Create temp directories for workspace.
  Depends on T015.

- [x] T017 [US1] Write unit tests for config in
  `cmd/ampel-plugin/config/config_test.go`. Test cases:
  (1) LoadSettings with all required fields succeeds.
  (2) LoadSettings without workspace returns error.
  (3) LoadSettings without profile returns error.
  (4) LoadSettings applies defaults for optional fields.
  (5) LoadSettings with absolute policy_dir uses it as-is.
  (6) LoadSettings with relative policy_dir resolves against
  workspace/ampel/.
  (7) LoadSettings with empty string values returns error.
  (8) Validate checks directories are created.
  Use temp directories and testify/require. Depends on T006.

**Checkpoint**: `complyctl generate` produces correct AMPEL policy
files. Policy linkage with assessment plan is verified through
fixture-based tests.

---

## Phase 4: User Story 2 - Scan Branch Protection (P1)

**Goal**: Invoke AMPEL toolchain to scan target repositories,
produce per-repository result files, return PVPResult to
complyctl.

**Independent Test**: Run `complyctl scan` with generated policy
and target repos. Verify per-repo result files in workspace and
observations in assessment-results.json.

### Test Fixtures for User Story 2

- [x] T018 [P] [US2] Create target config test fixtures in
  `cmd/ampel-plugin/targets/testdata/`:
  `valid-targets.yaml` (2 repos, multiple branches),
  `duplicates-targets.yaml` (same repo+branch listed twice),
  `empty-targets.yaml` (repositories: []),
  `invalid-url-targets.yaml` (invalid URL format).
  Follow schema from contracts/target-config-schema.yaml.

- [x] T019 [P] [US2] Create AMPEL verify output test fixtures
  in `cmd/ampel-plugin/results/testdata/`:
  `ampel-verify-pass.json` (all tenets pass),
  `ampel-verify-fail.json` (some tenets fail with reasons),
  `ampel-verify-error.json` (tool error output).
  Base format on actual `ampel verify` JSON output structure.

### Tests for User Story 2

- [x] T020 [P] [US2] Write unit tests in
  `cmd/ampel-plugin/targets/targets_test.go`. Test cases:
  (1) Parse valid config returns correct repo count and branches.
  (2) Parse duplicates warns and deduplicates (assert len after).
  (3) Parse empty config returns error.
  (4) Parse invalid URL returns error with URL in message.
  (5) Parse missing file returns error.
  (6) Parse malformed YAML returns error.
  (7) Repo with empty branches list returns error.
  Use testdata fixtures from T018. Depends on T018.

- [x] T021 [P] [US2] Write unit tests in
  `cmd/ampel-plugin/results/results_test.go`. Test cases:
  (1) Parse pass output → all subjects ResultPass.
  (2) Parse fail output → failed subjects with reasons.
  (3) Parse error output → subjects with ResultError.
  (4) Parse empty output → returns error.
  (5) Parse malformed JSON → returns error.
  (6) Parse output with control characters in fields → stripped.
  (7) Parse output with oversized field values → returns error.
  (8) WritePerRepoResult creates JSON file with correct name.
  (9) WritePerRepoResult overwrites existing file.
  (10) ToPVPResult aggregates multiple per-repo results correctly,
  each repo as distinct Subject. Use testdata from T019.
  Depends on T019.

- [x] T022 [P] [US2] Write unit tests in
  `cmd/ampel-plugin/scan/scan_test.go`. Test cases:
  (1) constructSnappyCommand builds correct args for GitHub repo.
  (2) constructSnappyCommand builds correct args for GitLab repo.
  (3) constructAmpelVerifyCommand builds correct args with policy
  and attestation paths.
  (4) ScanRepository with mock exec returns expected output.
  (5) ScanRepository handles command not found error.
  (6) ScanRepository handles command execution error.
  Test command construction only (do not invoke real tools).
  Depends on scan.go interfaces being defined.

### Implementation for User Story 2

- [x] T023 [US2] Implement target config parsing in
  `cmd/ampel-plugin/targets/targets.go`. Define TargetConfig
  and TargetRepository structs per data-model.md. Implement
  `LoadTargets(path string) (*TargetConfig, []string, error)`
  that reads YAML file, validates URLs (must be HTTPS, must be
  github.com or gitlab.com host), validates branches non-empty,
  deduplicates by URL+branch, and returns config plus list of
  warning messages for duplicates. Use `goccy/go-yaml` for
  parsing. Depends on T018 (for test validation).

- [x] T024 [US2] Implement scan orchestration in
  `cmd/ampel-plugin/scan/scan.go`. Implement
  `constructSnappyCommand(repo TargetRepository, branch, outputDir string) []string`
  to build snappy CLI args for collecting branch protection data.
  Implement
  `constructAmpelVerifyCommand(policyPath, attestationPath, subjectRepo string) []string`
  to build ampel verify CLI args.
  Implement `ScanRepository(repo TargetRepository, branch string, cfg ScanConfig) (*RawScanResult, error)`
  that: (1) runs snappy to collect attestation data,
  (2) runs ampel verify against policy,
  (3) captures combined output, (4) returns raw result.
  Use `exec.LookPath` + `exec.Command` pattern from
  openscap-plugin oscap/oscap.go. Define ScanConfig struct with
  PolicyPath, OutputDir fields. Log each command execution via
  hclog.

- [x] T025 [US2] Implement result mapping in
  `cmd/ampel-plugin/results/results.go`. Define PerRepoResult
  and Finding structs per data-model.md. Implement
  `ParseAmpelOutput(raw []byte) (*PerRepoResult, error)` to
  parse ampel verify JSON output into PerRepoResult. Validate
  parsed output before returning: reject results with field
  values exceeding 10KB, strip control characters from string
  fields, and verify that tenet IDs and repository identifiers
  contain only printable ASCII. Return error with context if
  validation fails. Implement
  `WritePerRepoResult(result *PerRepoResult, dir string) error`
  to write per-repo JSON file named
  `{sanitized-repo}-{branch}.json`. Implement
  `ToPVPResult(results []*PerRepoResult) policy.PVPResult` that
  maps each PerRepoResult to policy.ObservationByCheck with:
  Title from tenet title, CheckID from tenet ID, Methods
  ["AUTOMATED"], Subjects with repo URL as ResourceID, repo
  name as Title, type "inventory-item", and mapped Result
  (pass/fail/error). Include evidence link to per-repo result
  file.

- [x] T026 [US2] Implement GetResults() method body in
  `cmd/ampel-plugin/server/server.go`. Flow:
  (1) Load target config via targets.LoadTargets(config.TargetsFile).
  (2) Log any duplicate warnings.
  (3) For each repo+branch: call scan.ScanRepository().
  (4) If scan errors for a repo, create PerRepoResult with
  status "error" and continue to next repo.
  (5) Parse scan output via results.ParseAmpelOutput().
  (6) Write per-repo result via results.WritePerRepoResult().
  (7) Collect all results and call results.ToPVPResult().
  (8) Return aggregated PVPResult.
  Log info for each repo scanned. Depends on T023-T025.

- [x] T027 [US2] Write unit tests for GetResults() in
  `cmd/ampel-plugin/server/server_test.go`. Test cases:
  (1) GetResults with valid config and mock scan output returns
  correct PVPResult observations.
  (2) GetResults with unreachable repo returns error status for
  that repo, continues scanning.
  (3) GetResults with missing targets file returns error.
  (4) GetResults creates per-repo result files in workspace.
  (5) GetResults with empty targets returns error.
  Use temp directories and mock fixtures. Depends on T026.

**Checkpoint**: `complyctl scan` produces per-repository result
files and returns PVPResult with per-repo observations. Error
repos get error status, scanning continues.

---

## Phase 5: User Story 3 - Validate Required Tools (P2)

**Goal**: Check that snappy and ampel are installed before
any plugin operation. Report missing tools clearly.

**Independent Test**: Remove ampel from PATH, run
`complyctl generate`. Verify error message names "ampel" and
suggests PATH check.

### Implementation for User Story 3

- [x] T028 [P] [US3] Implement tool presence checking in
  `cmd/ampel-plugin/toolcheck/toolcheck.go`. Define
  `RequiredTools = []string{"snappy", "ampel"}`.
  Implement `CheckTools() ([]string, error)` that uses
  `exec.LookPath` for each tool, collects missing tool names,
  returns them. Implement `FormatMissingToolsError(missing []string) error`
  that constructs an error message listing each missing tool
  by name and suggesting: "Ensure the following tools are
  installed and available on your PATH: {tools}. See
  AMPEL documentation for installation instructions."

- [x] T029 [P] [US3] Write unit tests in
  `cmd/ampel-plugin/toolcheck/toolcheck_test.go`. Test cases:
  (1) CheckTools when all tools exist on PATH returns empty
  missing list.
  (2) CheckTools when one tool missing returns that tool name.
  (3) CheckTools when all tools missing returns all names.
  (4) FormatMissingToolsError with one tool includes tool name.
  (5) FormatMissingToolsError with multiple tools lists all.
  (6) FormatMissingToolsError mentions PATH in message.
  Use exec.LookPath behavior (test with known-good command like
  "ls" and known-bad command like "nonexistent-tool-xyz").

- [x] T030 [US3] Integrate tool checking into server.go. Call
  `toolcheck.CheckTools()` at the beginning of Generate() and
  GetResults() in `cmd/ampel-plugin/server/server.go`. If any
  tools are missing, return the formatted error immediately
  before performing any other work. Log warning with missing
  tool names via hclog. Depends on T028.

- [x] T031 [US3] Add tool check integration tests to
  `cmd/ampel-plugin/server/server_test.go`. Test cases:
  (1) Generate returns tool error when required tool is missing.
  (2) GetResults returns tool error when required tool is
  missing.
  (3) Error message includes specific missing tool name.
  Depends on T030.

**Checkpoint**: Plugin reports missing tools before attempting
any operation.

---

## Phase 6: User Story 4 - Configure AMPEL Policy Location (P2)

**Goal**: Allow custom policy directory, results directory, and
targets file paths via plugin configuration.

**Independent Test**: Set policy_dir to a custom path in manifest
config, run `complyctl generate`, verify policy appears at
custom path.

### Implementation for User Story 4

- [x] T032 [US4] Implement path resolution in
  `cmd/ampel-plugin/config/config.go`. Add method
  `ResolvePaths() error` that resolves PolicyDir, ResultsDir,
  and TargetsFile: if absolute, use as-is; if relative,
  resolve against `{Workspace}/ampel/`. Create directories if
  they do not exist. Log resolved paths via hclog at debug
  level. Call ResolvePaths() at end of LoadSettings().
  Depends on T006.

- [x] T033 [US4] Write unit tests for custom path configuration
  in `cmd/ampel-plugin/config/config_test.go`. Test cases:
  (1) Absolute policy_dir is used unchanged.
  (2) Relative policy_dir resolves against workspace/ampel/.
  (3) Non-existent custom dir is created automatically.
  (4) Custom targets_file path is resolved correctly.
  (5) Custom results_dir path is resolved correctly.
  (6) Empty custom path falls back to default.
  Use temp directories. Depends on T032.

- [x] T034 [US4] Verify server.go Generate() and GetResults()
  use config paths correctly. Add test case in
  `cmd/ampel-plugin/server/server_test.go`:
  (1) Generate with custom policy_dir writes to that dir.
  (2) GetResults with custom results_dir writes per-repo files
  there. Depends on T032.

**Checkpoint**: Users can override policy location via manifest
configuration.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Quality gates, linting, documentation, final
validation.

- [x] T035 [P] Run `go vet ./cmd/ampel-plugin/...` and fix any
  warnings. Ensure all code passes static analysis.

- [x] T036 [P] Run `golangci-lint run ./cmd/ampel-plugin/...`
  (or project-configured equivalent) and fix any issues.

- [x] T037 Run `go test -race -v ./cmd/ampel-plugin/...` and
  verify all tests pass with race detection enabled.

- [x] T038 Verify test coverage does not have gaps in exported
  functions. Run
  `go test -coverprofile=coverage.out ./cmd/ampel-plugin/...`
  and review `go tool cover -func=coverage.out`.

- [ ] T039 Run quickstart.md validation: manually walk through
  each step in `specs/001-ampel-branch-scan/quickstart.md` and
  verify the documented workflow produces expected outputs.

---

## Phase 8: Post-Initial Implementation Improvements

**Purpose**: Address gaps discovered during integration testing
and real-world usage.

### Granular Policy Matching (replaces CEL generation)

- [x] T040 [US1] Implement `LoadGranularPolicies` in
  `cmd/ampel-plugin/convert/convert.go`. Loads all `*.json`
  files from the policy directory, parses each as an AmpelPolicy,
  returns the full set. Replaces the CEL generation approach
  from T013.

- [x] T041 [US1] Implement `MatchPolicies` in
  `cmd/ampel-plugin/convert/convert.go`. Matches OSCAL rule IDs
  against granular policy IDs. Returns only the policies that
  correspond to rules in the assessment plan.

- [x] T042 [US1] Implement `MergeToBundle` in
  `cmd/ampel-plugin/convert/convert.go`. Merges matched
  policies into a single AmpelPolicyBundle and writes to
  `complytime-ampel-policy.json`. Bundle ID is always
  "complytime-ampel-policy".

### Per-Repository Spec Configuration

- [x] T043 [US2] Add `specs` field to TargetRepository in
  `cmd/ampel-plugin/targets/targets.go`. Each repository MUST
  specify one or more snappy spec file references. Support
  `builtin:` prefix for embedded specs and absolute paths for
  custom specs. Validate that specs is non-empty. Deduplicate
  specs within a repository.

- [x] T044 [US2] Update scan orchestration in
  `cmd/ampel-plugin/scan/scan.go` to iterate over each
  repo/branch/spec combination. Embed spec files under
  `scan/specs/` using `//go:embed`. Resolve `builtin:` prefix
  to embedded files written to workspace.

- [x] T045 [US2] Add test fixtures and tests for per-repo spec
  configuration in targets and server packages.

### DSSE Envelope Handling

- [x] T046 [US2] Add DSSE envelope unwrapping to
  `ParseAmpelOutput` in `cmd/ampel-plugin/results/results.go`.
  Before parsing the in-toto statement, check if the raw JSON
  is a DSSE envelope (has payloadType and payload fields). If
  so, base64-decode the payload (try RawURL then StdEncoding)
  and parse the decoded content. Add `encoding/base64` import.

- [x] T047 [US2] Create DSSE test fixture at
  `cmd/ampel-plugin/results/testdata/ampel-verify-dsse-fail.json`
  and add `TestParseAmpelOutput_DSSEEnvelope` test case.

### Multi-Target Observation Grouping

- [x] T048 [US2] Update `ToPVPResult` in
  `cmd/ampel-plugin/results/results.go` to group findings by
  CheckID. Each unique CheckID produces one ObservationByCheck
  with multiple Subjects (one per repository). Use insertion-
  order tracking for deterministic output. This prevents
  last-write-wins overwrites in the downstream oscal-sdk-go
  observation manager.

- [x] T049 [US2] Update tests: `TestToPVPResult` to verify
  CheckID grouping produces 1 observation with 2 subjects.
  Add `TestToPVPResult_MultipleChecks` for 2 repos with 2
  distinct checks. Update `TestGetResults_MultipleSpecs` in
  server tests.

### Documentation

- [x] T050 Create `cmd/ampel-plugin/README.md` covering plugin
  structure, configuration, target format, installation of
  snappy and ampel via `go install`, GITHUB_TOKEN requirement,
  plugin registration, and complytime-demos VM setup.

- [x] T051 Create `cmd/ampel-plugin/docs/STRATEGY.md` covering
  granular policy approach, multi-target scanning, value of
  complyctl integration, and next actions (Gemara2Ampel update,
  plugin API update, Gemara results).

- [x] T052 Update speckit files (spec.md, plan.md, tasks.md,
  data-model.md, quickstart.md, research.md, contracts/) to
  reflect all post-initial implementation changes.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all
  user stories
- **US1 Generate (Phase 3)**: Depends on Phase 2 completion
- **US2 Scan (Phase 4)**: Depends on Phase 2 completion (can
  run parallel with Phase 3 if staffed)
- **US3 Tool Validation (Phase 5)**: Depends on Phase 2; can
  run parallel with Phases 3-4
- **US4 Config Location (Phase 6)**: Depends on Phase 3
  (Generate) and Phase 4 (GetResults) being implemented
- **Polish (Phase 7)**: Depends on all previous phases

### User Story Dependencies

- **US1 (P1)**: Depends only on Foundational. No dependency on
  other stories.
- **US2 (P1)**: Depends only on Foundational. Can run parallel
  with US1. However, real end-to-end test requires US1 first
  (need generated policy to scan).
- **US3 (P2)**: Depends only on Foundational. Integrates into
  server.go after US1/US2 methods are implemented.
- **US4 (P2)**: Enhancement to existing config; depends on US1
  and US2 being functional.

### Within Each User Story

- Test fixtures before tests
- Tests before or alongside implementation (flexible per
  constitution)
- Core logic before server integration
- Server integration before server tests

### Parallel Opportunities

- Phase 1: T002, T003, T004 are all [P]
- Phase 2: T005, T006 are [P] (T007 depends on both)
- Phase 3: T008, T009, T010, T011 all [P] (fixtures)
- Phase 4: T018, T019 are [P] (fixtures); T020, T021, T022
  are [P] (tests in different packages)
- Phase 5: T028, T029 are [P]
- Phase 7: T035, T036 are [P]

---

## Parallel Example: User Story 1

```bash
# Launch all fixtures in parallel:
Task: T008 "Create full assessment plan fixture"
Task: T009 "Create subset assessment plan fixture"
Task: T010 "Create expected AMPEL policy fixtures"
Task: T011 "Create broader pre-existing policy fixture"

# Then launch tests (depends on fixtures):
Task: T012 "Write conversion unit tests"

# Then implement (can parallelize with tests):
Task: T013 "Implement PolicyToAmpel conversion"
Task: T014 "Implement WritePolicy function"

# Then integrate:
Task: T015 "Implement Generate() in server.go"
Task: T016 "Write Generate() server tests"
Task: T017 "Write config unit tests"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL)
3. Complete Phase 3: User Story 1 (Generate)
4. **STOP and VALIDATE**: Generate produces correct AMPEL policy
   from assessment plan. Mock fixture tests pass. Policy
   accuracy verified.
5. Deploy/demo: `complyctl generate` works with AMPEL plugin.

### Incremental Delivery

1. Setup + Foundational → Plugin compiles and registers
2. Add US1 (Generate) → Test policy linkage → Demo generate
3. Add US2 (Scan) → Test end-to-end → Demo full cycle
4. Add US3 (Tool Validation) → Better error UX
5. Add US4 (Config Location) → Custom deployments supported
6. Polish → Production ready

### Parallel Team Strategy

With multiple developers:
1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: US1 (Generate)
   - Developer B: US2 (Scan — fixtures and package code)
   - Developer C: US3 (Tool Validation)
3. US4 after US1+US2 merge
4. Polish as final pass

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Each user story is independently completable and testable
- All test fixtures are designed so that modifying the input
  assessment plan and re-running tests shows how the AMPEL
  policy changes accordingly (per user requirement)
- Zero new dependencies: all imports from existing go.mod
- Commit after each task or logical group
