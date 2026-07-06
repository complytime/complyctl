## 1. Add evaluator-id uniqueness validation

- [x] 1.1 Add `validateUniqueEvaluatorIDs(state *cache.State, complypacks []complytime.PolicyEntry) error` to `cmd/complyctl/cli/get.go`
- [x] 1.2 Build a map of evaluator-id → []repository from `state.Complypacks`
- [x] 1.3 For entries with len > 1, format error listing all conflicting repositories
- [x] 1.4 Call `validateUniqueEvaluatorIDs` after `syncAllComplypacks` returns in `syncComplypacks`

## 2. Unit tests

- [x] 2.1 [P] `TestValidateUniqueEvaluatorIDs_NoDuplicates` — passes cleanly
- [x] 2.2 [P] `TestValidateUniqueEvaluatorIDs_DuplicateDetected` — returns error with both repos
- [x] 2.3 [P] `TestValidateUniqueEvaluatorIDs_SingleEntry` — passes
- [x] 2.4 [P] `TestValidateUniqueEvaluatorIDs_EmptyState` — passes

## 3. Verification

- [x] 3.1 Run `go test -race ./cmd/complyctl/cli/`
- [x] 3.2 Run `go vet ./...`
- [x] 3.3 E2E: configure two complypacks with same evaluator-id, run `complyctl get`, verify error
