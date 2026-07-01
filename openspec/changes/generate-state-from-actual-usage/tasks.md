## 1. Thread complypack content availability

- [ ] 1.1 In `cmd/complyctl/cli/generate.go`, track which evaluator-ids had complypack content resolved (non-empty path from `LookupByEvaluatorID`)
- [ ] 1.2 Pass `availableEvaluators []string` to `saveGenerationAndPrint`
- [ ] 1.3 Update `saveGenerationAndPrint` signature to accept `availableEvaluators`

## 2. Filter recorded digests

- [ ] 2.1 In `saveGenerationAndPrint`, filter `complypackDigestsByEvaluator` output to only include evaluator-ids present in `availableEvaluators`
- [ ] 2.2 Apply the same filtering in the `scan.go` code path that saves generation state

## 3. Unit tests

- [ ] 3.1 [P] `TestSaveGenerationAndPrint_AllComplypacksAvailable` — all digests recorded
- [ ] 3.2 [P] `TestSaveGenerationAndPrint_ComplypackUnavailable` — missing evaluator digest is empty
- [ ] 3.3 [P] `TestSaveGenerationAndPrint_MixedAvailability` — only available evaluators' digests recorded
- [ ] 3.4 [P] `TestSaveGenerationAndPrint_NoComplypacks` — nil/empty availableEvaluators, no digests recorded

## 4. Verification

- [ ] 4.1 Run `go test -race ./cmd/complyctl/cli/`
- [ ] 4.2 Run `go vet ./...`
- [ ] 4.3 E2E: delete complypack cache dir, run `generate`, verify generation state has empty complypack digest; then `get` to restore, run `scan`, verify regeneration triggers
