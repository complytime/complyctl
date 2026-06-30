## 1. Implement version eviction in Store()

- [ ] 1.1 In `internal/cache/complypack.go` `Store()`, after preparing the temp directory and before `os.RemoveAll` of the target path: list all entries in `{cacheDir}/complypacks/{evaluator-id}/`
- [ ] 1.2 For each entry that is a directory, not hidden (no `.` prefix), and not the target version name: call `os.RemoveAll` on it
- [ ] 1.3 Log a warning (to stderr) if removal fails but do not return an error

## 2. Unit tests

- [ ] 2.1 [P] `TestStore_EvictsOldVersions` — pre-seed `E/1.0.0/`, store `E/2.0.0`, verify `1.0.0/` removed
- [ ] 2.2 [P] `TestStore_EvictsMultipleOldVersions` — pre-seed 3 versions, store new, verify all old removed
- [ ] 2.3 [P] `TestStore_DoesNotAffectOtherEvaluators` — pre-seed `opa/1.0.0` and `ampel/1.0.0`, store `opa/2.0.0`, verify `ampel/` untouched
- [ ] 2.4 [P] `TestStore_SameVersionIdempotent` — store same version twice, no error
- [ ] 2.5 [P] `TestStore_NoExistingDir` — store with no prior evaluator dir, succeeds

## 3. Verification

- [ ] 3.1 Run `go test -race ./internal/cache/` and confirm all tests pass
- [ ] 3.2 Run `go vet ./...`
- [ ] 3.3 E2E: `complyctl get` with version change → verify only new version dir exists
