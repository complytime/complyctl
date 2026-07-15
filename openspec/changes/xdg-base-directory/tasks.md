<!-- Tasks use `[P]` to mark parallel-eligible tasks.
     [P] tasks within the same section have no dependencies on each other
     and can be executed concurrently. Non-[P] tasks are sequential. -->

## 1. Core Path Resolution

- [x] 1.1 Add `xdgAppName` const (`"complytime"`) and `ResolveDataDir()` function in `internal/complytime/consts.go` that checks `$XDG_DATA_HOME` (ignoring empty strings and relative paths per XDG spec), falls back to `$HOME/.local/share` (Linux), `$HOME/Library/Application Support` (macOS), or `%LocalAppData%` (Windows), and joins with `xdgAppName`. Returns `(string, error)` propagating `os.UserHomeDir()` failures.
- [x] 1.2 Rewrite `ResolveCacheDir()` in `internal/complytime/consts.go` to use `os.UserCacheDir()` joined with `xdgAppName` instead of `os.UserHomeDir()` + `stateSubdir`. Propagate `os.UserCacheDir()` errors.
- [x] 1.3 Rewrite `ResolveProviderDir()` in `internal/complytime/consts.go` to use `ResolveDataDir()` joined with `providerSubdir` instead of `os.UserHomeDir()` + `stateSubdir` + `providerSubdir`
- [x] 1.4 Remove `stateSubdir` const from `internal/complytime/consts.go`
- [x] 1.5 Add unit tests for `ResolveDataDir()`, `ResolveCacheDir()`, and `ResolveProviderDir()` in `internal/complytime/consts_test.go`: one test case per spec scenario (FR-001 through FR-003), including error paths (`$HOME` unset), empty `$XDG_DATA_HOME`, relative `$XDG_DATA_HOME` ignored; use `t.Setenv()` for env var manipulation; table-driven structure per go.md TC-006

## 2. State File Separation

- [x] 2.1a Update CLI-layer callers to pass `ResolveDataDir()` result for state.json operations -- update `cache.LoadState()` call sites in `cmd/complyctl/cli/scan.go`, `get.go`, `providers.go`, `generate.go`, `list.go`, `doctor.go` AND `cache.SaveState()` call sites (at minimum `get.go`)
- [x] 2.1b Update internal `SaveState()` callers in `internal/cache/sync.go` and `internal/cache/complypack_sync.go` to write state.json to the data directory instead of the cache directory -- these are the primary state mutation paths during `complyctl get` and must receive a `dataDir` parameter or have their `Dir()` usage redirected
- [x] 2.2 Update `internal/doctor/doctor.go` `Run()` function to accept a `dataDir` parameter for state.json resolution; update all internal `LoadState(cacheDir)` calls within `doctor.go` to use `dataDir`; update the caller in `cmd/complyctl/cli/doctor.go`
- [x] 2.3 Update `SaveState()` in `internal/cache/state.go` to use `0700` permissions in its `os.MkdirAll` call (currently `0755`) to comply with FR-007; rename the `cacheDir` parameter to `baseDir` for clarity since it now receives either cache or data directory
- [x] 2.4 Add test verifying that state.json is written to `ResolveDataDir()` path and is NOT present in `ResolveCacheDir()` path after a state save operation (regression protection for FR-004 "State survives cache clear")

## 3. Legacy Path Migration Warning

- [x] 3.1 Add `CheckLegacyDir()` function that detects `~/.complytime/` existence AND absence of EITHER XDG cache dir OR XDG data dir; warning text distinguishes re-fetchable cache from non-recoverable state.json; follows `printDeprecationWarning()` pattern in `workspace.go`
- [x] 3.2 Wire `CheckLegacyDir()` into `PersistentPreRun` in `cmd/complyctl/cli/root.go` (structural once-per-invocation guarantee; the check prints to stderr and does not return an error, so the non-error `PersistentPreRun` variant is appropriate)
- [x] 3.3 Add unit tests for legacy detection: (a) legacy exists + XDG absent -> warn, (b) legacy exists + XDG exists -> no warn, (c) legacy absent -> no warn, (d) legacy exists + cache exists but data absent -> warn, (e) verify warning message content contains expected paths, (f) verify output goes to stderr not stdout, (g) verify legacy directory contents are unchanged after warning

## 4. Hardcoded String Cleanup

- [x] [P] 4.1 Update error message in `cmd/complyctl/cli/init.go:52` to use `complytime.WorkspaceDir` instead of hardcoded `.complytime`
- [x] [P] 4.2 Update timeout hint in `pkg/provider/manager.go:267` to use `complytime.WorkspaceDir` + `complytime.LogFileName` instead of hardcoded `.complytime/complyctl.log`
- [x] [P] 4.3 Update code comments referencing `~/.complytime` in `internal/doctor/doctor.go` and `cmd/test-provider/main.go` to reflect new XDG paths

## 5. Test Updates

- [x] [P] 5.1 Update path assertions in `internal/complytime/config_test.go` (lines 1291-1304) for new resolver return values
- [x] [P] 5.2 Update E2E test home-directory paths in `tests/e2e/e2e_test.go` and `tests/e2e/helpers_test.go` -- change cache paths (`policies/`, `complypacks/`) to XDG cache dir and data paths (`providers/`, `state.json`) to XDG data dir; leave workspace-local `.complytime/` paths unchanged
- [x] [P] 5.3 Update behavioral test paths in `tests/behavioral/supply_chain.go`, `tests/behavioral/reusable_steps.go`, `tests/behavioral/plugin_security.go`
- [x] [P] 5.4 Update `pkg/provider/client_test.go` hardcoded path in test assertion
- [x] 5.5 Run `make test-unit` and fix any remaining test failures

## 6. Shell Script and CI Updates

- [x] [P] 6.1 Update `.devcontainer/scripts/post-create.sh` lines 76 and 81 to use `${XDG_DATA_HOME:-$HOME/.local/share}/complytime/providers` instead of `${HOME}/.complytime/providers`
- [x] [P] 6.2 Update `tests/integration_test.sh` to set `$HOME=$TEST_HOME` and use XDG defaults; create `$TEST_HOME/.local/share/complytime/providers` for provider binaries and `$TEST_HOME/.cache/complytime` for cache paths
- [x] [P] 6.3 Update `tests/cross-repo/cross_repo_integration_test.sh` to use XDG-style paths with same fixture strategy as 6.2
- [x] [P] 6.4 Update `.github/actions/setup-complyctl/action.yml` to use XDG-style paths; detect platform for macOS runners (cache uses `$HOME/Library/Caches` on macOS, not `$HOME/.cache`)

## 7. Documentation

- [x] [P] 7.1 Update `README.md` path references from `~/.complytime` to XDG paths, including the architecture diagram
- [x] [P] 7.2 Update `docs/QUICK_START.md` path references
- [x] [P] 7.3 Update `docs/TESTING_ENVIRONMENT.md` path references
- [x] [P] 7.4 Update `docs/man/complyctl.md` path references
- [x] [P] 7.5 Add migration note to `CHANGELOG.md` describing the path change, manual migration steps (copy state.json before removing legacy dir), and note that downgrading requires manual file moves
- [x] [P] 7.6 Update `AGENTS.md` Architecture section (line 273 `~/.complytime/policies/`) and Recent Changes section with xdg-base-directory change summary
- [x] 7.7 Add supersession note to `specs/001-gemara-native-workflow/spec.md` Session 2026-02-27 XDG decision indicating it is superseded by this change and ADR-0016

## 8. Doctor Split-Layout Verification

- [x] 8.1 Add `CheckDirectoryLayout()` to `internal/doctor/doctor.go` that verifies both XDG cache and data directories are accessible, and detects state.json in cache dir but not data dir (FR-008)
- [x] 8.2 Add unit test for `CheckDirectoryLayout()` covering: both dirs accessible, cache missing, data missing, state.json misplaced

## 9. XDG Directory Creation

- [x] 9.1 Ensure XDG directories are created with `0700` permissions in `ResolveCacheDir()` and `ResolveDataDir()` callers (or in a shared `EnsureXDGDir()` helper), consistent with `Workspace.EnsureDir()` pattern (FR-007)
- [x] 9.2 Add test verifying directory creation uses `0700` permissions

## 10. Final Verification

- [x] 10.1 Run `make lint` and fix any lint issues
- [x] 10.2 Run `make test-unit` and confirm all tests pass
- [x] 10.3 Run `make sanity` to confirm no unintended changes
- [x] 10.4 Grep for remaining hardcoded `~/.complytime` in all non-vendor files (Go, shell, YAML, Markdown, GitHub Actions) and fix any stragglers

<!-- spec-review: passed -->
<!-- code-review: passed -->
