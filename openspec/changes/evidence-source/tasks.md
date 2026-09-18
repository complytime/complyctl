## 1. Proto API

- [x] 1.1 Add `EvidenceMapping` message and `source` field to
  `api/plugin/plugin.proto`; remove stale ADR 0023 comment.
  Add proto comment on `coordinate`/`entry_id` noting upstream
  mutual-exclusivity constraint. Update `Evidence` message
  comment to reference Gemara schema. Verify: `buf lint` and
  `buf breaking --against .git#branch=main` both pass.
- [x] 1.2 Regenerate Go code via `make proto`. Verify:
  `api/plugin/plugin.pb.go` compiles and contains the new
  `EvidenceMapping` type and `Source` field on `Evidence`.

## 2. Provider SDK

- [x] 2.1 Add `EvidenceSource` struct to `pkg/provider/client.go`
  with fields `ReferenceID`, `Coordinate`, `EntryID`, `Digest`,
  `Remarks`. Add `Source *EvidenceSource` field to `Evidence`.
  Verify: `go build ./pkg/provider/...` succeeds.
- [x] 2.2 Update `internalEvidenceToProto` in
  `pkg/provider/server.go` to map `Source` to proto
  `EvidenceMapping`. Note: `internalEvidenceToProto` currently
  has no unit tests. Add a table-driven test in
  `proto_mapping_test.go` covering: (a) nil/empty input,
  (b) fully populated evidence without source, (c) fully
  populated evidence with source, (d) evidence with partial
  source fields (only ReferenceID + Coordinate set).
- [x] 2.3 Update `protoEvidenceToInternal` in
  `pkg/provider/client.go` to map proto `EvidenceMapping` to
  `EvidenceSource`. Verify: add tests for round-trip with source
  populated, source nil, and source with partial fields in
  `proto_mapping_test.go`.

## 3. Evaluator

- [x] 3.1 Update evidence mapping in
  `internal/output/evaluator.go` to set `gemara.Evidence.Source`
  from `provider.EvidenceSource` when non-nil. Verify: update
  `TestGemaraLog_EvidencePopulated` to assert `Source` is set;
  add test for nil source producing empty `Source`.
- [x] 3.2 Verify evidence source is serialized in YAML/JSON
  evaluation log output. Verify: update
  `TestEvaluator_Write_EvidenceSerialized` to assert `source:`
  key appears in YAML output. Also verify JSON output
  serializes source fields with kebab-case names
  (`reference-id`, `entry-id`). Add assertion that YAML
  output omits `source:` when provider sends no source.

## 4. Markdown Formatter

- [x] 4.1 Update `formatEvidenceMeta` in
  `internal/output/markdown.go` to include source provenance
  (reference-id, coordinate when present). Verify: add tests
  in `markdown_test.go` for evidence with source+coordinate,
  source without coordinate, and no source.

## 5. Test Provider and E2E

- [x] 5.1 Update `cmd/test-provider/main.go` to populate
  `Evidence.Source` with representative values. Verify: test
  provider builds (`make build-test-provider`).
- [x] 5.2 Run full test suite: `make test-unit` passes with no
  regressions. Run `make test-e2e` and verify the evaluation
  log output file contains `source:` with the reference-id
  value populated by the test provider. Run
  `make test-schema-validation` to verify output passes CUE
  schema validation.

## 6. Cleanup

- [x] 6.1 Run `make lint` and `make vet` to verify zero lint
  issues.
- [x] 6.2 Run `make sanity` to verify no unintended changes.

## 7. Documentation

- [x] 7.1 Add CHANGELOG.md entry under `## Unreleased / ### Added`
  describing evidence source provenance capability (new proto
  `EvidenceMapping` message, `source` field on `Evidence`,
  `EvidenceSource` SDK type, Markdown rendering).
- [x] 7.2 Add AGENTS.md "Recent Changes" entry for
  `evidence-source` documenting the proto API, SDK, evaluator,
  and Markdown formatter changes.

<!-- spec-review: passed -->
<!-- code-review: passed -->
