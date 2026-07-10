## 1. Add complypack dependency

- [x] 1.1 Add `github.com/complytime/complypack` to `go.mod` and run `go mod tidy && go mod vendor`
- [x] 1.2 Verify no dependency conflicts with existing oras-go and go-gemara versions

## 2. Extend workspace config with complypacks section

- [x] 2.1 Add `Complypacks []PolicyEntry` field to `WorkspaceConfig` in `internal/complytime/config.go` with `yaml:"complypacks,omitempty"` tag
- [x] 2.2 Add `ComplypacksSubdir` constant to `internal/complytime/consts.go` (analogous to `PoliciesSubdir = "policies"`). No separate state file constant needed -- complypack state is stored in the existing `state.json`
- [x] 2.3 [P] Add `validateComplypacks(entries []PolicyEntry) error` as a separate function called from `Validate()`. Complypack validation is independent of policy validation -- an empty complypacks list is valid. Same uniqueness checks as policies (duplicate URL, duplicate effective ID) but independent from the policies list
- [x] 2.4 [P] Add unit tests for config:
  - `TestValidate_ComplypackDuplicateURL` -- two complypacks with same URL -> error
  - `TestValidate_ComplypackDuplicateEffectiveID` -- two complypacks with same derived ID -> error
  - `TestValidate_ComplypackAndPolicySameURL_Allowed` -- cross-list independence
  - `TestValidate_ComplypackInvalidOCIRef` -- shell injection, bare word rejection
  - `TestValidate_ComplypackEmptyURL` -- empty URL -> error
  - `TestValidate_ComplypackEmpty_Allowed` -- no complypacks section is valid
  - `TestLoadFrom_WithComplypacks` -- config loading with complypacks section
  - `TestLoadFrom_WithoutComplypacks` -- config loading without complypacks section

## 3. Implement complypack cache

- [x] 3.1 Create `internal/cache/complypack.go` with `ComplypackCache` struct managing `~/.complytime/complypacks/{evaluator-id}/{version}/` directory structure
- [x] 3.2 Implement `ValidatePathComponent(value string) error` that rejects evaluator-id and version values containing path separators (`/`, `\`), parent directory references (`..`), null bytes, or characters outside `[a-zA-Z0-9._-]`
- [x] 3.3 Implement `Store(config complypack.Config, content io.Reader) (string, error)` that validates evaluator-id and version via `ValidatePathComponent`, writes `content.tar.gz` and `config.json` to a temporary directory first, then atomically renames to the final cache path. Returns the content path
- [x] 3.4 Implement `Lookup(evaluatorID, version string) (string, *complypack.Config, error)` that finds the cached complypack content path and config for a specific evaluator-id and version
- [x] 3.5 Extend existing `State` struct in `internal/cache/state.go` with `Complypacks map[string]PolicyState` field for tracking complypack sync state alongside policy state in the single `state.json` file. Add `UpdateComplypackState(evaluatorID, version, digest string)` and `GetComplypackState(evaluatorID string) (PolicyState, bool)` accessor methods mirroring the existing policy state accessors
- [x] 3.6 [P] Add unit tests for cache:
  - `TestValidatePathComponent_Valid` -- accepts `io.complytime.opa`, `1.0.0`, `my-pack`
  - `TestValidatePathComponent_PathTraversal` -- rejects `../../etc`, `foo/bar`, `foo\bar`
  - `TestValidatePathComponent_NullByte` -- rejects strings with null bytes
  - `TestComplypackCache_Store_CreatesDirectoryStructure` -- verify `{evaluator-id}/{version}/content.tar.gz` and `config.json` exist
  - `TestComplypackCache_Store_AtomicWrite` -- verify incomplete writes don't leave partial cache entries
  - `TestComplypackCache_Store_MultipleVersions` -- store v1.0.0 and v2.0.0 for same evaluator-id
  - `TestComplypackCache_Lookup_Found` -- verify correct path and config returned
  - `TestComplypackCache_Lookup_NotFound` -- evaluator-id with no cached content returns appropriate error
  - `TestState_ComplypackRoundTrip` -- save and load complypack state, verify fields match
  - `TestState_ComplypackLoadMissing_ReturnsEmpty` -- missing complypacks key returns empty map

## 4. Extend get command to pull complypacks

- [x] 4.1 Create `ComplypackSource` in `internal/cache/` for OCI fetch and artifact type verification (mirroring existing `RegistrySource` / `PolicySource` pattern)
- [x] 4.2 Create `ComplypackSync` in `internal/cache/` for the fetch-unpack-store pipeline (mirroring `Sync.SyncPolicy` pattern). Includes incremental sync check -- compare remote manifest digest against state and skip if unchanged
- [x] 4.3 Add complypack sync loop in `cmd/complyctl/cli/get.go` `run()` after the existing policy sync loop, delegating to `ComplypackSync`
- [x] 4.4 Log WARNING for each fetched complypack indicating the artifact has not been cryptographically verified (until complypack library implements signature verification)
- [x] 4.5 Add progress output for complypack fetching (reuse existing spinner/status pattern)
- [x] 4.6 [P] Add unit tests:
  - `TestComplypackSync_FetchAndStore` -- artifact type detection and unpack-to-cache flow using in-memory OCI stores
  - `TestComplypackSync_IncrementalSkip` -- first sync succeeds, second sync with same digest is a no-op
  - `TestComplypackSync_DigestChanged` -- second sync with different digest triggers re-fetch
  - `TestComplypackSync_InvalidEvaluatorID` -- malicious evaluator-id is rejected during store
  - ~~`TestComplypackSync_UnsignedWarning`~~ -- superseded: warning is the CLI layer's responsibility (see `cmd/complyctl/cli/get.go`); the sync layer does not emit warnings

## 5. Extend protobuf with complypack content path

- [x] 5.1 Add `string complypack_content_path` as the next available field number in `GenerateRequest` in `api/plugin/plugin.proto` (currently field 4; verify at implementation time)
- [x] 5.2 Regenerate `api/plugin/plugin.pb.go` and `api/plugin/plugin_grpc.pb.go`
- [x] 5.3 Add `ComplypackContentPath string` field to the domain `GenerateRequest` struct in `pkg/provider/client.go` (distinct from the proto-generated struct)
- [x] 5.4 Update `Client.Generate()` in `pkg/provider/client.go` to map domain `ComplypackContentPath` to the proto request field
- [x] 5.5 [P] Update `grpcServer.Generate()` in `pkg/provider/server.go` to pass the complypack content path through to the provider implementation
- [x] 5.6 [P] Add unit tests for proto field propagation:
  - `TestClient_Generate_ComplypackPathPopulated` -- verify domain field maps to proto request
  - `TestClient_Generate_ComplypackPathEmpty` -- verify backward compatibility when field is empty

## 6. Wire complypack dispatch into scan pipeline

- [x] 6.1 Update the generate phase in `cmd/complyctl/cli/scan.go` to look up cached complypack content path by evaluator-id via `ComplypackCache.Lookup` and populate it on the domain `GenerateRequest.ComplypackContentPath` before passing to `RouteGenerate`
- [x] 6.2 [P] Add unit and integration tests for dispatch:
  - `TestManager_RouteGenerate_WithComplypackPath` -- verify complypack content path is populated on GenerateRequest
  - `TestManager_RouteGenerate_NoComplypackForProvider` -- verify GenerateRequest.ComplypackContentPath is empty when no complypack cached (backward compatible)
  - `TestScan_ComplypackDispatchIntegration` -- integration test verifying complypack content path reaches the test-provider via GenerateRequest

## 7. Enhance providers command

- [x] 7.1 Update `cmd/complyctl/cli/providers.go` to look up cached complypack availability per provider evaluator-id and display version or "none" in the output table
- [x] 7.2 Add unit tests for providers output:
  - `TestProviders_WithComplypack` -- output contains complypack version string for matched provider
  - `TestProviders_WithoutComplypack` -- output indicates no complypack available

## 8. Enhance doctor command

- [x] 8.1 Add `CheckComplypacks` to `internal/doctor/doctor.go` that resolves each policy's evaluator-ids via `PolicyGraphResolver` (same pattern as `CheckVariables`) and verifies matching cached complypacks exist
- [x] 8.2 Emit warning when a complypack is missing, suggesting `complyctl get`
- [x] 8.3 [P] Add unit tests:
  - `TestCheckComplypacks_AllPresent` -- status == StatusPass
  - `TestCheckComplypacks_Missing` -- status == StatusWarn, message contains evaluator-id, message contains "complyctl get" suggestion
  - `TestCheckComplypacks_NilConfig` -- returns nil (following existing nil-guard pattern)
  - `TestCheckComplypacks_NoComplypacks` -- empty complypacks list, check passes

## 9. Update test-provider for complypack support

- [x] 9.1 Update `cmd/test-provider/` to log or acknowledge receipt of `complypack_content_path` in its `Generate` handler
- [x] 9.2 Add E2E test that configures a complypack in `complytime.yaml`, runs `complyctl get` against the mock registry, then runs `complyctl scan` and verifies: (a) the test-provider received a non-empty `complypack_content_path`, (b) the cache directory structure exists after `get`, (c) state.json contains complypack entries

## 10. Documentation

- [x] 10.1 Update `docs/QUICK_START.md` Step 3 (add `complypacks:` example to `complytime.yaml`) and Step 4 (mention complypack fetching alongside policy fetching)
- [x] 10.2 Update `complyctl get --help` long description to mention complypack fetching
- [x] 10.3 [P] Update `complyctl providers --help` and `complyctl doctor --help` long descriptions to mention complypack diagnostics
- [x] 10.4 [P] Update `docs/man/complyctl.md` COMMANDS section to include `get` (with complypack awareness), `providers` (with complypack availability), and `doctor` (with complypack diagnostics) command descriptions

## 11. Final verification

- [x] 11.1 Run `go build ./...` to verify compilation
- [x] 11.2 Run `go test ./...` to verify all tests pass
- [x] 11.3 Run `golangci-lint run` to verify no lint issues (golangci-lint not available; `go vet` passed clean)
- [x] 11.4 Verify no hardcoded evaluator-id assumptions or file-extension-based dispatch were introduced
<!-- spec-review: passed -->
