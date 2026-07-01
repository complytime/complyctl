## 1. Implement version eviction in Store()

- [x] 1.1 In `internal/cache/complypack.go` `Store()`, after preparing the temp directory and before `os.RemoveAll` of the target path: list all entries in `{cacheDir}/complypacks/{evaluator-id}/`
- [x] 1.2 For each entry that is a directory, not hidden (no `.` prefix), and not the target version name: call `os.RemoveAll` on it
- [x] 1.3 Log a warning (to stderr) if removal fails but do not return an error

## 2. Unit tests

- [x] 2.1 [P] `TestStore_EvictsOldVersions` — pre-seed `E/1.0.0/`, store `E/2.0.0`, verify `1.0.0/` removed
- [x] 2.2 [P] `TestStore_EvictsMultipleOldVersions` — pre-seed 3 versions, store new, verify all old removed
- [x] 2.3 [P] `TestStore_DoesNotAffectOtherEvaluators` — pre-seed `opa/1.0.0` and `ampel/1.0.0`, store `opa/2.0.0`, verify `ampel/` untouched
- [x] 2.4 [P] `TestStore_SameVersionIdempotent` — store same version twice, no error
- [x] 2.5 [P] `TestStore_NoExistingDir` — store with no prior evaluator dir, succeeds

## 3. Verification

- [x] 3.1 Run `go test -race ./internal/cache/` and confirm all tests pass
- [x] 3.2 Run `go vet ./...`
- [x] 3.3 E2E: `complyctl get` with version change → verify only new version dir exists
