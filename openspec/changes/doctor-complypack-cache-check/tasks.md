## 1. Fix root cause: SyncComplypack cache-existence check

- [x] 1.1 Add `cacheExistsForState(ps PolicyState) bool` to `ComplypackSync` in `internal/cache/complypack_sync.go`
- [x] 1.2 Update incremental skip guard: require `cacheExistsForState(localState)` alongside digest match
- [x] 1.3 [P] `TestComplypackSync_CacheMissing_RefetchesDespiteMatchingDigest` — re-fetches when cache deleted

## 2. Doctor check (evaluated and removed)

A separate `CheckComplypackCacheIntegrity` was implemented and then removed during
review. The existing `CheckComplypacks` already detects missing complypack caches
via `LookupByEvaluatorID`. Adding a second check produced duplicate warnings for
the same condition. With the SyncComplypack root-cause fix in place, `complyctl get`
self-heals automatically — a redundant doctor check adds noise without unique value.

## 3. Verification

- [x] 3.1 Run `go test -race ./internal/cache/`
- [x] 3.2 Run `go vet ./...`
- [x] 3.3 E2E: delete complypack dir → `complyctl doctor` warns → `complyctl get` re-fetches → `complyctl doctor` passes
