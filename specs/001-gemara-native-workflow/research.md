# Research: Gemara-Native Decoupled Workflow

**Branch**: `001-gemara-native-workflow` | **Date**: 2026-02-14

## R1: OCI Registry Client Library

**Decision**: Use `oras.land/oras-go/v2` for all OCI registry interactions.

**Rationale**: Already vendored in `go.mod`. Provides `remote.Repository` with `PlainHTTP`, `FetchReference`, and `Resolve` for both production HTTPS and local HTTP registries. Eliminates need for custom HTTP clients.

**Alternatives considered**:
- `google/go-containerregistry`: Heavier footprint, overlapping functionality with oras-go
- `cuelang.org/go/mod/modregistry`: CUE-specific; adds unnecessary dependency chain
- Custom HTTP client: Violates Constitution V (Do Not Reinvent the Wheel)

## R2: Gemara Library

**Decision**: Use `github.com/gemaraproj/go-gemara v0.0.1` as primary library for parsing, validating, and merging Gemara artifacts (FR-001).

**Rationale**: Already vendored. Provides `GuidanceCatalog`, `ControlCatalog`, `Policy`, `AssessmentLog`, `EvaluationLog`, and `Result` types. The `gemaraconv` subpackage provides SARIF conversion. The `loaders` package handles YAML deserialization.

**Alternatives considered**:
- Raw YAML parsing: Would duplicate type definitions and miss validation
- Custom types: Violates Constitution V; go-gemara is the canonical Go implementation

## R3: OSCAL Output Types

**Decision**: Use `github.com/defenseunicorns/go-oscal v0.7.0` for OSCAL Assessment Results output (FR-014).

**Rationale**: Already vendored. Provides `oscal-1-1-3` types (`AssessmentResults`, `Finding`, `Observation`, `Metadata`) and UUID generation via `src/pkg/uuid`. Spec clarification (Session 2026-02-14 Q2) confirms removal of `oscal-sdk-go` and use of `go-oscal` directly, with a note to eventually upstream OSCAL generation to `go-gemara`.

**Alternatives considered**:
- `oscal-sdk-go`: Removed per spec clarification — larger framework with unnecessary C2P coupling
- Custom OSCAL structs: Violates Constitution V; go-oscal provides validated types

## R4: gRPC Plugin Framework

**Decision**: Use `github.com/hashicorp/go-plugin v1.7.0` for gRPC plugin lifecycle management.

**Rationale**: Already vendored. Battle-tested Go plugin framework using subprocess isolation with gRPC transport. Handles plugin discovery, health checking, and graceful shutdown. Used by Terraform, Vault, and other HashiCorp tools.

**Alternatives considered**:
- Raw gRPC: Would require custom subprocess management, health checking, and cleanup
- Shared libraries (`.so`): Platform-specific, no isolation, crash propagation risk

## R5: Protobuf Code Generation

**Decision**: Use `buf` for protobuf code generation, linting, and breaking change detection (FR-016).

**Rationale**: Spec explicitly requires buf. Already configured (`buf.yaml`, `buf.gen.yaml` present in repo root). Contract defined in `specs/001-gemara-native-workflow/contracts/plugin.proto`.

**Alternatives considered**:
- `protoc` directly: Less ergonomic, no built-in linting or breaking change detection
- `connect-go`: Different RPC framework; hashicorp/go-plugin requires standard gRPC

## R6: Docker Credential Resolution (Updated Session 2026-02-14d)

**Decision**: Use `oras.land/oras-credentials-go` for Docker credential resolution. Delete all custom auth code from `internal/registry/auth.go`.

**Rationale**: The custom implementation (~130 lines) reinvents `credHelpers` → `credsStore` → `auths` chain already provided by `oras-credentials-go`. The `FIXME` comment in `auth.go` acknowledges this. `oras-credentials-go` is maintained by the ORAS project (same team as `oras-go/v2`), returns `auth.CredentialFunc` compatible with the existing `auth.Client`, and handles edge cases (credential helper protocol, config path resolution) that the custom code may not cover. Aligns with Constitution V (Do Not Reinvent the Wheel).

**Migration scope**:

| Component | Before | After |
| :--- | :--- | :--- |
| `internal/registry/auth.go` | ~130 lines: `Authenticator` struct, `AuthConfig` interface, `OrasAuthConfig`, `loadDockerCredentials()`, `readDockerConfig()`, `queryCredHelper()`, `resolveInlineAuth()` | ~15 lines: `NewCredentialFunc() auth.CredentialFunc` backed by `credentials.NewStoreFromDocker()` |
| `internal/registry/client.go` | `auth *Authenticator` field, `GetAuthConfig()` calls | `credFunc auth.CredentialFunc` field, direct `auth.Client{Credential: credFunc}` creation |
| `get.go`, `generate.go`, etc. | `registry.NewAuthenticator()` | `registry.NewCredentialFunc()` (or inline in `NewClient`) |

**New dependency**: `oras.land/oras-credentials-go` (latest stable)

**Alternatives considered**:
- Keep custom implementation: Violates Constitution V; duplicates upstream functionality with less coverage
- `github.com/docker/cli/cli/config`: Pulls in Docker CLI internals; heavier dependency chain
- `github.com/google/go-containerregistry/pkg/authn`: Different ecosystem; adds cross-dependency between oras-go and go-containerregistry

## R7: Removal Scope — Old C2P/OSCAL Code

**Decision**: Remove all `compliance-to-policy-go/v2` and `oscal-sdk-go` imports and code paths per spec Session 2026-02-14 clarifications.

**Rationale**: The Gemara workflow fully replaces the old C2P workflow. Keeping dual code paths adds maintenance burden and confusion. The spec explicitly states removal of `plan`, `info` commands, old `generate`/`scan` paths, and `internal/complytime` C2P-specific logic.

**Files/packages to remove or gut**:

| Target | Action |
| :--- | :--- |
| `cmd/complyctl/cli/plan.go` + test | Delete entirely |
| `cmd/complyctl/cli/info.go` + test | Delete entirely |
| `cmd/complyctl/option/common.go` ComplyTime struct | Remove C2P-specific `ToPluginOptions` and `FrameworkID`. Keep `Common` struct and `UserWorkspace` path helpers for directory resolution. Rename `ComplyTime` → `Options` if it no longer has C2P semantics. |
| `internal/complytime/plugins.go` + test | Delete (C2P plugin management) |
| `internal/complytime/scan.go` + test | Delete (C2P scan logic) |
| `internal/complytime/plan.go` + test | Delete (OSCAL plan R/W) |
| `internal/complytime/catalogs.go` + test | Delete (OSCAL catalog loading) |
| `internal/complytime/controls.go` + test | Delete (OSCAL control resolution) |
| `internal/complytime/scope.go` + test | Delete (assessment scope logic) |
| `internal/complytime/configuration.go` | Gut C2P functions (`Config`, `ActionsContextFromPlan`, `FindComponentDefinitions`, `replacePlaceholdersInPlan`). Keep `EnsureUserWorkspace`, `NewApplicationDirectory`, path helpers |
| `cmd/complyctl/cli/generate.go` | Remove old `runGenerate` C2P path; make `--policy-id` required |
| `cmd/complyctl/cli/scan.go` | Remove old `runScan` C2P path; make `--policy-id` required |
| `cmd/complyctl/cli/list.go` | Remove old framework listing path; always Gemara mode |
| `cmd/complyctl/cli/root.go` | Remove `planCmd`, `infoCmd` from `AddCommand` |
| `go.mod` | Remove `compliance-to-policy-go/v2`, `oscal-sdk-go` |

## R8: Workspace Configuration Pattern

**Decision**: YAML-based workspace config at `./complytime.yaml` (default) with `registry`, `policies`, and `targets` sections.

**Rationale**: Already implemented in `internal/config/`. Convention over configuration per Constitution VII. The config structure maps directly to FR-003 and FR-024 requirements.

## R9: Atomic Cache Updates

**Decision**: Use `oras-go/v2/content/oci` OCI Layout store for atomic cache mutations (FR-006). Supersedes previous write-to-temp-then-rename pattern per R17.

**Rationale**: The OCI Layout store handles content-addressable blob storage with built-in digest verification. `oras.Copy()` performs atomic remote-to-local transfer. The `state.json` file is updated only after successful copy, providing two-phase commit semantics. Previous custom temp-dir-rename pattern replaced to reduce maintenance burden (Constitution V).

## R10: Output Format Strategy

**Decision**: Four output modes — EvaluationLog (default), OSCAL, Markdown, SARIF.

**Rationale**: Per FR-012/FR-014/FR-025/FR-026/FR-028. EvaluationLog always produced (separate file). Formatted reports only when `--format` specified. SARIF delegates to `go-gemara/gemaraconv.ToSARIF`. OSCAL uses `go-oscal` types. Markdown is custom with optional EvaluationLog embedding.

## R11: FR-011 — "Check Functions" Terminology (Session 2026-02-15)

**Decision**: "Check functions" in FR-011 are validation requirement checks registered by plugins. The system routes assessment plan requirement IDs to the correct plugin evaluator via `GroupByEvaluator()`.

**Rationale**: Plugins register checks for specific requirement IDs. The existing `internal/policy/assessment.go` already implements this routing via `ExtractAssessmentConfigs()` and `GroupByEvaluator()`. No new abstraction needed.

## R12: Local Overrides Scope (Session 2026-02-15)

**Decision**: Local overrides are restricted to parameter selection from `Policy.adherence.assessment-plans.parameters.accepted-values`. No manual cache edits or locally authored layer files.

**Rationale**: Constrains the override surface to what the policy author explicitly allows. Prevents cache corruption from manual edits. Validation error returned if selected value is not in the accepted-values list.

## R13: Policy Validator Deferral (Session 2026-02-15)

**Decision**: `internal/policy/validator.go` remains a stub. Defer full validation to future `go-gemara` integration.

**Rationale**: `go-gemara` is at v0.0.1. Validation should be a library responsibility, not duplicated in `complyctl`. The stub provides the interface for future integration. Registry/resolver validation tasks (T029, T030) are included for this branch.

## R14: Success Criteria Benchmarking Deferral (Session 2026-02-15)

**Decision**: SC-001 through SC-007 are aspirational acceptance targets. This branch delivers functional correctness only.

**Rationale**: Numeric thresholds require dedicated test infrastructure (load testing, network simulation). Premature optimization. SC-008 (atomic cache) is validated functionally via FR-006.

## R15: No "Offline Mode" Concept (Session 2026-02-15)

**Decision**: Remove all "offline mode" language. Cached OCI artifacts are local resources. No special mode, flag, or warning.

**Rationale**: The cache IS the local store. `complyctl list` always reads from cache. `complyctl get` errors if registry unreachable. Scans use whatever is cached. No behavioral mode change needed.

## R16: Non-Interactive Init — Static Config Convention (Session 2026-02-16, updated Session 2026-02-23e)

**Decision**: ~~`complyctl init --config <path>` supports both interactive and non-interactive modes.~~ (superseded Session 2026-02-23e). `complytime.yaml` is a static convention in the workspace root (like `go.mod`). No `--config` flag. `complyctl init` errors if `complytime.yaml` already exists (like `go mod init`). CI/CD path: commit `complytime.yaml` to the repo, run `complyctl doctor` and `complyctl get` directly.

**Rationale**: CI/CD pipelines require non-interactive operation. The config file IS the input — users commit `complytime.yaml` alongside their code (same pattern as `go.mod`). `init` is strictly for first-time setup. `go mod init` errors on existing `go.mod` — same principle. Convention over configuration. Aligns with Constitution VII.

## R17: OCI Layout Cache Migration (Session 2026-02-14b)

**Decision**: Migrate local policy cache from custom filesystem layout to `oras-go/v2/content/oci` OCI Layout store with `oras.Copy()` for remote-to-local sync.

**Rationale**: The custom cache (`internal/cache/cache.go`, `sync.go`) implements ~150 lines of manual temp-dir-rename atomicity, per-layer directory creation, and manifest storage. The oras-go `content/oci` package provides all of this out of the box via OCI Image Layout (`index.json`, `oci-layout`, `blobs/sha256/`). `oras.Copy()` handles remote-to-local transfer with built-in digest verification and content-addressable deduplication. Aligns with Constitution V (Do Not Reinvent the Wheel) — delegating blob storage to upstream reduces maintenance burden.

**Migration scope**:

| Component | Before | After |
| :--- | :--- | :--- |
| `internal/cache/cache.go` | Custom `EnsurePolicyDir`, `StoreManifest`, `StoreLayer` (97 lines) | OCI Layout store wrapper: `oci.New(path)`, tag/resolve (≈40 lines) |
| `internal/cache/sync.go` | Custom temp-dir-rename, `downloadPolicyToDir` (147 lines) | `oras.Copy()` from remote to local store (≈60 lines) |
| `internal/cache/state.go` | JSON state file (95 lines) | Unchanged — oras tracks blobs but not policy-level metadata |
| `internal/policy/loader.go` | `os.ReadDir`/`os.ReadFile` on flat dirs | OCI descriptor resolution via `store.Resolve()` + `store.Fetch()` |
| Cache layout | `{id}/{version}/{layer}/{file}.json` | `{id}/index.json`, `{id}/oci-layout`, `{id}/blobs/sha256/...` |

**New vendor packages**: `oras.land/oras-go/v2/content/oci` (OCI Layout store), top-level `oras.land/oras-go/v2` for `oras.Copy()`.

**Alternatives considered**:
- Keep custom filesystem layout: Simpler loader but maintains ~150 lines of custom atomicity/storage code. Human-readable cache dirs useful for debugging but not essential.
- Hybrid (OCI Layout + symlinks): Added complexity with no clear benefit over pure OCI Layout.
- `content/file`: File-based store without OCI Layout structure — loses manifest/descriptor tracking.

## R18: gRPC vs net/rpc Plugin Protocol (Session 2026-02-14c)

**Decision**: Keep gRPC as the plugin protocol. Do not switch to `net/rpc`.

**Rationale**: gRPC provides type-safe protobuf contracts, cross-language plugin authoring (plugins can be written in any language with gRPC support), and streaming capability. `buf` provides codegen, linting, and breaking-change detection. The codegen toolchain is a one-time setup cost — proto files rarely change after initial API stabilization. `hashicorp/go-plugin` recommends gRPC as the preferred protocol.

**Alternatives considered**:
- `net/rpc` (Go stdlib): Simpler (no proto files, no codegen), but Go-only. No cross-language plugin support, weaker contract enforcement via `encoding/gob`.
- gRPC without `buf` (raw `protoc`): Less ergonomic, no built-in linting or breaking-change detection.

## R19: Plugin Manifest Removal (Session 2026-02-14c)

**Decision**: Remove YAML sidecar manifest files entirely. Plugin registration uses executable naming convention (`complyctl-provider-*`) plus HealthCheck RPC only.

**Rationale**: The manifest duplicated information already derivable at runtime. Evaluator ID comes from the executable filename (FR-029). Plugin health and version come from HealthCheck RPC (FR-030). Removing manifests eliminates a file that plugin authors must maintain in sync with the binary — simplest possible plugin authoring experience.

**What was in the manifest**:

| Field | Replacement |
| :--- | :--- |
| `evaluator_ids` | Derived from executable filename (`complyctl-provider-X` → evaluator `X`) |
| `checksum` (SHA256) | Removed (see R20) |
| `version` | Reported via HealthCheck RPC response |
| `capabilities` | Not needed — all plugins implement the same `Plugin` gRPC service |
| `configuration` | Not needed for discovery; plugin-specific config passed via target variables |

**Code impact**: Delete `manifest.go`. Remove `ManifestPath` from `PluginInfo`. Remove `Manifest` from `Plugin` struct. Remove `LoadManifest()` call from `LoadPlugins()`.

**Alternatives considered**:
- Keep manifests as-is: Plugin authors must maintain YAML sidecar alongside binary. No clear benefit over runtime derivation.
- Slim manifest (checksum only): Still requires a sidecar file with no security value (see R20).
- Embed metadata in binary via extended HealthCheck: Considered but current HealthCheck already provides version. No additional metadata needed.

## R20: Checksum Verification Threat Model (Session 2026-02-14c)

**Decision**: Remove SHA256 checksum verification of plugin binaries. Plugin trust deferred to future code-signing feature.

**Rationale — Threat model analysis**:

| Threat | Mitigation by Checksum | Assessment |
| :--- | :--- | :--- |
| Binary tampering (attacker replaces plugin) | Checksum in manifest detects mismatch | **Ineffective** — manifest and binary share same directory/permissions. Attacker replaces both. |
| Supply chain attack (compromised download) | Checksum verifies integrity | **Ineffective** — manifest is unsigned YAML, not from a trusted separate channel. |
| Accidental corruption (bit rot) | Checksum detects corruption | **Marginal** — filesystem integrity already handles this. |

`hashicorp/go-plugin` `SecureConfig` is designed for checksums from a *trusted, separate channel* (e.g., embedded in host binary, fetched from signed registry). Colocated unsigned manifest provides zero security boundary. Net security gain: zero against tampering.

**Code impact**: Remove `SecureConfig` block from `ClientConfig`. Remove `crypto/sha256` and `encoding/hex` imports from `initialization.go`. Remove `Checksum` field from `Manifest` struct (struct itself deleted per R19).

**Alternatives considered**:
- Keep checksum with manifest: No security value per threat model above.
- Re-introduce checksum from trusted source (embed in config, fetch from registry): Adds complexity for marginal gain without signatures.
- Code signing (cosign, Sigstore): Proper solution but out of scope for this branch. Deferred to future feature.

## R21: Simplified NewClient Signature (Session 2026-02-14c)

**Decision**: Simplify `NewClient` to `(executablePath string, logger hclog.Logger)`. Remove `Manifest` parameter, `ClientFactoryFunc` type, and `ClientFactory` function.

**Rationale**: With manifests removed (R19) and checksum removed (R20), `ClientFactory` has no manifest-specific work to do. The factory indirection exists solely to inject the manifest into `ClientConfig`. Without it, `NewClient` can directly create the `go-plugin.Client` with just the executable path and a logger.

**Before** (3 parameters + factory indirection):
```go
func NewClient(executablePath string, manifest Manifest, clientFN ClientFactoryFunc) (*Client, error)
```

**After** (2 parameters, direct creation):
```go
func NewClient(executablePath string, logger hclog.Logger) (*Client, error)
```

**Code impact**: Rewrite `initialization.go` — remove `ClientFactoryFunc`, `ClientFactory()`, inline `go-plugin.Client` creation in `NewClient`. Update `Manager` to remove `clientFactory` field. Update `LoadPlugins()` to call `NewClient(path, logger)` directly.

**Alternatives considered**:
- Keep factory pattern for testability: Factory was used to inject mock client creation, but tests now use `RegisterPluginForTest()` via Go's `export_test.go` pattern — factory not needed for test injection.

## R22: Scan RPC Contract — Requirement IDs Only (Session 2026-02-14d)

**Decision**: Scan RPC sends only requirement IDs and targets. No assessment plan configuration (plan ID, parameters) in the Scan request.

**Rationale**: The plugin was configured during `complyctl generate` and retains that state. Sending the full assessment configuration again is redundant and creates a risk of configuration drift between generate and scan. The existing `ScanRequest` protobuf message already reflects this: `repeated Target targets` + `repeated string requirement_ids`. No change to the contract needed — this decision confirms current implementation is correct.

**Alternatives considered**:
- Include full assessment configuration in ScanRequest: Enables stateless plugins but adds complexity and drift risk.
- Include plan ID only (no parameters): Half-measure with no clear benefit over current design.

## R23: Parameter Validation at Generate Time (Session 2026-02-14d)

**Decision**: `complyctl generate` validates parameter override values against `Policy.adherence.assessment-plans.parameters.accepted-values` before sending configuration to plugins. Invalid values produce a validation error naming the parameter and listing accepted values.

**Rationale**: Plan already resolves the policy graph, so accepted values are available. Failing fast prevents sending invalid configuration to plugins. Aligns with edge case table: "If a selected value is not in the accepted-values list, return a validation error naming the parameter and listing accepted values."

**Code impact**: Add validation step in `plan.go` between `applyParameterOverrides()` and plugin invocation. Requires `go-gemara` policy types to expose accepted values, or parse them from the assessment layer content.

**Alternatives considered**:
- Validate at scan time: Defers the error, wastes plugin plan cycle.
- Validate in both: Adds complexity for marginal safety gain.

## R24: Delete Authenticator Struct (Session 2026-02-14d)

**Decision**: Delete `Authenticator` struct, `AuthConfig` interface, and `OrasAuthConfig` wrapper. Replace with a single `NewCredentialFunc() auth.CredentialFunc` backed by `credentials.NewStoreFromDocker()`.

**Rationale**: With `oras-credentials-go` handling the full Docker credential chain (R6 updated), the `Authenticator` struct becomes a pass-through wrapper with no logic of its own. The `AuthConfig` interface and `OrasAuthConfig` type exist solely to bridge the custom credential resolution to the oras `auth.Client`. Removing them simplifies the registry client to accept a `CredentialFunc` directly.

**Code impact**: Rewrite `internal/registry/auth.go` (~130 lines → ~15 lines). Update `internal/registry/client.go` to accept `auth.CredentialFunc` instead of `*Authenticator`. Update callers in `get.go` and `cache/source.go` to use the new API.

**Alternatives considered**:
- Keep `Authenticator` as thin wrapper: No value — it delegates everything to `oras-credentials-go` with zero added logic. Extra indirection for no benefit.

## R25: Multi-Layer Bundled OCI Manifest (Session 2026-02-14e)

**Decision**: Each policy ID maps to a single multi-layer OCI Image Manifest. All Gemara Layers 1-3 (catalogs, guidance, policy/assessments) are separate layers within one manifest. FR-021 confirmed.

**Rationale**: The `gemara-content-service` server's current seed data stores artifacts as separate repos (`catalogs/X`, `policies/X`, `guidance/X`). This is developmental. The server will evolve to produce bundled manifests. A single manifest per policy simplifies complyctl: one `oras.Copy()` call fetches everything needed for a policy. Layer identification uses media type strings on layer descriptors.

**Manifest structure**:
```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "config": {
    "mediaType": "application/vnd.oci.empty.v1+json",
    "digest": "sha256:...",
    "size": 2
  },
  "layers": [
    {"mediaType": "application/vnd.gemara.catalog.v1+yaml", "digest": "sha256:...", "size": ...},
    {"mediaType": "application/vnd.gemara.guidance.v1+yaml", "digest": "sha256:...", "size": ...},
    {"mediaType": "application/vnd.gemara.policy.v1+yaml", "digest": "sha256:...", "size": ...}
  ]
}
```

**Alternatives considered**:
- Separate repos per artifact type: Matches current server seed data but requires multiple fetches, complex dependency tracking, and complicates cache management.
- Policy repo only (plugins handle catalog/guidance): Limits plugin flexibility; complyctl should own the full policy graph.

## R26: Flat OCI Repository Path Mapping (Session 2026-02-14e)

**Decision**: Policy ID is the OCI repository name directly. No prefix or path transformation. `rhel11` → `registry.example.com/rhel11`.

**Rationale**: FR-002 already specifies flat mapping. Server will use flat repo names for bundled manifests. Simplest possible URL construction — no prefix logic, no configuration.

**Alternatives considered**:
- Prefixed (`policies/rhel11`): Matches current server seed data but adds unnecessary path manipulation.
- Configurable base path: Adds configuration complexity for marginal flexibility.

## R27: YAML Content Format (Session 2026-02-14e)

**Decision**: All Gemara layer content is YAML with `+yaml` media type suffix. The `+json` reference in Session 2026-02-13 was incorrect.

**Rationale**: The `gemara-content-service` server produces YAML content. `go-gemara/loaders` parses YAML. Gemara artifacts are authored in YAML. All three canonical media types use `+yaml`: `application/vnd.gemara.catalog.v1+yaml`, `application/vnd.gemara.policy.v1+yaml`, `application/vnd.gemara.guidance.v1+yaml`.

**Code impact**: Define three media type constants in `internal/complytime/consts.go`. Policy loader (`internal/policy/loader.go`) iterates manifest layers and matches by media type string to determine layer type. No JSON parsing path needed.

## R28: oras.Copy() Transparent Digest Handling (Session 2026-02-14e)

**Decision**: `oras.Copy()` handles manifest Content-Type negotiation and digest computation transparently. No special handling needed in complyctl.

**Rationale**: `oras.Copy()` sends the standard OCI `Accept` header and computes digests client-side from the response body. The server's lack of `Docker-Content-Digest` header is immaterial — oras-go does not depend on it. FR-004 updated to reflect client-side digest computation.

## R29: ConfidenceLevel Enum — Proto Type Change (Session 2026-02-23)

**Decision**: Change proto `confidence` field from `double` (0.0-1.0) to `ConfidenceLevel` enum with values `CONFIDENCE_LEVEL_NOT_SET`, `CONFIDENCE_LEVEL_UNDETERMINED`, `CONFIDENCE_LEVEL_LOW`, `CONFIDENCE_LEVEL_MEDIUM`, `CONFIDENCE_LEVEL_HIGH`. 1:1 mapping with `go-gemara` `ConfidenceLevel` type. Proto package stays at `complyctl.plugin.v1` (pre-release break).

**Rationale**: `go-gemara` defines `ConfidenceLevel` as an `int` enum — not a float. The original `double confidence` was a placeholder that didn't match the domain model. The `FIXME(jpower432)` comment in `cmd/openscap-plugin/server/server.go` acknowledged this mismatch. Using a float introduces lossy semantics (what does 0.7 mean vs 0.8?). The enum provides discrete, well-defined levels with clear string serialization for YAML/JSON output.

**Code impact**:

| Component | Before | After |
| :--- | :--- | :--- |
| `contracts/plugin.proto` | `double confidence = 4` | `ConfidenceLevel confidence = 4` + new enum definition |
| `pkg/plugin/client.go` | `Confidence float64` | `Confidence ConfidenceLevel` (new type mapping go-gemara enum) |
| `pkg/plugin/server.go` | `Confidence: a.Confidence` (float64) | `Confidence: a.Confidence` (enum) |
| `cmd/openscap-plugin/server/server.go` | `Confidence: 1.0` / `Confidence: 0` | `Confidence: plugin.ConfidenceLevelHigh` / `Confidence: plugin.ConfidenceLevelNotSet` |
| `internal/output/evaluator.go` | float64 → string conversion | enum → go-gemara `ConfidenceLevel` direct mapping |

**Alternatives considered**:
- Keep float, convert at output boundary: Lossy — float semantics don't match domain. Every consumer must interpret the scale.
- String field: Loses proto type safety. Enum provides compile-time validation.

## R30: oscalPolicy Removal — OpenSCAP Plugin Refactoring (Session 2026-02-23)

**Decision**: Delete `policytype` package (`cmd/openscap-plugin/policytype/types.go`). Remove `oscalPolicy` global variable. Refactor all tailoring/scan functions to use `[]plugin.AssessmentConfiguration` as the data source for rules and parameters. Rename all "OSCAL policy" error messages and comments to "assessment configuration."

**Rationale**: The `oscalPolicy` variable and `policytype.Policy` type are remnants of the `oscal-compass/compliance-to-policy-go` integration. The Gemara-native workflow replaces this with `AssessmentConfiguration`, which already carries requirement IDs (mapping to XCCDF rule short names in the OpenSCAP model) and parameters (mapping to XCCDF variables). Functions like `PolicyToXML`, `unselectAbsentRules`, `selectAdditionalRules`, `updateTailoringValues` already accept `[]plugin.AssessmentConfiguration` but ignore it — using `oscalPolicy` instead. The refactoring makes these functions actually use their parameter.

**Mapping (code-level — policytype removal)**:

| policytype.Policy field | AssessmentConfiguration equivalent |
| :--- | :--- |
| `RuleSet.Rule.ID` | `RequirementID` (1:1 in OpenSCAP; requirement ID = XCCDF rule short name) |
| `RuleSet.Rule.Parameters[].ID` | Key in `Parameters` map |
| `RuleSet.Rule.Parameters[].Value` | Value in `Parameters` map |
| `RuleSet.Checks[].ID` | Derived from `RequirementID` (OpenSCAP: OVAL check short name = rule short name) |

**Conceptual mapping (OSCAP → Gemara, Session 2026-02-25)**:

| OSCAP Concept | Gemara Artifact | Layer |
| :--- | :--- | :--- |
| XCCDF Rule | Control + Assessment Requirement | Layer 2 (Control Catalog) |
| OVAL Check | Assessment Plan | Layer 3 (Policy) |
| CIS ID (`cis-*`) | Guideline-mapping on Control | Layer 2 → Layer 1 |

**Code impact**: Delete `cmd/openscap-plugin/policytype/types.go`. Update `tailoring.go` (~12 references). Update `tailoring_test.go` (~15 references). Update `server.go` (2 references). Remove `policytype` import from all files.

**Alternatives considered**:
- Create new intermediate type: Adds unnecessary abstraction. AssessmentConfiguration provides all needed data.
- Keep policytype under a new name: Duplicates AssessmentConfiguration fields. Violates Constitution I (Single Source of Truth).

## R31: Parameters vs Vars — Plugin Data Flow Distinction (Session 2026-02-23)

**Decision**: Formalize two distinct data channels in the plugin protocol. Parameters: test configuration from Layer 3 Gemara policy, passed via `AssessmentConfiguration.parameters` in Generate RPC. Vars: plugin-specific runtime variables (auth tokens, connection strings, endpoints), passed via `Target.variables` in Scan RPC. ~~Plugins do not advertise expected vars through the protocol — var documentation is out-of-band (plugin README/quickstart).~~ (Superseded R51: providers now declare required variable *names* via `HealthCheckResponse.required_global_variables` and `required_target_variables`; documentation of valid *values* remains out-of-band.)

**Rationale**: Parameters and vars serve different purposes at different lifecycle stages. Parameters are authored by policy authors and define how a test should be configured (e.g., `var_password_hashing_algorithm: SHA512` in OpenSCAP). Vars are provided by system administrators and define how to connect to the target (e.g., `kubeconfig: /path/to/kubeconfig` for a Kubernetes plugin). Conflating them creates confusion about where configuration lives and who owns it. Adding a protocol mechanism to advertise vars (e.g., `GetCapabilities` RPC) adds protocol complexity for minimal runtime benefit — vars are inherently environment-specific and change per deployment.

**Data flow**:

| Channel | Source | Lifecycle | RPC | Owner |
| :--- | :--- | :--- | :--- | :--- |
| Parameters | Layer 3 Gemara policy `assessment-plans.parameters` | `complyctl generate` | `GenerateRequest.configurations[].parameters` | Policy author |
| Vars | Workspace config `targets[].variables` | `complyctl scan` | `ScanRequest.targets[].variables` | System admin |

**Alternatives considered**:
- Extend HealthCheck response with `expected_vars`: Adds protocol complexity, requires schema evolution for every new var.
- Add `GetCapabilities` RPC: Was already removed in Session 2026-02-14c. Re-adding for vars contradicts the simplification decision.
- Merge parameters and vars: Different owners, different lifecycles, different RPCs. Merging loses the semantic distinction.

## R32: Per-Assessment-Plan Evaluator Routing (Session 2026-02-23b)

**Decision**: When a Gemara Policy defines multiple evaluation methods, route each assessment plan to the plugin identified by that plan's `evaluation-methods[].executor.id`. No single policy-level evaluator — routing is per-assessment-plan.

**Rationale**: Heterogeneous policies (e.g., a CIS benchmark evaluated partly by OpenSCAP and partly by a Kubernetes evaluator) require per-plan routing. A single top-level evaluator would force policy authors to split logically unified policies across multiple artifacts. Per-plan routing keeps the policy cohesive while allowing the runtime to dispatch requirements to different plugins.

**Code impact**:

| Component | Before | After |
| :--- | :--- | :--- |
| `internal/policy/resolver.go` | Single `EvaluatorID` per policy | Per-assessment `EvaluatorID` from `plan.evaluation-methods[].executor.id` |
| `internal/policy/assessment.go` | `GroupByEvaluator()` groups by policy-level evaluator | `GroupByEvaluator()` groups by per-assessment evaluator ID |
| `internal/plugin/manager.go` | One `Generate()` call per policy | N `Generate()` calls per policy (one per evaluator group) |
| Edge case | — | Unmatched evaluator ID → error listing available plugins |

**Alternatives considered**:
- Single top-level evaluator per policy: Forces policy splitting for multi-tool environments.
- Plugin self-selection (plugins filter requirements they handle): Requires plugins to understand policy structure. Violates separation of concerns.

## R33: `complyctl plugins` Command (Session 2026-02-23b)

**Decision**: Add `complyctl plugins` as a top-level command showing discovered plugins from `~/.complytime/providers/`. Displays evaluator ID, executable path, health status, and version. Symmetric with `complyctl list` (policies).

**Rationale**: Admins need visibility into which plugins are installed before running `complyctl generate` or `scan`. Without this, a missing plugin produces a cryptic "unmatched evaluator ID" error at generate/scan time. The `plugins` command provides a pre-flight check. Symmetry with `list` (policies) and `plugins` (evaluators) makes the CLI discoverable.

**Code impact**: New file `cmd/complyctl/cli/plugins.go`. Calls `plugin.Manager.DiscoverPlugins()` (existing) and formats output as a table. Register `pluginsCmd` in `root.go`.

**Alternatives considered**:
- `complyctl list --plugins` subcommand flag: Overloads `list` with two concerns (policies vs plugins). Less discoverable.
- No command (rely on error messages): Poor UX — admin must trigger a failure to learn what's missing.

## R34: `generate` Command with Digest-Based Freshness (Session 2026-02-23b, updated)

**Decision**: Keep `complyctl generate` as the explicit translation command. `generate` resolves the dependency graph, invokes plugins via Generate RPC, persists generated artifacts, records the policy cache digest at generation time, and outputs a tabular execution plan. `scan` is smart about reuse: (1) no artifacts → auto-generate; (2) fresh artifacts → reuse; (3) stale (digest mismatch) → warn and auto-regenerate. `scan --dry-run` provides a pre-flight preview without executing checks.

**Rationale**: AI-driven generation is near-term — expensive, non-deterministic, requires human review. Collapsing generate into scan would force regeneration on every scan (wasteful for AI). Keeping generate explicit enables: generate once, review, scan many times. Smart scan solves the staleness problem (user pulls new policy via `get` but doesn't re-generate): scan detects digest mismatch and auto-regenerates with a warning.

**Evolution**: Originally `generate` → renamed to `plan` (earlier in Session 2026-02-23b) → `plan` collapsed into `scan --dry-run` → then `generate` restored as separate command with digest-based freshness after recognizing near-term AI generation needs.

**Code impact**:

| Component | Before | After |
| :--- | :--- | :--- |
| `cmd/complyctl/cli/generate.go` | `generateCmd` (existing) | Stays as `generateCmd`; add digest persistence + execution plan output |
| `internal/policy/generation_state.go` | (new) | Stores `GenerationState` (policy digest, timestamp, evaluator IDs) |
| `cmd/complyctl/cli/scan.go` | Calls Scan RPC directly | Checks generation freshness first; auto-generates if needed |
| Proto comments | "Called during complyctl generate" | "Called during complyctl generate (or auto-generate within scan)" |

**Alternatives considered**:
- `plan` as standalone command: Collapsed into scan because it doesn't feed state. Brought back `generate` instead because AI generation needs explicit control.
- Scan-only (no separate generate): Forces regeneration every scan. Wasteful for expensive AI generation.
- Scan with `--skip-generate`: Inverts the default — admin must opt out rather than opt in for reuse.

## R35: `plugin_variables` → `evaluator_config` (Session 2026-02-23b)

**Decision**: Replace `plugin_variables` (flat `map[string]string` in `complytime.yaml`) with `evaluator_config` — a nested map keyed by evaluator ID under `PolicyConfig`. Each evaluator's config is a `map[string]string`.

**Rationale**: `plugin_variables` was a flat bag with no indication of which plugin consumed which key. With per-assessment-plan routing (R32), a policy may use multiple evaluators, each requiring different config. Keying by evaluator ID makes ownership explicit: `evaluator_config: {openscap: {profile: cis_workstation_l1}, kube: {kubeconfig: /path}}`.

**Code impact**:

| Component | Before | After |
| :--- | :--- | :--- |
| `internal/complytime/config.go` | `PluginVariables map[string]string` | `EvaluatorConfig map[string]map[string]string` |
| `api/plugin/plugin.proto` | `map<string, string> plugin_variables = 1` | `map<string, string> evaluator_config = 1` |
| `cmd/complyctl/cli/generate.go` | Passes flat map | Looks up `evaluator_config[evaluatorID]` per group |
| `cmd/openscap-plugin/server/server.go` | `req.PluginVariables` | `req.EvaluatorConfig` |
| `complytime.yaml` | `plugin_variables:` | `evaluator_config:` |

**Alternatives considered**:
- Keep `plugin_variables` with naming convention (prefix keys with evaluator ID): Fragile string parsing. No structural guarantee of ownership.
- Top-level `evaluator_config` outside `PolicyConfig`: Config is policy-scoped — keeping it under `PolicyConfig` matches the data model.

## R36: Tabular Execution Plan Format (Session 2026-02-23b)

**Decision**: The execution plan (output by `complyctl generate` and `complyctl scan --dry-run`) contains two tables: (1) evaluator-to-requirements (evaluator ID, requirement count, matched plugin path, health status); (2) target-to-policy (target ID, policy ID, evaluator IDs in scope). Unmatched evaluators show error status.

**Rationale**: Admins need a scannable pre-flight summary — not a verbose dump of every requirement. Two lean tables answer the two key questions: "Which plugins handle what?" and "Which targets are scanned?" Error rows for unmatched evaluators make missing plugins immediately visible.

**Output format** (example):

```text
Execution Plan: cis-fedora-l1-workstation

Evaluator Routing:
  EVALUATOR        REQUIREMENTS  PLUGIN                                       STATUS
  openscap         47            ~/.complytime/providers/complyctl-provider-openscap  healthy
  kube-evaluator   3             (not found)                                     ERROR

Target Scope:
  TARGET           POLICY                          EVALUATORS
  local            cis-fedora-l1-workstation       openscap, kube-evaluator
```

**Code impact**: New function `internal/output/execution_plan.go` → `FormatExecutionPlan()`. Called from `generate` command and `scan --dry-run` after graph resolution and plugin prep.

**Alternatives considered**:
- JSON output: Machine-readable but not scannable by humans. Could be a future `--output json` option.
- Verbose per-requirement listing: Too long for policies with 50+ requirements. Tabular summary is sufficient for pre-flight.

## R37: Generate/Scan Lifecycle with Digest-Based Freshness (Session 2026-02-23b)

**Decision**: `generate` persists a `GenerationState` (policy cache digest, timestamp, evaluator IDs used) alongside generated artifacts. `scan` checks this state before executing:

| Condition | Behavior |
| :--- | :--- |
| No `GenerationState` exists | Auto-generate (resolve graph, invoke Generate RPC, persist state) |
| State exists, digest matches current cache | Reuse generated artifacts (skip Generate RPC) |
| State exists, digest differs (policy updated via `get`) | Warn admin, auto-regenerate, then scan |

**Rationale**: Solves the "get without re-generate" staleness problem raised in spec clarification. Digest comparison is cheap (SHA256 string compare against `state.json` PolicyState.digest). No complex state management — just one comparison before scan decides whether to regenerate.

**GenerationState structure**:

```yaml
# {workspace}/.complytime/generation/{policy-id}.json
# One file per policy — directory created on first generate
{
  "policy_id": "cis-fedora-l1-workstation",
  "policy_digest": "sha256:abc123...",
  "generated_at": "2026-02-23T15:04:05Z",
  "evaluator_ids": ["openscap"]
}
```

**Dry-run persistence**: `scan --dry-run` persists `GenerationState` after invoking Generate RPC. The plugin already created artifacts as side effects (e.g., tailoring XML); persisting state ensures a subsequent `scan` reuses them. "Dry" means "don't execute Scan RPC," not "don't generate."

**Code impact**:

| Component | Change |
| :--- | :--- |
| `internal/policy/generation_state.go` | New file: `GenerationState` struct, `SaveState()`, `LoadState()`, `IsFresh()`. Path: `{workspace}/.complytime/generation/{policy-id}.json` |
| `cmd/complyctl/cli/generate.go` | After Generate RPC, save `GenerationState` with current policy digest |
| `cmd/complyctl/cli/scan.go` | Before Scan RPC, check `IsFresh()`: if stale → warn + auto-generate; if missing → auto-generate; if fresh → reuse. `--dry-run` persists state then exits (no Scan RPC) |

**Alternatives considered**:
- No persistence (scan always regenerates): Wasteful for expensive AI generation. Loses reviewed artifacts.
- File modification timestamps: Fragile — filesystem ops can change mtime without content change.
- Digest in workspace config: Pollutes user-authored config with runtime state.
- Single generation-state.json for all policies: Requires read-modify-write for multi-policy workspaces. Per-policy file avoids contention.
- Dry-run without persistence: Discards Generate RPC work — next scan regenerates needlessly.

## R38: Charmbracelet Rendering for All Tabular Outputs (Session 2026-02-23c)

**Decision**: All tabular CLI outputs (`list`, `plugins`, `generate` execution plan, `scan --dry-run` execution plan) MUST use `charmbracelet/bubbles/table` + `charmbracelet/lipgloss` for rendering. Reuse the existing `internal/terminal` package helpers (`Model`, `ShowPlainTable`). The `--plain` flag is available only on discovery commands (`list`, `plugins`).

**Rationale**: `complyctl list` already uses charmbracelet for styled table output. The `plugins` command uses raw `text/tabwriter` and `internal/output/execution_plan.go` uses `fmt.Fprintf` — both are inconsistent with the established UX pattern. Standardizing on charmbracelet provides a polished, consistent terminal experience across all commands. The `internal/terminal` package already provides reusable helpers (`Model` with `bubbletea`, `ShowPlainTable` for `--plain` mode), minimizing new code.

**Code impact**:

| Component | Before | After |
| :--- | :--- | :--- |
| `cmd/complyctl/cli/plugins.go` | `text/tabwriter` + `fmt.Fprintf` | `charmbracelet/bubbles/table` + `lipgloss` via `internal/terminal`. Add `--plain` flag |
| `internal/output/execution_plan.go` | `fmt.Fprintf` with manual column alignment | Two `bubbles/table` instances (Evaluator Routing + Target Scope) rendered via `lipgloss`. Returns styled string |
| `cmd/complyctl/cli/generate.go` | `fmt.Print(output.FormatExecutionPlan(...))` | Same call, `FormatExecutionPlan` now returns charmbracelet-rendered output |
| `cmd/complyctl/cli/scan.go` | Same as generate for `--dry-run` path | Same change |
| `internal/terminal/table.go` | Existing `Model`, `ShowPlainTable` | No changes needed — already provides the reusable helpers |

**Execution plan tables**: Two separate charmbracelet tables, each with a header label above it. Matches FR-033's explicit two-table structure.

**Alternatives considered**:
- Keep `text/tabwriter` for non-interactive outputs: Inconsistent UX; charmbracelet handles terminal width detection and ANSI gracefully
- Single combined table for execution plan: Loses the two-concern separation (evaluator routing vs. target scope)
- Add `--plain` to all commands: Execution plan is an operational tool, not typically piped — styling-only is sufficient

## R39: parsePolicyLayer — Gemara-Only Format (Session 2026-02-23c)

**Decision**: `parsePolicyLayer` accepts only `gemara.Policy` with `adherence.assessment-plans`. Remove the structured format (`evaluator_id` + `requirements` list) and legacy format (flat list with per-entry `evaluator_id`). Change function signature from `parsePolicyLayer(policyID string, data []byte) policyLayerResult` to `parsePolicyLayer(policyID string, data []byte) (policyLayerResult, error)`. Return an error when YAML does not parse as a valid `gemara.Policy` or when `adherence.assessment-plans` is empty.

**Rationale**: The structured and legacy formats were transitional scaffolding for pre-Gemara development. With Gemara as the only workflow (Session 2026-02-14), all policy layers MUST be valid Gemara Policy YAML. Silent fallback to non-Gemara formats masks data issues and makes debugging hard. Failing fast aligns with the spec's edge case handling pattern and Constitution II (Simplicity & Isolation) — fewer code paths, fewer states.

**Code impact**:

| Component | Before | After |
| :--- | :--- | :--- |
| `internal/policy/resolver.go` `parsePolicyLayer` | 3 format attempts (Gemara → structured → legacy), no error return | 1 format: `gemara.Policy` unmarshal + `extractFromGemaraPolicy`. Returns `(policyLayerResult, error)` |
| `internal/policy/resolver.go` `ResolvePolicyGraph` | `policyLayer := parsePolicyLayer(...)` (ignores format failures) | `policyLayer, err := parsePolicyLayer(...)` — propagates error to caller |
| `internal/policy/resolver.go` structured/legacy structs | ~35 lines of inline struct definitions + unmarshal logic | Deleted |
| Tests | May test structured/legacy parsing | Remove those test cases; add error-path test for invalid YAML |

**Alternatives considered**:
- Return empty result (silent degradation): Masks data issues. Admin wouldn't know their policy layer is malformed until requirements are missing at scan time
- Keep structured as a fallback: Two code paths to maintain. Gemara is the canonical format; structured is a deviation with no defined schema
- Call `extractFromGemaraPolicy` directly from `ResolvePolicyGraph`: Loses the single entry point for YAML unmarshalling. `parsePolicyLayer` provides a clean boundary between raw bytes and typed data

## R40: Two-Tier Terminal Output (Session 2026-02-23d)

**Decision**: Implement two-tier terminal output using explicit `fmt.Fprintf` calls for user-facing progress/summary and the existing `logger` (hclog → charmbracelet/log → file) for structured log events. No `io.MultiWriter` — keep log file and terminal output as separate concerns.

**Rationale**: The current architecture already separates these paths: `logger` writes to `complyctl.log` (file), and `fmt.Println`/`fmt.Printf` calls write to stdout (terminal). The two-tier model formalizes this pattern:

- **Tier 1 (init, get)**: Add `fmt.Fprintf(os.Stderr, ...)` progress messages at key points (per-policy sync status, config validation). Existing `logger.Info(...)` calls remain for structured log events written to file. Stderr used for progress (not stdout) to keep stdout available for machine-parseable output.
- **Tier 2 (list, plugins, generate, scan)**: Already implemented — charmbracelet tables (`fmt.Print(output.FormatExecutionPlan(...))`) and `FormatPreScanSummary` write to stdout. Logger writes to file. No changes needed for existing Tier 2 commands.

**Code impact**:

| Component | Change |
| :--- | :--- |
| `cmd/complyctl/cli/get.go` | Add `fmt.Fprintf(os.Stderr, "Syncing policy %d/%d: %s...\n", i+1, total, policy.ID)` before each `sync.SyncPolicy()` call. Add `fmt.Fprintf(os.Stderr, " done\n")` after success. Replace final `fmt.Println("Synchronization completed.")` with `fmt.Fprintln(os.Stderr, "Synchronization completed.")` |
| `cmd/complyctl/cli/init.go` | Show config creation status (if applicable), delegate to `get` for sync progress, then show doctor diagnostic results (R52, supersedes R50 ordering) |
| `cmd/complyctl/cli/root.go` | No structural change — `logger` stays file-only. Log file created lazily on first write via `lazyLogWriter` (Session 2026-02-23f). No terminal notification |

**Design decisions**:
- Stderr for progress (Tier 1), stdout for summaries (Tier 2): Allows `complyctl list 2>/dev/null` to pipe clean table output. Progress messages are operational, not data.
- No multi-writer: Avoids coupling log verbosity with terminal UX. File logs can be debug-level; terminal progress stays concise.
- No spinner/progress bar library: Per-policy count-based progress (`1/3`, `2/3`) is sufficient for typical workloads (3-5 policies). Spinner adds dependency for marginal UX gain.

**Alternatives considered**:
- `io.MultiWriter(file, os.Stderr)` for logger: Mixes structured log output with terminal UX. File logs contain operational details (digests, versions) that clutter terminal output.
- charmbracelet spinner for sync: Adds visual polish but requires `bubbletea` program lifecycle per command. Overhead exceeds benefit for 3-5 policy syncs.
- Separate `terminalLogger` with different verbosity: Over-engineering — `fmt.Fprintf` calls are explicit and reviewable at each call site.

## R41: Evaluator Config Validation Before Plugin Dispatch (Session 2026-02-23d)

**Decision**: Add `ValidateEvaluatorConfig()` in `internal/policy/assessment.go` that checks all evaluator IDs in `groups` have a corresponding non-nil entry in `graph.EvaluatorConfig`. Call this validation in `generate.go` and `scan.go` after `GroupByEvaluator()` returns but before any `RouteGenerate()` call. Enhance plugin configuration errors in `manager.go` with evaluator ID and config path context.

**Rationale**: The current error path is: `generate` → `RouteGenerate` → plugin `Generate RPC` → plugin's `LoadSettings()` → `missing configuration value for option "workspace"`. The error message names the plugin and the missing field but doesn't tell the admin which workspace config section to fix. Validating at the complyctl layer produces a clear, actionable error: `evaluator "openscap" has no evaluator_config entry in complytime.yaml — add evaluator_config.openscap with required fields (see plugin documentation)`. Plugin-specific field errors are enhanced by wrapping the plugin error with evaluator ID context.

**Code impact**:

| Component | Change |
| :--- | :--- |
| `internal/policy/assessment.go` | New function: `ValidateEvaluatorConfig(groups map[string]EvaluatorGroup, configPath string) error`. Iterates groups; if `group.EvaluatorConfig == nil`, returns error naming the evaluator ID and config path |
| `cmd/complyctl/cli/generate.go` | Add `if err := policy.ValidateEvaluatorConfig(groups, ws.Path()); err != nil { return err }` after `GroupByEvaluator()` |
| `cmd/complyctl/cli/scan.go` | Same validation call after `GroupByEvaluator()` |
| `pkg/plugin/manager.go` `RouteGenerate` | Wrap plugin `Generate` error with evaluator context: `fmt.Errorf("plugin %s (evaluator %q): %w", p.Info.PluginID, evaluatorID, resp.ErrorMessage)` |

**Validation scope**: Existence check only — complyctl validates that a config entry exists for each evaluator. Field-level validation (which keys are required) remains the plugin's responsibility. This avoids coupling complyctl to plugin-specific configuration schemas.

**Alternatives considered**:
- Plugin advertises required config keys via HealthCheck: Adds protocol complexity. Plugins may have context-dependent required fields (e.g., `workspace` required for OpenSCAP but not for a cloud evaluator).
- Hard-coded required fields per evaluator in complyctl: Violates separation of concerns. complyctl is evaluator-agnostic.
- No validation (current behavior): Error message is accurate but not actionable — admin must trace from plugin error to workspace config manually.

## R42: Scan Output Directory Constant (Session 2026-02-24)

**Decision**: Define the scan output directory name as a constant `ScanOutputDir = ".complytime/scan"` in `internal/complytime/consts.go`. All code that references the scan output directory MUST use this constant instead of the inline string `".complytime/scan"`.

**Rationale**: The string `".complytime/scan"` is currently hardcoded inline in `cmd/complyctl/cli/scan.go` (line 248: `outDir := filepath.Join(".", ".complytime/scan")`), referenced in tests (`tests/e2e/e2e_test.go`, `tests/behavioral/audit.go`, `tests/integration_test.sh`), and documented in `docs/E2E_INTEGRATION.md`, `docs/QUICK_START.md`, `README.md`, and the OpenSCAP plugin README. Per Constitution I (Single Source of Truth), centralizing this value prevents divergence — if the directory name changes, updating one constant updates all Go code. Shell scripts and documentation reference the name by convention, but all Go production code must use the constant.

**Code impact**:

| Component | Before | After |
| :--- | :--- | :--- |
| `internal/complytime/consts.go` | (no scan output dir constant) | `const ScanOutputDir = ".complytime/scan"` |
| `cmd/complyctl/cli/scan.go` | `filepath.Join(".", ".complytime/scan")` | `filepath.Join(".", consts.ScanOutputDir)` |
| `internal/output/evaluator.go` | (receives `outDir` parameter — no change) | No change (caller passes the constant) |
| `tests/e2e/e2e_test.go` | `filepath.Join(scanDir, ".complytime/scan")` | `filepath.Join(scanDir, consts.ScanOutputDir)` |
| `tests/behavioral/audit.go` | `filepath.Join(ctx.WorkDir, ".complytime/scan")` | `filepath.Join(ctx.WorkDir, consts.ScanOutputDir)` |

**Alternatives considered**:
- Keep inline string: Violates Constitution I. Six Go files reference the same string — any rename requires shotgun surgery.
- Environment variable: Over-engineering. The directory name is not environment-specific. Convention over configuration (Constitution VII).
- Configurable via `complytime.yaml`: Adds complexity for minimal user benefit. Sensible default is sufficient.

## R43: Post-Scan Summary Table with Emoji Status (Session 2026-02-24)

**Decision**: After every scan execution, `complyctl scan` displays a charmbracelet-styled summary table in the terminal showing per-requirement results with emoji status indicators. Table columns: Requirement ID, Title/Description, Status (emoji). Rows sorted by status priority: ❌ failed → ⚠️ error → ⏭️ skipped → ✅ passed. Aggregate totals line below the table. This table appears regardless of `--format` flag (FR-037).

**Rationale**: The spec previously defined only a one-line pre-scan summary (FR-034) and the EvaluationLog file output (FR-028). Admins had no terminal-visible pass/fail breakdown without opening the EvaluationLog file. The summary table provides immediate actionable feedback: failures are surfaced first, emoji status indicators are recognizable across terminal themes, and the aggregate totals line gives at-a-glance posture assessment. Aligns with FR-035 Tier 2 output model (scan shows "summary + log" to terminal).

**Implementation**:

| Component | Change |
| :--- | :--- |
| `internal/complytime/consts.go` | Add emoji constants: `StatusPassed = "✅"`, `StatusFailed = "❌"`, `StatusSkipped = "⏭️"`, `StatusError = "⚠️"` |
| `internal/output/scan_summary.go` | New file: `FormatScanSummary(assessments []plugin.AssessmentLog, reqTitles map[string]string) string` — builds charmbracelet table with 3 columns (Requirement ID, Title, Status emoji), sorts rows by status priority, appends aggregate totals line. Uses `internal/terminal` helpers for consistent styling |
| `cmd/complyctl/cli/scan.go` | After scan completes (post-`eval.Write()`), call `output.FormatScanSummary()` and print to stdout. The summary table appears after the EvaluationLog is written, before any formatted report |

**Status priority sort order** (highest priority first):

| Priority | Status | Emoji | Semantic |
| :--- | :--- | :--- | :--- |
| 1 | Failed | ❌ | Requirement not satisfied — immediate action needed |
| 2 | Error | ⚠️ | Evaluation failed (plugin error) — investigation needed |
| 3 | Skipped | ⏭️ | Excluded by tailoring — informational |
| 4 | Passed | ✅ | Requirement satisfied — no action |

**Aggregate totals line**: Rendered below the table as a single styled line: `Total: 44 ✅, 3 ❌, 2 ⏭️, 1 ⚠️`

**Requirement title source**: Titles extracted from the resolved `DependencyGraph` catalog layer. If a requirement has no title in the catalog, fall back to the requirement ID as the title.

**Alternatives considered**:
- Summary in `--format pretty` only: Hides actionable data behind a flag. Admins should always see what passed/failed.
- Plain text (no charmbracelet): Inconsistent with `list`, `plugins`, execution plan tables. Charmbracelet provides terminal width handling and consistent borders.
- Color-coded text instead of emoji: Less accessible in colorblind scenarios. Emoji is universally recognizable and renders in non-ANSI terminals as text fallback.

## R44: `complyctl doctor` — Pre-flight Diagnostics Command (Session 2026-02-24)

**Decision**: Add `complyctl doctor` as a comprehensive pre-flight diagnostics command. Checks: (1) workspace config file exists and passes syntax/field validation, (2) scanning provider directory exists and contains discoverable providers with passing HealthCheck, (3) OCI registry reachability (non-blocking — warning if unreachable, not error), (4) `evaluator_config` entries in workspace config align with discovered provider evaluator IDs. Output uses emoji + message per check (same pattern as scan summary). Exit 0 only if all blocking checks pass. Registry unreachability is a warning, not a failure.

**Rationale**: Before `doctor`, config validation was implicit — errors surfaced only during `init`, `get`, or `scan` execution. Admins had no single command to verify environment readiness. `doctor` follows an established CLI pattern (`brew doctor`, `flutter doctor`, `gcal-organizer doctor`) where a single diagnostic command surfaces all issues before the user attempts real work. The emoji + message output format (e.g., `✅ Workspace config valid`, `❌ No providers found`, `⚠️ Registry unreachable`) matches the scan summary UX pattern, keeping the CLI experience consistent.

**Implementation**:

| Component | Change |
| :--- | :--- |
| `cmd/complyctl/cli/doctor.go` | New file: `doctorCmd` registered in `root.go`. Calls `doctor.Run()` |
| `internal/doctor/doctor.go` | New package: `Run(configPath, providerDir, registryURL string) []CheckResult`. Iterates checks, returns pass/fail/warn per check. Each check is an independent function |
| `internal/doctor/checks.go` | Individual check functions: `CheckConfig()`, `CheckProviders()`, `CheckRegistry()`, `CheckEvaluatorConfig()` |
| `cmd/complyctl/cli/root.go` | Register `doctorCmd` alongside other top-level commands |

**Check sequence**:

| Check | Blocking | Pass | Fail | Warn |
| :--- | :--- | :--- | :--- | :--- |
| Config exists and valid | Yes | `✅ Workspace config valid` | `❌ Config not found at ./complytime.yaml` | — |
| Providers discovered | Yes | `✅ 2 scanning providers found (openscap, kube-evaluator)` | `❌ No providers found in ~/.complytime/providers/` | — |
| Provider HealthCheck | Yes | `✅ openscap: healthy (v1.2.0)` | `❌ openscap: unhealthy` | — |
| Registry reachable | No | `✅ Registry reachable: registry.example.com` | — | `⚠️ Registry unreachable (cached policies available)` |
| Required variables (R51) | Yes | `✅ All required variables present` | `❌ Provider "openscap": missing target variable "profile" for target "local"` | — |

**Exit code**: 0 if all blocking checks pass (registry warning allowed). Non-zero if any blocking check fails.

**Alternatives considered**:
- Standalone `complyctl validate` command: Too narrow — validates config only, misses provider and registry issues.
- `complyctl init --validate-only`: Overloads `init` with a diagnostic concern. `doctor` is a separate, composable command.
- No diagnostics (rely on error messages during scan): Poor UX — admin discovers issues one at a time during execution rather than all at once up front.

## R45: Scan Summary Redesign — ActionError-Style Output (Session 2026-02-24)

**Decision**: Replace the multi-row charmbracelet table (R43) with an ActionError-style output inspired by gcal-organizer's UX pattern. Only non-passing results are listed individually, each as a standalone line: emoji + log message (no requirement ID). Passed results appear only in the totals row. Below the individual lines, a single-row charmbracelet totals table summarizes counts for all statuses including passed.

**Rationale**: R43 defined a full charmbracelet table with Requirement ID, Title/Description, and Status columns. Feedback indicated that admins care about *why* something failed, not *what ID* failed. The gcal-organizer `ActionError` pattern (`❌ Error: <message>`) provides immediately actionable output — the failure message tells the admin what went wrong and (implicitly) what to fix. A wall of passed rows buries failures. Showing only non-passing results with actionable messages, followed by a compact totals row, maximizes signal-to-noise.

**Design**:

```text
❌ SSH service must be configured with approved ciphers only
❌ Password hashing algorithm must use SHA512
⚠️ Failed to evaluate: openscap plugin returned timeout for xccdf_rule_9999
⏭️ Skipped: Rule not applicable to target configuration

┌──────────┬──────────┬──────────┬──────────┐
│ 44 ✅    │ 2 ❌     │ 1 ⏭️     │ 1 ⚠️     │
└──────────┴──────────┴──────────┴──────────┘
```

**Message source**: `AssessmentLog.Steps[].Message` field directly. Scanning provider authors control the text of failure/skip/error messages. No secondary mapping layer or requirement title substitution.

**Result aggregation**: Delegates to go-gemara's built-in results aggregation function for determining the overall result per `AssessmentLog`. Display the message from the step whose result matches the aggregated outcome (first match). No custom aggregation logic in complyctl.

**Implementation**:

| Component | Before (R43) | After (R45) |
| :--- | :--- | :--- |
| `internal/output/scan_summary.go` | `FormatScanSummary()` builds 3-column charmbracelet table (Req ID, Title, Status) | `FormatScanSummary()` builds emoji + message lines for non-passing results, then single-row charmbracelet totals table |
| Table structure | Multi-row table with all results | Individual lines per non-passing result + one-row totals table |
| Data source per line | `reqTitles[requirementID]` (catalog title) | `AssessmentLog.Steps[].Message` (provider-authored) |
| Result aggregation | Custom `aggregateResultFromSteps()` | `go-gemara` aggregation function |
| Passed results | Listed as table rows | Totals only (not listed individually) |
| Sort order | ❌ → ⚠️ → ⏭️ → ✅ (all statuses) | ❌ → ⚠️ → ⏭️ (non-passing only) |

**Code impact**: Rewrite `FormatScanSummary()` in `scan_summary.go`. Remove `ScanSummaryRow.RequirementID` and `ScanSummaryRow.Title` fields (no longer needed). Replace `aggregateResultFromSteps()` with call to go-gemara's aggregation. Update `scan.go` to pass full `[]plugin.AssessmentLog` (message extraction happens inside `FormatScanSummary`). `extractReqTitles()` helper no longer called from scan path (may still be useful for other output formats).

**Alternatives considered**:
- Keep R43 full table + add message column: Table becomes too wide for terminal; message text varies in length.
- Show only failures (not errors/skipped): Errors and skips are also actionable — admin needs to see them.
- Include requirement ID in the line: Adds noise. The message should be self-explanatory. Requirement ID is in the EvaluationLog file for detailed investigation.

## R46: Scanning Provider Terminology (Session 2026-02-24)

**Decision**: User-facing documentation adopts "scanning providers" for individual evaluator executables and "scanning interface" for the gRPC contract they implement. Code-level packages (`pkg/plugin`, `internal/plugin`) retain `plugin` naming for compatibility with `hashicorp/go-plugin`. CLI command: `complyctl providers`.

**Rationale**: "Plugin" is overloaded — it means different things in different ecosystems (IDE plugins, browser plugins, build plugins). "Scanning provider" is domain-specific and self-documenting: it tells the admin *what the thing does* (provides scanning capability) rather than *what it is architecturally* (a plugin binary). "Scanning interface" follows the same pattern — it describes the contract that providers implement, not the wire protocol (gRPC). The code-level `plugin` package name stays because `hashicorp/go-plugin` uses that terminology natively, and renaming would create a confusing mismatch between the library's API and the wrapper's package name.

**Mapping**:

| Before | After (user-facing) | Code-level |
| :--- | :--- | :--- |
| plugin (the executable) | scanning provider / provider | `plugin.Plugin` interface |
| plugin system | scanning interface | `pkg/plugin/`, `internal/plugin/` packages |
| `complyctl plugins` | `complyctl providers` | `cmd/complyctl/cli/providers.go` |
| plugin directory | provider directory | `~/.complytime/providers/` (path unchanged) |
| plugin discovery | provider discovery | `plugin.Manager.LoadPlugins()` (unchanged) |
| plugin protocol | scanning interface protocol | gRPC `Plugin` service (proto name unchanged) |

**Code impact**: Rename `cmd/complyctl/cli/plugins.go` → `providers.go`. Update `cobra.Command.Use` from `plugins` to `providers`. Update `Short` description to reference "scanning providers." All internal Go code keeps `plugin` naming — no package renames.

**Alternatives considered**:
- Full rename (code + user-facing): Breaks `hashicorp/go-plugin` convention. Creates confusing mismatch between library docs and wrapper.
- "Evaluator" as user-facing term: Already used for `evaluator_id` (a property of a provider). Using it for the executable itself would overload the term.
- Keep "plugin" everywhere: Misses opportunity to make the domain language self-documenting for compliance admins who may not be developers.

## R47: Targets-Only Scan RPC (Session 2026-02-24)

**Decision**: Remove `requirement_ids` from `ScanRequest`. Scan RPC receives only targets (each with target ID and target variables). The scanning provider evaluates all requirements it was configured with during Generate. No selective scanning by requirement ID.

**Rationale**: The plugin was configured during Generate with its full requirement set (assessment configurations, parameters, evaluator config). Passing requirement_ids again during Scan is redundant and introduces a drift vector — Generate might configure N requirements while Scan requests M. The plugin already knows what to scan from its Generate-time state. Targets-only Scan is the simplest correct contract: "you already know what to check, here's where to check it."

**Proto impact**:

| Before                                                                                        | After                                                  |
|-----------------------------------------------------------------------------------------------|--------------------------------------------------------|
| `message ScanRequest { repeated Target targets = 1; repeated string requirement_ids = 2; }`   | `message ScanRequest { repeated Target targets = 1; }` |

**Code impact**:

| Component                                   | Change                                                                                          |
|---------------------------------------------|-------------------------------------------------------------------------------------------------|
| `contracts/plugin.proto`                    | Remove `repeated string requirement_ids` from `ScanRequest`                                     |
| `pkg/plugin/client.go`                      | Remove `RequirementIDs` from `ScanRequest` domain type                                          |
| `pkg/plugin/server.go`                      | Update adapter — no requirement_ids mapping                                                     |
| `internal/plugin/manager.go` `RouteScan()`  | Remove `reqIDs []string` parameter. Provider receives targets only                              |
| `cmd/complyctl/cli/scan.go`                 | Remove requirement ID collection and passing to `RouteScan()`                                   |
| `cmd/openscap-plugin/server/server.go`      | Remove requirement ID filtering in Scan handler — evaluate all requirements from Generate state |
| `cmd/test-plugin/main.go`                   | Update test plugin Scan handler                                                                 |

**Alternatives considered**:
- Keep `requirement_ids` for selective re-scanning: The spec does not support selective scanning. If needed later, add an optional filter field — simpler to add than to remove.
- Keep `requirement_ids` for validation (assert provider received same set): Adds complexity for a check that should never fail if Generate and Scan use the same policy version.

## R48: Three-Tier Variable Model (Session 2026-02-24)

**Decision**: Replace `evaluator_config` (a nested map keyed by evaluator ID under PolicyConfig) with a three-tier variable model:

| Tier | Name | Scope | Source | RPC | Owner |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 1 | Global variables | Workspace | Top-level `variables` in `complytime.yaml` | Generate RPC | System admin |
| 2 | Target variables | Per-target | `targets[].variables` in `complytime.yaml` | Scan RPC | System admin |
| 3 | Test variables | Per-requirement | Decomposed Gemara policy assessment plan | Generate RPC | Policy author |

**Rationale**: `evaluator_config` mixed two concerns: workspace-level settings (like scan output directory) and evaluator-specific configuration (like profile selection). It was keyed by evaluator ID, which forced the admin to understand the evaluator routing to configure the workspace. The three-tier model separates by domain role: admins configure what they own (global workspace settings + per-target runtime config), policy authors define test parameters in the policy itself. The evaluator doesn't need its own config section — it receives global variables (workspace-wide), test variables (from policy), and target variables (from target runtime) through the natural flow.

**Config structure**:

```yaml
# complytime.yaml
registry:
  url: https://registry.example.com
variables:                     # Tier 1: Global variables (workspace-scoped)
  workspace: ./.complytime/scan
policies:
  - id: cis-fedora-l1-workstation
targets:
  - id: local
    policy_ids:
      - cis-fedora-l1-workstation
    variables:                 # Tier 2: Target variables (per-target)
      profile: cis_workstation_l1
      kubeconfig: /path/to/kubeconfig
# Tier 3: Test variables come from the policy itself (not configured here)
```

**Code impact**:

| Component | Before | After |
| :--- | :--- | :--- |
| `internal/config/config.go` `PolicyConfig` | `EvaluatorConfig map[string]map[string]string` | Remove `EvaluatorConfig` field |
| `internal/config/config.go` `WorkspaceConfig` | No top-level variables | Add `Variables map[string]string` |
| `internal/config/config.go` `TargetConfig` | `Variables map[string]string` (unchanged) | Unchanged — already correct |
| `internal/policy/assessment.go` | `ValidateEvaluatorConfig()` | `ValidateGlobalVars()` — checks required global variables |
| `cmd/complyctl/cli/generate.go` | Passes `group.EvaluatorConfig` to `RouteGenerate()` | Passes `ws.Config().Variables` (global) to `RouteGenerate()` |
| `cmd/complyctl/cli/scan.go` | Passes `group.EvaluatorConfig` to `RouteGenerate()` | Same change as generate.go |
| `internal/plugin/manager.go` `RouteGenerate()` | `evaluatorConfig map[string]string` param | `globalVars map[string]string` param |
| `contracts/plugin.proto` `GenerateRequest` | `map<string, string> evaluator_config` | `map<string, string> global_variables` |
| `cmd/openscap-plugin/server/server.go` | `req.EvaluatorConfig["workspace"]` | `req.GlobalVariables["workspace"]` |

**Alternatives considered**:
- Keep `evaluator_config` and rename: Still keyed by evaluator ID — admin must understand evaluator routing to configure workspace. Wrong abstraction level.
- Merge everything into `Target.variables`: Global settings (workspace dir) are not target-specific. Would force repetition across targets or require a "default" target concept.
- Two-tier only (global + target): Loses the distinction that test parameters come from the policy, not admin config. Three tiers makes ownership explicit.

## R49: Global Variables Config Location (Session 2026-02-24)

**Decision**: Global variables live in a top-level `variables` section in `complytime.yaml`. Not under `PolicyConfig`, not under `TargetConfig`.

**Rationale**: Global variables are workspace-scoped — they apply to all policies and all targets. Placing them under PolicyConfig (as `evaluator_config` was) forces per-policy duplication or a confusing "which policy's config wins?" precedence rule. Top-level placement follows Constitution VII (Convention Over Configuration): one section, one purpose, no ambiguity.

**Validation**: FR-036 updated to validate that required global variables are present and non-empty before Generate RPC dispatch. Missing required variables produce a validation error naming the variable and pointing to the workspace config `variables` section.

**Alternatives considered**:
- Under `PolicyConfig`: Wrong scope — global variables are not policy-specific.
- Environment variables: Would bypass the config file as single source of truth. Admin must manage config in two places.
- CLI flags: Adds per-command flag overhead. Workspace config is the natural location for persistent settings.

## R50: Init as Composite Orchestrator + Static Config Convention (Session 2026-02-23e)

**Decision**: `complyctl init` is a composite orchestrator with three phases: ~~(1) create `complytime.yaml` via interactive prompts, (2) run `complyctl doctor` to validate the environment, (3) run `complyctl get` to fetch policies — only if doctor's blocking checks pass.~~ (Superseded R52: (1) create `complytime.yaml` via interactive prompts, (2) run `complyctl get` to fetch policies, (3) run `complyctl doctor` to validate the environment with full policy context.) `init` errors if `complytime.yaml` already exists (like `go mod init` errors when `go.mod` exists). `complytime.yaml` is a static convention (like `go.mod`) — no `--config` flag on any command. All validation logic lives in `doctor`; `init` delegates entirely.

**Rationale**: Before R50, `init` duplicated validation logic (`ValidateTargetPolicyVersions`, explicit "Validating workspace configuration..." messaging) that `doctor` already owns. This created two places where validation could diverge. Moving all validation to `doctor` and having `init` delegate achieves single responsibility: `init` = setup, `doctor` = validate, `get` = fetch. The static config convention (like `go.mod`) eliminates the `--config` flag — one fewer decision for the user. `go mod init` refuses to overwrite an existing `go.mod` — same principle: `init` is strictly for bootstrapping, not re-initialization. CI/CD path: commit `complytime.yaml`, run `doctor` + `get`.

**Command separation**:

| Command | Analogy | Responsibility |
| :--- | :--- | :--- |
| `init` | `go mod init` | Create config → get → doctor (errors if config exists, R52) |
| `get` | `go get -u` | Incremental policy sync from registry |
| `doctor` | `flutter doctor` | Diagnose environment health (requires policy cache, R52) |

**Code impact**:

| Component | Before | After |
| :--- | :--- | :--- |
| `cmd/complyctl/cli/init.go` | `--config` flag, `ValidateTargetPolicyVersions`, "Validating..." messages | No flags; errors if config exists; calls `getOpts.run()` then `runDoctor()` (R52) |
| `internal/complytime/config.go` | `Load()` calls `Validate()` internally | `LoadFrom(path)` returns parsed config; callers validate explicitly |
| `internal/doctor/doctor.go` `CheckConfig` | Calls `complytime.Load()` (basic parse) | Calls `LoadFrom()` + `Validate()` + `ValidateTargetPolicyVersions()` (full validation) |
| `cmd/complyctl/cli/get.go` | No explicit `Validate()` | Explicit `Validate(cfg)` after `workspace.Load()` |
| `cmd/complyctl/cli/generate.go` | `ValidateTargetPolicyVersions` (deep validation in wrong place) | `Validate(cfg)` only (structural checks) |

**Alternatives considered**:
- Keep `--config` flag: Adds complexity for a problem already solved by convention. `go.mod` doesn't need `--config`.
- Init with no doctor: Misses the opportunity to catch issues early. Admin runs `init`, `get` fails, they must run `doctor` manually.
- Doctor inside `get` instead of `init`: `get` is a focused sync command. Embedding diagnostics would slow every sync and conflate concerns.

## R51: HealthCheck Required Variables Extension (Session 2026-02-23g)

**Decision**: Extend `HealthCheckResponse` with two new fields: `repeated string required_global_variables` and `repeated string required_target_variables`. Providers declare the variable *names* they need; `doctor` validates those keys exist in the workspace config. Global variables are checked against `config.variables`; target variables are checked against each relevant `config.targets[].variables` section (using policy → evaluator → target mapping resolved from cached policies).

**Rationale**: FR-039 check (5) previously said "global variables present if configured" — a vague existence check. Providers documented expected variables out-of-band only (R31), so doctor had no way to know *which* variables were required. This created a gap: misconfigured variables were only caught at generate/scan time (provider returns error), defeating doctor's purpose as a pre-flight diagnostic. Extending HealthCheck is the lowest-friction approach — providers already implement it, it runs during doctor, and adding two repeated string fields is a single proto change. Convention-based (documentation-only) approaches can't catch mismatches programmatically.

**Proto impact**:

| Before                                                                                             | After                                                                                                                                                                                           |
|----------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `message HealthCheckResponse { bool healthy = 1; string version = 2; string error_message = 3; }`  | `message HealthCheckResponse { bool healthy = 1; string version = 2; string error_message = 3; repeated string required_global_variables = 4; repeated string required_target_variables = 5; }` |

**Backward compatibility**: Proto3 defaults empty repeated fields to `[]`. Existing providers that don't populate these fields return empty lists — doctor treats them as "no variables required" and passes variable validation. Providers are updated incrementally.

**Doctor validation flow**:
1. Discover providers in `~/.complytime/providers/`
2. Call HealthCheck on each — collect `required_global_variables` and `required_target_variables`
3. For global: verify each required key exists in `config.variables`
4. For target: resolve policy → evaluator mapping from cached policies, then for each target whose policies route to this provider, verify required target variable keys exist in `target.variables`

**Code impact**:

| Component | Change |
| :--- | :--- |
| `contracts/plugin.proto` | Add two fields to `HealthCheckResponse` |
| `pkg/plugin/client.go` | Add `RequiredGlobalVariables []string` and `RequiredTargetVariables []string` to domain `HealthCheckResponse` type |
| `pkg/plugin/server.go` | Map new proto fields in adapter |
| `internal/doctor/doctor.go` | New check: `CheckVariables()` — after HealthCheck, validate required keys against config sections |
| `cmd/openscap-plugin/server/server.go` | Populate `required_global_variables: ["workspace"]` and `required_target_variables: ["profile"]` in HealthCheck response |

**Supersedes**: R31 statement "Vars are not advertised by plugins through any protocol mechanism" and R41 "Plugin advertises required config keys via HealthCheck: Adds protocol complexity." R51 accepts the protocol extension because doctor's value depends on concrete variable validation — existence checks alone are insufficient.

**Alternatives considered**:
- Existence check only (non-empty sections): Doctor can't distinguish "admin intentionally left variables empty" from "admin forgot a required variable." No signal without provider declaration.
- New `GetRequirements` RPC: Adds a separate method when HealthCheck already runs during doctor. Extra protocol surface for no additional benefit.
- Structured `VariableRequirement` message with scope enum: Heavier than two flat lists. The global/target distinction is already encoded by field name — no need for a scope enum.

## R52: Init Flow Reordering — Get Before Doctor (Session 2026-02-23g)

**Decision**: Reorder the `init` orchestration phases from create config → doctor → get to create config → **get** → **doctor**. Doctor requires the policy cache to resolve the provider-to-target mapping for target variable validation (R51). `get` must run first to populate the cache.

**Rationale**: R51 introduces target variable validation in doctor. To validate that the correct targets have the required target variables, doctor must know which provider serves which target. This mapping comes from the policy content (Layer 3: assessment plan → evaluator ID), which is only available after `get` fetches policies to cache. Running doctor before `get` would force either: (a) skipping target variable validation (partial doctor), or (b) requiring `get` to have been run separately before doctor (poor UX for `init`). Reordering eliminates the need for progressive validation modes — doctor always has full context.

**Tradeoffs**:
- `get` fails fast on invalid config or unreachable registry — these errors are still surfaced, just not in doctor's emoji diagnostic format
- Doctor becomes a comprehensive *post-setup* diagnostic rather than a *pre-fetch* gate
- Standalone `doctor` requires the policy cache to exist — errors with guidance to run `get` first if missing

**Impact on R50**: Updates the R50 init flow. R50 said "create config → doctor → get." R52 changes this to "create config → get → doctor." All other R50 decisions (static convention, no `--config` flag, errors if config exists) remain unchanged.

**Command separation (updated)**:

| Command | Analogy | Responsibility |
| :--- | :--- | :--- |
| `init` | `go mod init` | Create config → get → doctor (errors if config exists) |
| `get` | `go get -u` | Incremental policy sync from registry |
| `doctor` | `flutter doctor` | Diagnose environment health (requires policy cache) |

**Code impact**:

| Component | Before (R50) | After (R52) |
| :--- | :--- | :--- |
| `cmd/complyctl/cli/init.go` | Calls `runDoctor()` then `getOpts.run()` | Calls `getOpts.run()` then `runDoctor()` |
| `cmd/complyctl/cli/doctor.go` | No cache dependency | Check policy cache exists; error if missing with "run `complyctl get` first" guidance |
| `internal/doctor/doctor.go` | Variable check is existence-only | Variable check resolves policy → evaluator → target mapping from cache |

**Alternatives considered**:
- Keep doctor → get order with partial validation: Introduces two validation modes (pre-cache and post-cache). Added complexity for doctor's implementation and testing.
- Doctor calls `get` internally if cache is missing: Conflates concerns. Doctor is diagnostic, not a sync command.
- Skip target variable validation entirely: Undermines the value of R51 — the whole point is catching misconfigured target variables before scan time.

## R53: Comply-Pack Manifest / Runtime Config Separation (Session 2026-02-25b)

**Decision**: Comply-packs use two separate configuration files with distinct ownership. `complypack.yaml` (pack manifest) declares what the pack contains — developer-owned, immutable after build, ships in the pack. `complytime.yaml` (runtime config) declares how to run — consumer-owned, mutable per-environment, does NOT ship in the pack. The pack ships a `complytime.yaml.example` as a starter template.

**Rationale**: An earlier design (Session 2026-02-24b) combined both concerns into a single file. This created three problems: (1) adding a target or rotating a credential required a pack rebuild, (2) target variables containing `${CREDENTIALS}` are environment-specific and don't belong in a distributable artifact, (3) the pack developer was dictating runtime config that only the consumer should control. Separating the files aligns with Constitution I (Single Source of Truth — each file has one authoritative purpose), VII (Convention Over Configuration — `complytime.yaml` already exists as the runtime convention), and II (Simplicity — clear ownership boundary).

**`complypack.yaml` schema**:

| Field | Type | Required | Owner | Description |
| :--- | :--- | :--- | :--- | :--- |
| `id` | `string` | Yes | Developer | Pack identifier (e.g., `fedora-compliance`) |
| `version` | `string` | Yes | Developer | Semantic version |
| `description` | `string` | No | Developer | Human-readable description |
| `platform` | `PlatformConfig` | No | Developer | Target OS and datastream path |
| `registry` | `RegistryConfig` | Yes | Developer | Source registry for policy fetch during build |
| `policies` | `[]PackPolicyEntry` | Yes (>=1) | Developer | Policies to bundle |
| `providers` | `[]PackProviderEntry` | Yes (>=1) | Developer | Provider binaries to bundle |
| `system-dependencies` | `[]SystemDependency` | No | Developer | OS packages required at runtime |

**`complytime.yaml` in pack context** — identical to the existing `WorkspaceConfig` schema (R8, R48, R49). Key differences from standalone usage:
- `registry.url` is optional (policies pre-cached in pack)
- `policies[]` may be a subset of what the pack bundles
- `targets[]` and `variables{}` are fully consumer-controlled

**`doctor` reads both files** when both exist:
- Pack-layer checks from `complypack.yaml`: manifest schema, provider binaries exist, cache digest integrity, system dependency `check` commands
- Config-layer checks from `complytime.yaml`: target-policy bindings, variable resolution, global variable completeness
- If `complypack.yaml` absent: standalone mode (existing behavior)
- If `complytime.yaml` absent but `complypack.yaml` present: pack OK + config missing with remediation guidance

**Fedora comply-pack inventory** (all Fedora content from ComplianceAsCode/oscal-content):

| Policy ID | Catalog | SSG Profile | Source |
| :--- | :--- | :--- | :--- |
| `policies/cis-fedora-l1-server` | `cis_fedora` | `cis_server_l1` | `component-definitions/fedora/fedora-cis_fedora-l1_server` |
| `policies/cis-fedora-l1-workstation` | `cis_fedora` | `cis_workstation_l1` | `component-definitions/fedora/fedora-cis_fedora-l1_workstation` |
| `policies/cis-fedora-l2-server` | `cis_fedora` | `cis_server_l2` | `component-definitions/fedora/fedora-cis_fedora-l2_server` |
| `policies/cis-fedora-l2-workstation` | `cis_fedora` | `cis_workstation_l2` | `component-definitions/fedora/fedora-cis_fedora-l2_workstation` |
| `policies/cusp-fedora-default` | `cusp_fedora` | `cusp_fedora` | `component-definitions/fedora/fedora-cusp_fedora-default` |

**Pack build output**: Tarball containing `complypack.yaml`, `bin/complyctl`, `bin/complyctl-provider-*`, `policies/*/` (OCI layouts), `complytime.yaml.example`. NO `complytime.yaml`.

**Pack distribution**: `complyctl pack push --tag <version>` publishes as OCI artifact. `complyctl pack pull <id> --tag <version>` retrieves.

**Code impact** (deferred to 002-comply-packs):

| Component | Description |
| :--- | :--- |
| `internal/complytime/pack.go` | `PackManifest` struct, `LoadPackManifest()`, `ValidatePackManifest()` |
| `internal/doctor/doctor.go` | Extend `Run()` to detect and validate `complypack.yaml` alongside `complytime.yaml` |
| `cmd/complyctl/cli/pack.go` | `pack doctor`, `pack build`, `pack push`, `pack pull` subcommands |
| `internal/pack/build.go` | Pack assembly: fetch policies, copy binaries, generate example config, create tarball |

**Alternatives considered**:
- Single combined file: Mixes developer and consumer concerns. Pack rebuild required for credential rotation. Environment-specific config in a distributable artifact.
- Three files (manifest + defaults + overrides): Over-engineering. Two files with clear ownership is sufficient.
- Pack ships a real `complytime.yaml`: Consumer edits a file that came from the pack. Ambiguous ownership — whose file is it? The example pattern makes ownership explicit.

## R54: Init Redesign — Config-Only + Pack Init + Dual-Mode Config (Session 2026-02-25c)

**Decision**: Three interconnected changes to the init workflow and config model:

1. **`complyctl init` is config-creation-only**: Creates `complytime.yaml` via interactive prompts and exits. No `get`, no `doctor`. User runs those separately. Supersedes R50/R52 composite orchestrator.
2. **`complyctl pack init` in 001 scope**: Creates `complypack.yaml` for pack developers. Prompts for pack id, version, registry URL, policy IDs, provider IDs. Platform/system-deps default empty. `pack` is a top-level command group (noun-first, like `providers`). Build/push/pull remain in 002.
3. **Dual-mode `complytime.yaml`**: Two mutually exclusive modes — (a) `pack` field: OCI reference to a comply-pack (e.g., `registry.com/fedora-pack@v1.0`); (b) `registry` + `policies` fields: standalone mode. Error if both present. `complyctl init` prompts for pack reference by default; standalone mode requires manual YAML editing.

**Rationale**:

| Change | Rationale | Constitution |
| :--- | :--- | :--- |
| Init config-only | Decouples config creation from network (get) and provider ecosystem (doctor). Matches `go mod init` which only creates `go.mod` without fetching dependencies. Each command does one thing. | II (Simplicity), VII (Convention) |
| Pack init in 001 | Pack developers need to author manifests now. Only the creation command is needed — build/push/pull have their own dependency chain (tarball assembly, OCI push). Minimal scope increase. | III (Incremental) |
| Dual-mode config | Pack consumers shouldn't duplicate registry URL and policy IDs already in the pack. Pack reference is a single OCI ref. Standalone mode remains for development, testing, and no-pack environments. Mutual exclusion prevents ambiguous config states. | I (Single Source), VII (Convention) |

**Config structure (pack mode)**:

```yaml
# complytime.yaml — pack consumer
pack: registry.com/fedora-pack@v1.0
variables:
  workspace: ./.complytime/scan
targets:
  - id: local
    policy_ids:
      - cis-fedora-l1-workstation
    variables:
      profile: cis_workstation_l1
```

**Config structure (standalone mode)**:

```yaml
# complytime.yaml — standalone (development/testing)
registry:
  url: https://registry.example.com
policies:
  - id: nist-800-53-r5
  - id: cis-benchmark
    version: v1.0.0
variables:
  workspace: ./.complytime/scan
targets:
  - id: local
    policy_ids:
      - cis-benchmark
    variables:
      profile: cis_workstation_l1
```

**Validation**: `Validate()` checks mutual exclusion: if `pack` is non-empty AND (`registry.url` is non-empty OR `policies` is non-empty), return error. In pack mode, `policies` list is not required in `complytime.yaml` — the pack provides them. In standalone mode, `registry.url` and `policies` are required (existing behavior).

**Code impact**:

| Component | Before | After |
| :--- | :--- | :--- |
| `internal/complytime/config.go` `WorkspaceConfig` | `Registry`, `Policies`, `Targets`, `Variables` | Add `Pack string` field. `Registry`/`Policies` optional in pack mode |
| `internal/complytime/config.go` `Validate()` | Requires `registry.url` and `policies` | Mutual exclusion check: `pack` XOR (`registry` + `policies`). In pack mode, `registry`/`policies` not required |
| `cmd/complyctl/cli/init.go` | Composite orchestrator: config → get → doctor | Config-only: prompt for pack ref + targets → save → exit |
| `cmd/complyctl/cli/pack.go` | (deferred to 002) | New: `pack init` subcommand with rich prompts (id, version, registry, policies, providers) |
| `cmd/complyctl/cli/root.go` | No `pack` command group | Register `pack` command group with `init` subcommand |

**Impact on R50/R52**: R54 supersedes the init flow from R50/R52. The composite orchestrator design (create → get → doctor) is replaced by config-creation-only. `get` and `doctor` remain standalone commands. R50's other decisions (static convention, no `--config` flag, errors if exists) are unchanged. R52's cache-before-doctor dependency is unchanged for standalone `doctor` usage.

**Impact on R53**: Pack manifest types and validation (already in 001 per R53) gain a CLI command (`pack init`). Pack manifest schema is unchanged. Build/push/pull remain in 002.

**Alternatives considered**:
- Keep composite orchestrator + add pack flag: `init` does too many things. Coupling config creation to network operations means `init` fails on air-gapped environments before creating the config.
- Separate `init` and `init pack` (subcommand on init): `init pack` implies pack is a type of init. `pack init` (noun-first) is clearer — pack is a domain, init is an action within it.
- Pack reference in `registry.url` field: Conflates two concepts — a pack reference is not a registry URL. A pack contains policies from potentially multiple registries. Dedicated `pack` field is clearer.
- Allow both pack and standalone fields (merge semantics): Creates precedence ambiguity — which policies win? Mutual exclusion is simpler and eliminates an entire class of config errors.

## R55: Doctor Redesign — Version Comparison + Per-Provider Config Summary (Session 2026-02-25e)

**Decision**: Three interconnected changes to `complyctl doctor`:

1. **Replace registry reachability probe with per-policy version comparison**: Doctor compares cached policy versions against latest available remotely. Per-policy output: pass if at latest, warning if stale (showing cached vs. available version). Non-blocking warning per unreachable registry — policies from that registry get no staleness line.
2. **Per-provider configuration summary**: Default output shows resolved variable count + missing count per provider. `--verbose` flag expands to full list of expected keys and resolved status per provider. Supersedes validation-only behavior (failures-only).
3. **`--verbose` flag scoped to provider config detail**: Version comparison stays per-policy summary always. Policy evaluation periods (active start/end dates from Gemara policy metadata) are a future `--verbose` candidate when go-gemara exposes validity period fields.

**Rationale**:

| Change | Rationale | Constitution |
| :--- | :--- | :--- |
| Version comparison | Reachability probe provides near-zero actionable value — if `get` succeeded, the registry was reachable. Admins need to know *whether their cached policies are current*, not whether the registry responds to HTTP GET. Per-policy staleness is actionable: run `complyctl get` to update. | IV (Code for Humans — actionable output), VII (Convention — staleness check follows `get`-before-`doctor` ordering from R52) |
| Per-provider config summary | Failures-only output tells admins what's broken but not what's working. A count summary (e.g., `3/3 global vars`) gives confidence that the provider is fully configured without verbose key dumps. `--verbose` provides drill-down for debugging without cluttering default output. | II (Simplicity — default output stays concise), IV (Humans First — admins see completeness at a glance) |
| `--verbose` scope | Mixing version detail (digests, timestamps) with provider config detail into one flag makes output hard to reason about. Version comparison output is already clear per-policy. `--verbose` focuses on the one area where detail expansion matters most: provider config keys. | II (Simplicity — single-concern flag), III (Incremental — additional `--verbose` expansions can be added later) |

**Implementation**:

| Component | Before | After |
| :--- | :--- | :--- |
| `internal/doctor/doctor.go` `CheckRegistries()` | HTTP GET to `/v2/` per unique registry — reachability probe | `CheckPolicyVersions()` — query latest version per policy from registry; compare against `PolicyState.digest`/`version` in `state.json` |
| `internal/doctor/doctor.go` `CheckVariables()` | Reports only missing variables (failures) | Reports per-provider summary: `N/M global vars, N/M target vars`. `--verbose` expands to key-level detail |
| `cmd/complyctl/cli/doctor.go` | No `--verbose` flag | Add `--verbose` bool flag to cobra command. Pass to `doctor.Run()` |
| `internal/doctor/doctor.go` `Run()` | `func Run(cfg, configPath, providerDir, registries, cacheDir, resolver, logOutput)` | Add `verbose bool` parameter. `CheckVariables` receives verbose to control output granularity |
| `internal/cache/state.go` | Provides `PolicyState` (version, digest, synced_at) | No change — doctor reads existing state for version comparison |
| `internal/registry/resolver.go` | `DefinitionVersion()` resolves version from registry | Reused by doctor to query latest remote version per policy |

**Version comparison flow**:
1. Load `state.json` from cache — get per-policy `PolicyState` (cached version + digest)
2. For each policy in config, group by registry via `UniqueRegistries()`
3. Per registry: attempt to query latest version via `DefinitionVersion()`. If registry unreachable → emit warning per registry, skip all policies from that registry
4. Per reachable policy: compare cached version against remote latest. Stale → warning with both versions + remediation. Up-to-date → pass

**Per-provider summary format**:

```text
# Default output
✅ provider/openscap: 3/3 global vars, 2/2 target vars
❌ provider/kube-eval: 1/2 global vars — missing workspace

# --verbose output
✅ provider/openscap: 3/3 global vars, 2/2 target vars
   global: workspace ✅, output_dir ✅, log_level ✅
   target[local]: profile ✅, datastream ✅
❌ provider/kube-eval: 1/2 global vars — missing workspace
   global: workspace ❌, namespace ✅
```

**Supersedes**: R44 registry reachability check (replaced by version comparison). R44's other checks (config, providers, variable validation) unchanged.

**Alternatives considered**:
- Keep reachability + add version comparison as separate check: Two checks for registry interaction is redundant. Version comparison implicitly proves reachability.
- `--verbose` expands everything (version detail + provider config): Mixes concerns. Version comparison output is already clear per-policy.
- Show full key list by default (no `--verbose`): Clutters default output for admins with many providers/variables. Count summary is sufficient for the happy path.

## R56: Unit Test Strategy for Critical Untested Packages (Session 2026-02-26)

**Decision**: Five unit test coverage decisions for three critical packages (`internal/policy/`, `pkg/plugin/discovery.go`, `internal/cache/state.go`) that lacked direct unit tests.

| Decision | Choice | Rationale |
| :--- | :--- | :--- |
| `internal/policy/` test scope | All exported functions with positive + negative cases | Dependency resolution core — `ResolvePolicyGraph`, `GroupByEvaluator`, `parsePolicyLayer`, `ExtractAssessmentConfigs`, generation state `Save/Load/IsFresh` all drive scan correctness |
| Policy layer test fixtures | Minimal synthetic YAML stubs | Self-contained, no go-gemara fixture dependency. Stubs unmarshal into `gemara.Policy` — upstream shape changes break tests explicitly |
| Plugin discovery test approach | Real temp directory with mock executables | Tests actual filesystem behavior (permission bits, prefix matching, evaluator ID extraction). Zero production code changes. Constitution II (Simplicity) |
| `internal/cache/state.go` coverage | Indirect via `sync_test.go` (sufficient) | State operations exercised through sync scenarios: success, failure, incremental skip, 100-iteration stress test |
| Resolver mock strategy | Interface-based mock at Loader level (`PolicyLoader` interface) | `ResolvePolicyGraph` calls `Loader.LoadLayerByMediaType`, `PolicyExists`, `ResolveVersion`. Interface keeps resolver tests focused on graph assembly. Loader gets separate tests against real OCI stores if needed |

**New test files**:

| File | Package | Key Functions | Scenarios |
| :--- | :--- | :--- | :--- |
| `internal/policy/resolver_test.go` | `policy_test` | `ResolvePolicyGraph`, `parsePolicyLayer`, `extractFromGemaraPolicy` | Empty/invalid inputs, missing layers, valid synthetic Gemara YAML, multi-evaluator extraction. Uses `PolicyLoader` interface mock |
| `internal/policy/assessment_test.go` | `policy_test` | `ExtractAssessmentConfigs`, `GroupByEvaluator`, `ValidateGlobalVars` | Single-evaluator shortcut, multi-evaluator routing, empty graph, missing global vars |
| `internal/policy/loader_test.go` | `policy_test` | `ResolveVersion`, `LoadLayerByMediaType`, `PolicyExists`, `ListCachedPolicies` | Cache miss, version resolution fallback, media type not found |
| `internal/policy/generation_state_test.go` | `policy_test` | `SaveGenerationState`, `LoadGenerationState`, `IsFresh`, `NewGenerationState` | Save/load round-trip, digest match/mismatch, missing state file, corrupt JSON |
| `pkg/plugin/discovery_test.go` | `plugin_test` | `DiscoverPlugins`, `scanDir`, `expandPath` | Valid executable discovery, non-executable skipped, prefix matching, evaluator ID extraction, empty dir, nonexistent dir, user-dir precedence over system-dir |

**Interface addition**: `PolicyLoader` interface in `internal/policy/resolver.go` (or a `resolver_internal_test.go` mock) with methods: `LoadLayerByMediaType(policyID, version, mediaType string) ([]byte, error)`, `PolicyExists(policyID, version string) bool`, `ResolveVersion(policyID, configVersion string) (string, error)`. Production `Loader` satisfies this interface. Tests inject a mock implementation.

**Constitution alignment**:
- **I (Single Source of Truth)**: Test scenarios derived from the spec's edge case table and functional requirements — not invented independently
- **II (Simplicity & Isolation)**: Each test file tests one source file. No cross-package test dependencies. Mock interfaces are minimal
- **IV (Readability First)**: Synthetic YAML stubs are self-documenting — test reader sees the exact input structure
- **V (Do Not Reinvent the Wheel)**: Uses `testify/assert` + `testify/require` (already vendored). `t.TempDir()` for filesystem tests. No custom test framework

**Alternatives considered**:
- Full OCI store integration tests for resolver: Couples resolver tests to OCI internals. Slower, harder to reason about failures
- go-gemara fixture files for parsePolicyLayer: External dependency on upstream test data. Stubs are faster and break on contract changes
- DirScanner interface for discovery: Adds abstraction layer to production code for testability only. Real temp dir tests actual behavior without code changes
- Dedicated state_test.go: sync_test.go already exercises all state operations through 5 test scenarios including the 100-iteration stress test

## R57: Terminal Output Redesign — Plain Default + Log Relocation (Session 2026-02-26b)

**Decision**: Five UX decisions reshaping terminal output and log file handling.

| Decision | Choice | Rationale |
| :--- | :--- | :--- |
| Log file location | `.complytime/complyctl.log` | Logs are diagnostic artifacts — same category as scan output. Keeps workspace root clean. Supersedes `./complyctl.log` (Session 2026-02-23f) |
| Default table rendering | Plain aligned text (podman-style) + emoji | Pipeable, works in all terminals, no ANSI capability assumptions. Supersedes charmbracelet default (R38) |
| Scan summary totals | Compact inline: `44 ✅  3 ❌  2 ⏭️  1 ⚠️` | Single line, emoji counts, no labels. Matches podman/docker density |
| `bubbles/table` dependency | Remove entirely; keep `lipgloss/table` for `--pretty` | `bubbles/table` was the heavy TUI widget. `lipgloss/table` sufficient for styled non-interactive output |
| `list` columns | Two columns: `POLICY ID` + `VERSION` | Minimal like `podman images`. Registry visible in config if needed |

**Supersedes**: R38 (charmbracelet rendering for all tabular outputs). R38 mandated `charmbracelet/bubbles/table` + `lipgloss` as default for all outputs. R57 inverts: plain is default, `lipgloss/table` is opt-in via `--pretty`.

**Flag changes**:
- `--plain` removed (plain is now the default)
- `--pretty` added (enables `lipgloss/table` styled rendering)

**Code impact**:

| Component | Before | After |
| :--- | :--- | :--- |
| `internal/terminal/table.go` | `RenderTable` (lipgloss default), `ShowPlainTable` (--plain) | `ShowPlainTable` becomes primary. `RenderTable` retained for `--pretty`. Default function is `ShowPlainTable` |
| `cmd/complyctl/cli/providers.go` | `--plain` flag, `RenderTable` default | `--pretty` flag, `ShowPlainTable` default |
| `cmd/complyctl/cli/list.go` | `RenderTable` default | `ShowPlainTable` default, two columns: POLICY ID + VERSION |
| `internal/output/execution_plan.go` | `RenderTable` only | `ShowPlainTable` default, `RenderTable` via `--pretty` |
| `internal/output/scan_summary.go` | charmbracelet totals table | Compact inline: `fmt.Fprintf` with emoji counts |
| `cmd/complyctl/cli/root.go` | Log path: `./complyctl.log` | Log path: `.complytime/complyctl.log` via `WorkspaceDir` + `LogFileName` constants |
| `internal/complytime/consts.go` | No `LogFileName` constant | Add `LogFileName = "complyctl.log"` |
| `go.mod` / `vendor/` | `charmbracelet/bubbles` vendored | Remove `bubbles` dependency. Keep `lipgloss`, `lipgloss/table` |

**Alternatives considered**:
- Keep charmbracelet as default with `--plain` opt-out: Bulky in pipelines, breaks `grep`/`awk` workflows. Admin feedback: "more trouble than they are worth"
- Drop tables entirely (key:value lines like `docker inspect`): Loses columnar alignment for multi-row data. Too sparse for `list` and `providers`
- Remove `lipgloss` entirely (no `--pretty`): Removes all styled output capability. `--pretty` is low-cost to maintain and useful for demos/screenshots
- Log to `~/.complytime/complyctl.log` (user home): Global log loses workspace context. Per-workspace log preserves which workspace generated which diagnostics

## R58: Output Path Split + Discovery Command Simplification (Session 2026-02-26c)

**Decision**: Three UX decisions refining output location and discovery command flags.

| Decision | Choice | Rationale |
| :--- | :--- | :--- |
| `--pretty` on discovery commands | Remove from `list` and `providers` | Discovery commands are informational — plain text is the right default. No persona needs styled output for `list` or `providers`. `--pretty` reserved for reporting/summary commands (`scan`, `generate --dry-run`). Reduces flag surface area |
| `--format` output location | CWD (current working directory) | Formatted reports (OSCAL, SARIF, Markdown) are user-facing deliverables — users expect them in a visible location, not buried in `.complytime/scan/`. EvaluationLog (diagnostic artifact) stays in hidden dir. Terminal summary stays on stdout |
| EvaluationLog path printing | Always print to terminal | EvaluationLog is always produced. Users need to know where it is for debugging. One line of output, low noise. Deterministic path but worth surfacing |

**Output path split**:

| Artifact | Location | Rationale |
| :--- | :--- | :--- |
| EvaluationLog (always) | `{workspace}/.complytime/scan/` | Diagnostic artifact — hidden dir is appropriate |
| Formatted report (`--format`) | CWD | User-facing deliverable — visible location expected |
| Log file | `{workspace}/.complytime/complyctl.log` | Diagnostic — hidden dir (unchanged from R57) |
| Generation state | `{workspace}/.complytime/generation/` | Internal state — hidden dir (unchanged) |

**Supersedes**: R57 partially — R57 applied `--pretty` to all tabular commands including `list` and `providers`. R58 narrows `--pretty` scope to `scan` and `generate --dry-run` only.

**Code impact**:

| Component | Before | After |
| :--- | :--- | :--- |
| `cmd/complyctl/cli/list.go` | `--pretty` flag, `ShowPlainTable` default | `--pretty` flag removed. `ShowPlainTable` only |
| `cmd/complyctl/cli/providers.go` | `--pretty` flag, `ShowPlainTable` default | `--pretty` flag removed. `ShowPlainTable` only |
| `cmd/complyctl/cli/scan.go` | `outDir = ".complytime/scan"` for all output | `outDir` for EvaluationLog stays `.complytime/scan/`. Formatted reports (`--format`) written to `"."` (CWD) |
| `internal/output/evaluator.go` | `Write(outDir)` writes to passed dir | No change — caller controls path |
| `internal/output/oscal.go` | `ToOSCAL(log, outDir)` writes to passed dir | No change — caller passes CWD for `--format` |
| `internal/output/markdown.go` | `Write(outDir)` writes to passed dir | No change — caller passes CWD for `--format` |
| `internal/output/sarif.go` | `ToSARIF(log, uri, outDir)` writes to passed dir | No change — caller passes CWD for `--format` |

**Constitution alignment**:
- **II (Simplicity & Isolation)**: Removing `--pretty` from discovery commands eliminates unused flag paths. Fewer code branches = simpler maintenance
- **VI (Composability)**: Plain-only discovery output is pipeable by default. No styled output to strip
- **VII (Convention Over Configuration)**: Formatted reports in CWD matches standard CLI behavior (output where the user is working). Zero flags needed for the common case

**Alternatives considered**:
- `--output-dir` / `-o` flag on scan for explicit control: Added complexity for rare use case. CWD is the right default. Users who want a specific directory can `cd` there first
- Send formatted output to stdout (kubectl-style): Would require stderr for summary, complicating the output model. File-based is simpler and already established
- Move EvaluationLog to CWD when `--format` is specified: EvaluationLog is a diagnostic artifact — mixing it with user-facing deliverables muddies the intent. Hidden dir is correct for diagnostics

## R59: UX Refresh — Lipgloss Default, Scan Results Table, Execution Plan Collapse (Session 2026-02-26d)

**Decision**: Five interconnected UX decisions replacing the plain-default/`--pretty`-opt-in model with lipgloss-as-universal-default and restructuring scan/generate output.

| Decision | Choice | Rationale |
| :--- | :--- | :--- |
| Scan results table columns | Requirement ID, Control ID, Status (emoji), Message | Admins need enough context to act on failures without cross-referencing the EvaluationLog. Supersedes FR-037 "no table / ActionError-style" |
| Default table rendering | Lipgloss-rendered tables with subtle borders as universal default | Current `--pretty` look becomes the only look. Terminal-width-adaptive, cleaner than raw whitespace padding. `--pretty` flag removed from all commands |
| Report-style layout | Intro text → subtle lipgloss table → conclusion text | Wrapping tables with contextual intro/conclusion provides scannable, professional output. All tabular commands follow this pattern |
| Execution plan structure | Single table: Target, Provider, Requirements, Status | Two stacked lipgloss tables was the bulk problem. One table gives the same info in one view. Supersedes FR-033 two-table design |
| Non-TTY fallback | `ShowPlainTable` retained for piped/redirected output | TTY detection at render time — lipgloss for interactive, plain for pipes. No user-facing flag. Lipgloss gracefully degrades |

**Supersedes**: R57 (plain default + `--pretty` opt-in), R58 partially (discovery commands had no `--pretty` — now all commands use lipgloss with no flag). R38 fully (charmbracelet default → plain default → lipgloss default). Session 2026-02-26c (removed `--pretty` from `list`/`providers` — now `--pretty` removed from everything).

**Flag changes**:
- `--pretty` removed from `scan` and `generate` (was the last two commands that had it)
- `--plain` remains removed (from R57)
- No new flags — lipgloss is unconditional for TTY, plain for non-TTY

**Scan results table design**:

```text
Scan: cis-fedora-l1-workstation | Target: local | 50 requirements

┌─────────────────────┬─────────────────────┬────────┬──────────────────────────────────────┐
│ Requirement ID      │ Control ID          │ Status │ Message                              │
├─────────────────────┼─────────────────────┼────────┼──────────────────────────────────────┤
│ xccdf_req_dconf_db  │ dconf_db_up_to_date │ ❌     │ dconf database is not up to date     │
│ xccdf_req_firewalld │ service_firewalld   │ ❌     │ firewalld service is not running      │
│ xccdf_req_audit_cfg │ auditd_config       │ ⚠️     │ evaluation error: auditd not present  │
│ xccdf_req_usb_guard │ usb_guard_policy    │ ⏭️     │ not applicable: no USB devices        │
└─────────────────────┴─────────────────────┴────────┴──────────────────────────────────────┘

44 ✅  2 ❌  1 ⚠️  1 ⏭️
Evaluation log: .complytime/scan/evaluation-log.yaml
```

**Execution plan (collapsed)**:

```text
Execution Plan: cis-fedora-l1-workstation

┌────────┬──────────┬──────────────┬─────────┐
│ Target │ Provider │ Requirements │ Status  │
├────────┼──────────┼──────────────┼─────────┤
│ local  │ openscap │ 47           │ healthy │
└────────┴──────────┴──────────────┴─────────┘

Generation completed.
```

**Code impact**:

| Component | Before | After |
| :--- | :--- | :--- |
| `internal/terminal/table.go` | `ShowPlainTable` (default), `RenderTable` (--pretty) | `RenderTable` becomes primary (TTY). `ShowPlainTable` fallback (non-TTY). New `RenderReport(intro, headers, rows, conclusion)` function wraps the pattern. TTY detection via `term.IsTerminal(os.Stdout.Fd())` |
| `internal/output/scan_summary.go` | `FormatScanSummary` returns emoji+message lines + inline totals | `FormatScanSummary` returns report-style: intro text, 4-column table rows (reqID, ctrlID, status, message), conclusion (totals + file paths). Accepts `reqToControl` map for control ID lookup |
| `internal/output/execution_plan.go` | `FormatExecutionPlan` with two tables + `pretty bool` param | `FormatExecutionPlan` with single table (Target, Provider, Requirements, Status). No `pretty` param — always lipgloss for TTY. `ProviderRoute` and `TargetScope` structs merged into single `ExecutionPlanRow` |
| `cmd/complyctl/cli/scan.go` | `--pretty` flag, `o.pretty` field | Remove `--pretty` flag and `pretty` field from `scanOptions` |
| `cmd/complyctl/cli/generate.go` | `--pretty` flag, `o.pretty` field | Remove `--pretty` flag and `pretty` field from `generateOptions` |
| `cmd/complyctl/cli/list.go` | `ShowPlainTable` only | `RenderTable` (TTY) / `ShowPlainTable` (non-TTY) via `terminal.RenderReport` |
| `cmd/complyctl/cli/providers.go` | `ShowPlainTable` only | `RenderTable` (TTY) / `ShowPlainTable` (non-TTY) via `terminal.RenderReport` |
| `internal/complytime/consts.go` | `OutputFormatPretty = "pretty"` | Unchanged — `--format pretty` is the Markdown report format, not a rendering style |

**TTY detection strategy**: `internal/terminal/table.go` gains an `IsTTY()` function using `term.IsTerminal(os.Stdout.Fd())`. The `charmbracelet/x/term` package is already vendored (used by `TerminalWidth()`). All render functions check TTY status and dispatch to lipgloss or plain accordingly. No caller changes needed — the terminal package handles it transparently.

**Constitution alignment**:
- **I (Single Source of Truth)**: TTY detection centralized in `internal/terminal`. Scan results table column set defined once in `FormatScanSummary`. `reqToControl` map is the single source for requirement→control mapping.
- **II (Simplicity & Isolation)**: Removing `--pretty` from all commands eliminates branching logic in every CLI command. One rendering path per TTY state, not per flag combination.
- **IV (Readability First)**: Scan results table gives admins requirement ID + control ID + message in one view — no cross-referencing needed. Report-style layout (intro → table → conclusion) provides natural reading flow.
- **V (Do Not Reinvent the Wheel)**: `lipgloss/table` already handles terminal width adaptation, border rendering, and cell padding. `term.IsTerminal` from `charmbracelet/x/term` already vendored.
- **VI (Composability)**: Non-TTY fallback to plain text preserves pipeability. `grep`, `awk`, `jq` work on piped output.
- **VII (Convention Over Configuration)**: No flags needed — lipgloss for interactive, plain for pipes. Zero decisions for the user. Matches how modern CLIs like `gh` and `bat` detect TTY automatically.

**Alternatives considered**:
- Keep `--pretty` as opt-in for lipgloss: Two rendering paths to maintain. Admin feedback: "Remove pretty, make the default stylized." User explicitly wants one good default, not a flag.
- Use `--no-color` instead of TTY detection: Forces the user to opt out. TTY detection is automatic and handles CI/pipe contexts transparently. `NO_COLOR` env var is a future addition (standard convention) but not a flag.
- Show all results (including passed) in scan table: 47 green rows clutters the table. Non-passing only keeps the table actionable. Passed count in totals line confirms nothing was missed.
- Keep two execution plan tables with lipgloss: Still bulky. User explicitly said generate output is "super bulky." Single table collapses Provider Routing + Target Scope into one view.
- Add `--all` flag for full scan table: Added complexity for a niche use case. EvaluationLog already has full details. The table's job is to surface actionable items.
