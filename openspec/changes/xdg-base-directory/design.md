## Context

complyctl currently centralizes all user-scoped paths under `~/.complytime/`
via the `stateSubdir` constant in `internal/complytime/consts.go`. Two
resolver functions -- `ResolveCacheDir()` and `ResolveProviderDir()` -- build
paths from `os.UserHomeDir()` + `stateSubdir`. This mixes ephemeral cache
data (OCI policies, complypacks) with persistent data (provider binaries,
`state.json`) in a single directory tree.

The XDG Base Directory Specification separates user-scoped paths into
categories: cache (`$XDG_CACHE_HOME`, default `~/.cache`), data
(`$XDG_DATA_HOME`, default `~/.local/share`), config
(`$XDG_CONFIG_HOME`, default `~/.config`), and state
(`$XDG_STATE_HOME`, default `~/.local/state`). complyctl's workspace
config lives project-locally (`.complytime/complytime.yaml`) and is
unaffected.

**Supersedes Spec 001 Decision**: The original Spec 001
(`specs/001-gemara-native-workflow/spec.md`, Session 2026-02-27) explicitly
chose `~/.complytime/` over XDG, citing Constitution VII (Convention Over
Configuration) and cross-platform complexity. This decision is being reversed
because: (1) complypack adopted XDG in complytime/complypack#127, creating an
ecosystem inconsistency; (2) ADR-0016 was ratified as the cross-project
convention; (3) the pre-stable release window makes this the right time to
change before users depend on the old layout. XDG is itself a convention on
Linux, so adopting it aligns with the broader ecosystem convention rather than
maintaining an internal one.

The `complypack` sibling project adopted XDG in complytime/complypack#127.
ADR-0016 establishes this as the cross-project convention. All caller sites
(6 for `ResolveCacheDir`, 4 for `ResolveProviderDir`) are in
`cmd/complyctl/cli/` command files, making the change surface well-bounded.

## Goals / Non-Goals

**Goals:**
- Replace `~/.complytime/` with XDG-compliant paths for cache and data.
- Use `os.UserCacheDir()` for cross-platform cache resolution (Linux, macOS,
  Windows handled by Go stdlib).
- Implement manual `XDG_DATA_HOME` resolution with correct per-platform
  fallbacks (Linux, macOS, Windows).
- Separate state.json and providers (data) from policies and complypacks
  (cache).
- Emit a deprecation warning when legacy `~/.complytime/` exists, following the
  existing pattern in `workspace.go:printDeprecationWarning()`. Warning
  distinguishes re-fetchable cache from non-recoverable state data.
- Create XDG directories with `0700` permissions (owner-only, matching existing
  `EnsureDir()` pattern).
- Add `complyctl doctor` checks for split-directory layout verification.
- Maintain backward compatibility: workspace-local `.complytime/` and system
  provider paths are unchanged.

**Non-Goals:**
- Auto-migration of files from `~/.complytime/` to XDG paths. Users move
  files manually or let complyctl re-fetch on next run.
- XDG config directory (`$XDG_CONFIG_HOME`). Workspace config is
  project-scoped, not user-scoped.
- XDG state directory (`$XDG_STATE_HOME`). Log files are workspace-local.
- Changes to the provider gRPC API or OCI registry client.
- Updating path references in completed/archived spec artifacts (treated as
  historical records; this spec supersedes their path guidance).

## Decisions

### D1: Use `os.UserCacheDir()` for cache, manual resolution for data

Go stdlib provides `os.UserCacheDir()` which returns `$XDG_CACHE_HOME`
(Linux), `$HOME/Library/Caches` (macOS), or `%LocalAppData%` (Windows).
There is no equivalent `os.UserDataDir()` in Go's stdlib.

For data directories, implement `ResolveDataDir()` that checks
`$XDG_DATA_HOME` first (ignoring empty strings and relative paths per the
XDG specification), then falls back to `$HOME/.local/share` (Linux),
`$HOME/Library/Application Support` (macOS), or `%LocalAppData%` (Windows).

Both functions return `(string, error)` and propagate errors when the
underlying home directory cannot be resolved (e.g., `$HOME` unset in
containerized environments).

**Alternative considered**: Using a third-party XDG library (e.g.,
`github.com/adrg/xdg`). Rejected because complyctl only needs two XDG
categories (cache, data) and adding a dependency for two `os.Getenv` calls
is unnecessary overhead. The issue specification also explicitly calls for
this approach.

### D2: Separate cache and data callers at the CLI layer

Currently, `ResolveCacheDir()` returns the single root for everything.
After the change, callers that need state.json or provider paths must use
`ResolveDataDir()` or `ResolveProviderDir()` respectively. The `doctor`
command already receives `cacheDir` and `providerDir` as separate
parameters; it gains a `dataDir` parameter for state.json.

CLI commands that call `cache.LoadState(cacheDir)` must switch to
`cache.LoadState(dataDir)`. CLI commands that call `cache.SaveState(state,
cacheDir)` must switch to `cache.SaveState(state, dataDir)`. Affected
commands: `scan`, `get`, `providers`, `generate`, `list`, `doctor`.

**Alternative considered**: Keep state.json in the cache directory so
callers don't change. Rejected because state.json is persistent metadata
(verified digests, signer identities) that should survive cache clearing.
XDG semantics: cache is deletable without data loss; data is not.

### D3: Deprecation warning without auto-migration

On first invocation, if `~/.complytime/` exists AND (the new XDG cache
directory OR the new XDG data directory does not exist), print a
deprecation warning to stderr. The warning distinguishes between:
- Cache data (policies, complypacks): re-fetchable, safe to delete.
- State data (state.json, providers): non-recoverable, must be copied.

The warning is structurally once-per-invocation by being wired into
`PersistentPreRun` in `root.go`, which executes exactly once per CLI
invocation (same structural guarantee as the existing workspace config
deprecation warning). The non-error variant is appropriate since the
deprecation check prints to stderr and does not return an error.

No auto-migration because:
- Cache data (policies, complypacks) is re-fetchable. A fresh `get`
  repopulates.
- state.json is small and can be moved manually.
- Provider binaries are installed via `complyctl init` or package manager.
- Auto-migration risks permission issues and partial-move corruption.

**Alternative considered**: Silent fallback (check legacy dir first, then
XDG). Rejected because silent fallback creates indefinite dual-path
maintenance and users never learn about the new convention.

### D4: Remove `stateSubdir` const, keep `WorkspaceDir` const

The `stateSubdir` const (`.complytime`) is only used by `ResolveCacheDir()`
and `ResolveProviderDir()`. After XDG adoption, these functions no longer
need it. Remove `stateSubdir` and use the string `"complytime"` (without
leading dot) as the XDG subdirectory name, defined as a new
`xdgAppName` const.

`WorkspaceDir` (also `.complytime`) stays unchanged -- it serves the
distinct purpose of naming the workspace-local directory.

### D5: Path mapping

| Data | XDG Category | Resolution | Default (Linux) |
|------|-------------|------------|-----------------|
| Policies | Cache | `ResolveCacheDir()` + `policies/` | `~/.cache/complytime/policies/` |
| Complypacks | Cache | `ResolveCacheDir()` + `complypacks/` | `~/.cache/complytime/complypacks/` |
| state.json | Data | `ResolveDataDir()` + `state.json` | `~/.local/share/complytime/state.json` |
| Providers | Data | `ResolveDataDir()` + `providers/` | `~/.local/share/complytime/providers/` |
| Workspace config | Project-local | `{workspace}/.complytime/complytime.yaml` | unchanged |
| Logs | Project-local | `{workspace}/.complytime/complyctl.log` | unchanged |

### D6: Directory permissions

XDG directories are created with `0700` (owner-only access), consistent
with the existing `Workspace.EnsureDir()` pattern. This is appropriate
because:
- The data directory contains `state.json` with verification metadata
  (signer identities, digests) and provider binaries.
- The cache directory contains OCI policy artifacts.
- `0700` prevents other users on multi-user systems from reading
  compliance data.

## Risks / Trade-offs

- **[Risk] Users with existing `~/.complytime/` must re-fetch or manually move**
  -> Mitigation: Deprecation warning with clear instructions distinguishing
  re-fetchable cache from non-recoverable state. Cache data is re-fetchable
  via `complyctl get`. state.json must be manually copied.

- **[Risk] state.json verification metadata silently lost if user follows
  "remove and re-fetch" path** -> Mitigation: Deprecation warning explicitly
  warns that verification metadata will be lost if state.json is not copied.
  `complyctl doctor` detects misplaced state.json.

- **[Risk] E2E and integration tests hardcode `~/.complytime` paths**
  -> Mitigation: Tests already use `t.TempDir()` or `$TEST_HOME` for
  isolation. Update path construction in test helpers to use the resolver
  functions or the new XDG paths.

- **[Risk] CI environments may not set XDG variables**
  -> Mitigation: `os.UserCacheDir()` and `ResolveDataDir()` both have
  platform-appropriate fallbacks that do not require XDG env vars.

- **[Risk] Shell scripts (devcontainer, integration tests) use hardcoded paths**
  -> Mitigation: Update `.devcontainer/scripts/post-create.sh` and test
  scripts to use `$XDG_DATA_HOME` with fallback, matching the Go code's
  resolution logic. Platform-detect macOS vs Linux for cache paths.

- **[Trade-off] No auto-migration increases friction for existing users**
  -> Accepted: The user base is small (pre-stable), cache is re-fetchable,
  and auto-migration adds complexity disproportionate to the benefit.

- **[Trade-off] Constitution VII (Convention Over Configuration) tension**
  -> Accepted: XDG is itself the dominant convention on Linux. Adopting the
  ecosystem-wide convention (XDG) over an internal convention
  (`~/.complytime/`) better serves Constitution VII's intent. The
  cross-project alignment with complypack (ADR-0016) reinforces this.

- **[Risk] Downgrade to pre-XDG version requires manual file moves**
  -> Accepted: Downgrading the binary would require manually recreating
  `~/.complytime/` and moving files back. This is acceptable for a
  pre-stable release. Documented in CHANGELOG migration note.

- **[Risk] Sibling spec artifacts contain stale `~/.complytime` references**
  -> Accepted: Completed specs (001, 004, 005) and archived OpenSpec changes
  are treated as historical records. This change supersedes their path
  guidance. No automated update of archived specs.
