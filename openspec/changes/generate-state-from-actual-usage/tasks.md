## 1. Thread complypack content availability

- [x] 1.1 In `cmd/complyctl/cli/generate.go`, track which evaluator-ids had complypack content resolved (non-empty path from `LookupByEvaluatorID`)
- [x] 1.2 Pass `availableEvaluators []string` to `saveGenerationAndPrint`
- [x] 1.3 Update `saveGenerationAndPrint` signature to accept `availableEvaluators`

## 2. Filter recorded digests

- [x] 2.1 In `saveGenerationAndPrint`, filter `complypackDigestsByEvaluator` output to only include evaluator-ids present in `availableEvaluators`
- [x] 2.2 Apply the same filtering in the `scan.go` code path that saves generation state

## 3. Unit tests

- [x] 3.1 [P] `TestSaveGenerationAndPrint_AllComplypacksAvailable` — all digests recorded
- [x] 3.2 [P] `TestSaveGenerationAndPrint_ComplypackUnavailable` — missing evaluator digest is empty
- [x] 3.3 [P] `TestSaveGenerationAndPrint_MixedAvailability` — only available evaluators' digests recorded
- [x] 3.4 [P] `TestSaveGenerationAndPrint_NoComplypacks` — nil/empty availableEvaluators, no digests recorded

## 4. Verification

- [x] 4.1 Run `go test -race ./cmd/complyctl/cli/`
- [x] 4.2 Run `go vet ./...`
- [x] 4.3 E2E: delete complypack cache dir, run `generate`, verify generation state has empty complypack digest; then `get` to restore, run `scan`, verify regeneration triggers
