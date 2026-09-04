## 1. Proto API

- [ ] 1.1 Add `EvidenceMapping` message and `source` field to
  `api/plugin/plugin.proto`; remove stale ADR 0023 comment.
  Update `Evidence` message comment to reference Gemara schema.
  Verify: `buf lint` passes.
- [ ] 1.2 Regenerate Go code via `make proto`. Verify:
  `api/plugin/plugin.pb.go` compiles and contains the new
  `EvidenceMapping` type and `Source` field on `Evidence`.

## 2. Provider SDK

- [ ] 2.1 Add `EvidenceSource` struct to `pkg/provider/client.go`
  with fields `ReferenceID`, `Coordinate`, `EntryID`, `Digest`,
  `Remarks`. Add `Source *EvidenceSource` field to `Evidence`.
  Verify: `go build ./pkg/provider/...` succeeds.
- [ ] 2.2 Update `internalEvidenceToProto` in
  `pkg/provider/server.go` to map `Source` to proto
  `EvidenceMapping`. Verify: existing tests pass, add test for
  source mapping in `proto_mapping_test.go`.
- [ ] 2.3 Update `protoEvidenceToInternal` in
  `pkg/provider/client.go` to map proto `EvidenceMapping` to
  `EvidenceSource`. Verify: add tests for round-trip with source
  populated, source nil, and source with partial fields in
  `proto_mapping_test.go`.

## 3. Evaluator

- [ ] 3.1 Update evidence mapping in
  `internal/output/evaluator.go` to set `gemara.Evidence.Source`
  from `provider.EvidenceSource` when non-nil. Verify: update
  `TestGemaraLog_EvidencePopulated` to assert `Source` is set;
  add test for nil source producing empty `Source`.
- [ ] 3.2 Verify evidence source is serialized in YAML/JSON
  evaluation log output. Verify: update
  `TestEvaluator_Write_EvidenceSerialized` to assert `source:`
  key appears in YAML output.

## 4. Markdown Formatter

- [ ] 4.1 Update `formatEvidenceMeta` in
  `internal/output/markdown.go` to include source provenance
  (reference-id, coordinate when present). Verify: add tests
  in `markdown_test.go` for evidence with source+coordinate,
  source without coordinate, and no source.

## 5. Test Provider and E2E

- [ ] 5.1 Update `cmd/test-provider/main.go` to populate
  `Evidence.Source` with representative values. Verify: test
  provider builds (`make build-test-provider`).
- [ ] 5.2 Run full test suite: `make test-unit` passes with no
  regressions. Run `make test-e2e` to verify evidence source
  appears in end-to-end output.

## 6. Cleanup

- [ ] 6.1 Run `make lint` and `make vet` to verify zero lint
  issues.
- [ ] 6.2 Run `make sanity` to verify no unintended changes.
