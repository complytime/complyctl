# Tasks

## 1. Proto API

- [x] 1.1 Add `MappingReference` message to `api/plugin/plugin.proto`
  with fields `id` (1), `title` (2), `version` (3), `description`
  (4), `url` (5). Add `repeated MappingReference mapping_references = 3`
  to `ScanResponse`. Add proto comments mirroring go-gemara
  MappingReference field docs.
  Verify: `buf lint` passes.
- [x] 1.2 Regenerate Go code via `make proto`. Verify:
  `api/plugin/plugin.pb.go` compiles and contains the new
  `MappingReference` type and `MappingReferences` field on
  `ScanResponse`.

## 2. Provider SDK

- [x] 2.1 Add `MappingReference` struct to `pkg/provider/client.go`
  with fields `ID`, `Title`, `Version`, `Description`, `URL` (all
  strings). Add `MappingReferences []MappingReference` field to
  `ScanResponse`. Verify: `go build ./pkg/provider/...` succeeds.
- [x] 2.2 Add `internalMappingRefsToProto` in
  `pkg/provider/server.go` to map `[]MappingReference` to proto
  `[]*proto.MappingReference`. Update `grpcServer.Scan()` to
  include `MappingReferences` in the returned `proto.ScanResponse`.
  Add table-driven tests in `proto_mapping_test.go` covering:
  (a) nil/empty input, (b) fully populated references,
  (c) partial fields (only ID + Title set).
  Verify: `go test ./pkg/provider/... -run TestMappingRef` passes.
- [x] 2.3 Add `protoMappingRefsToInternal` in
  `pkg/provider/client.go` to map `[]*proto.MappingReference` to
  `[]MappingReference`. Update `Client.Scan()` to populate
  `ScanResponse.MappingReferences` from the proto response.
  Add round-trip tests in `proto_mapping_test.go` covering:
  fully populated, partial fields, and empty/nil input.
  Verify: `go test ./pkg/provider/... -run TestMappingRef` passes.

## 2b. Provider Manager

- [x] 2.4 Add `MappingReferences []MappingReference` field to
  `ScanResult` in `pkg/provider/manager.go`. Update
  `RouteScanResult()` to populate `ScanResult.MappingReferences`
  from `resp.MappingReferences` in both the targeted (single
  evaluator) and broadcast (all providers) code paths. Add tests
  in `manager_test.go` covering: targeted scan with refs, broadcast
  aggregation of refs from multiple providers, nil/empty refs.
  Verify: `go test ./pkg/provider/... -run TestScanResult` passes.

## 3. Merge Logic

- [x] 3.1 Add `MappingCollision` struct and
  `MergeMappingReferences(policyRefs []gemara.MappingReference,
  providerRefs []provider.MappingReference)
  ([]gemara.MappingReference, []MappingCollision)` to
  `internal/output/scan_summary.go`. The function MUST:
  (a) convert provider refs to gemara type,
  (b) deduplicate by `id` with policy-wins priority,
  (c) prepend collision note to retained entry's `Description`,
  (d) return collision report for warning channels.
  Add table-driven tests covering: no collision, policy-provider
  collision, provider-provider collision, empty inputs,
  description prepend (not overwrite).
  Verify: `go test ./internal/output/... -run TestMerge` passes.
- [x] 3.2 Add `FormatMappingCollisions(collisions []MappingCollision)
  string` to `internal/output/scan_summary.go` following the
  `FormatOperationalWarnings()` pattern. Returns empty string when
  no collisions. Add tests covering: no collisions, single
  collision, multiple collisions.
  Verify: `go test ./internal/output/... -run TestFormatMapping`
  passes.

## 4. Scan Pipeline

- [x] 4.1 Add `mappingReferences []provider.MappingReference` field
  to `scanOutput` struct in `cmd/complyctl/cli/scan.go`.
  Update `scanSingleTarget()` to return a 4th value
  `[]provider.MappingReference` collected from
  `scanResult.MappingReferences` across evaluator groups. Sort
  evaluator IDs before iteration for deterministic ordering (D6).
  Update `scanAllTargets()` to aggregate provider refs into
  `scanOutput.mappingReferences`. Update all callers of
  `scanSingleTarget`.
  Verify: `go build ./cmd/complyctl/...` succeeds. The threading
  logic is validated transitively through E2E tests (task 5.2).
- [x] 4.2 Update `processScanOutput()` to call
  `MergeMappingReferences(mappingRefs, scanOut.mappingReferences)`
  before `buildEvaluators()`. Add `reportMappingCollisions()`
  helper (prints stderr warning + logger.Warn per collision),
  called between `reportOperationalWarnings()` and
  `buildEvaluators()`. Pass merged refs to `buildEvaluators()`.
  Verify: `go build ./cmd/complyctl/...` succeeds.

## 5. Test Provider and E2E

- [x] 5.1 Update `cmd/test-provider/main.go` to populate
  `ScanResponse.MappingReferences` with a representative entry
  (e.g., `{ID: "test-source", Title: "Test Data Source"}`).
  Verify: `make build-test-provider` succeeds.
- [x] 5.2 Run full test suite: `make test-unit` passes with no
  regressions. Run `make test-e2e` and verify the evaluation log
  output file contains `mapping-references:` with the
  `test-source` entry from the test provider alongside any policy
  references. Run `make test-schema-validation` to verify output
  passes CUE schema validation (unique ref IDs enforced).

## 6. Cleanup

- [x] 6.1 Run `make lint` and `make vet` to verify zero lint issues.
- [x] 6.2 Run `make sanity` to verify no unintended changes.
- [x] 6.3 Run `make crapload-check` to verify new functions do not
  exceed CRAP threshold of 30.

## 7. Documentation

- [x] 7.1 Add CHANGELOG.md entry under `## Unreleased / ### Added`
  describing provider-side MappingReferences capability (new proto
  `MappingReference` message, `mapping_references` field on
  `ScanResponse`, SDK type, merge logic with collision warnings).
- [x] 7.2 Add AGENTS.md "Recent Changes" entry for
  `provider-mapping-references` documenting the proto API, SDK,
  merge logic, and scan pipeline changes.

<!-- spec-review: passed -->
<!-- code-review: passed -->
