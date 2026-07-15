## Why

complyctl stores all user-scoped data under `$HOME/.complytime`, mixing cache
(OCI policies, complypacks) and data (provider binaries, `state.json`) in a
single directory. This violates the
[XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/),
which is the standard convention for user-scoped paths on Linux and is respected
by most modern CLI tools. The `complypack` sibling project has already adopted
XDG in [complytime/complypack#127](https://github.com/complytime/complypack/pull/127).
Without this change, complytime ships two incompatible directory conventions at
its first stable release. This implements the complyctl side of
[ADR-0016](https://github.com/complytime/complytime/pull/41) and resolves
[#734](https://github.com/complytime/complyctl/issues/734).

## Supersedes

This change supersedes the Session 2026-02-27 decision in
`specs/001-gemara-native-workflow/spec.md` which chose `~/.complytime/`
(single dot-prefixed directory) over XDG, citing Constitution VII (Convention
Over Configuration) and cross-platform complexity concerns. The reasoning has
changed because:
- The `complypack` sibling project adopted XDG (complypack#127), creating an
  inconsistency within the complytime ecosystem.
- ADR-0016 was ratified as the cross-project convention for directory layout.
- XDG is itself a convention on Linux -- adopting it aligns with the broader
  ecosystem convention rather than an internal one.
- The pre-stable release window makes this the right time to break the path
  convention before users depend on the old layout.

## What Changes

- `ResolveCacheDir()` returns `os.UserCacheDir()/complytime` (e.g.
  `~/.cache/complytime`) instead of `~/.complytime`.
- New `ResolveDataDir()` function returns `$XDG_DATA_HOME/complytime` (fallback
  `~/.local/share/complytime` on Linux, `~/Library/Application Support/complytime`
  on macOS, `%LocalAppData%\complytime` on Windows) for provider binaries and
  `state.json`.
- `ResolveProviderDir()` returns `ResolveDataDir()/providers` instead of
  `~/.complytime/providers`.
- `state.json` is read from and written to the XDG data directory.
- Deprecation warning printed to stderr when legacy `~/.complytime/` exists and
  XDG directories do not, following the existing `printDeprecationWarning()`
  pattern in `workspace.go`. Warning distinguishes re-fetchable cache from
  non-recoverable state data.
- XDG directories created with `0700` permissions (owner-only).
- `complyctl doctor` verifies split directory layout and detects misplaced
  `state.json`.
- All hardcoded `.complytime` strings in user-facing messages replaced with
  resolver function calls.
- Test path assertions, shell scripts, and documentation updated to reflect new
  paths.
- Workspace-local `.complytime/` (per-project directory) is **unchanged**.
- System provider directory `/usr/libexec/complytime/providers/` is **unchanged**.

## Capabilities

### New Capabilities
- `xdg-path-resolution`: XDG-compliant path resolution for user-scoped cache and
  data directories, with cross-platform support via `os.UserCacheDir()` and
  manual `XDG_DATA_HOME` resolution with platform-appropriate fallbacks (Linux,
  macOS, Windows). Error handling for unset `$HOME` and invalid `$XDG_DATA_HOME`.
  Secure directory creation with `0700` permissions. Doctor verification of
  split directory layout.
- `legacy-path-migration`: Deprecation warning and migration guidance when legacy
  `~/.complytime/` directory is detected. Warning distinguishes re-fetchable
  cache data from non-recoverable verification state. Expanded trigger condition
  covers partial migration (data dir missing but cache dir exists).

### Modified Capabilities
- `ResolveCacheDir()`: Return value changes from `~/.complytime` to
  `os.UserCacheDir()/complytime`.
- `ResolveProviderDir()`: Return value changes from `~/.complytime/providers` to
  `ResolveDataDir()/providers`.
- `state.json` location: Moves from cache directory to data directory; callers
  must pass separate data directory parameter.
- `complyctl doctor`: Gains data directory parameter and split-layout
  verification checks.

## Impact

- **Core path resolution**: `internal/complytime/consts.go` -- `ResolveCacheDir()`,
  `ResolveProviderDir()`, new `ResolveDataDir()`, removal of `stateSubdir` const.
- **State management**: `internal/cache/state.go` -- state.json moves from cache
  dir to data dir; callers in `cmd/complyctl/cli/` commands must pass separate
  data dir.
- **Doctor diagnostics**: `internal/doctor/doctor.go` -- gains `dataDir` parameter
  for state.json resolution and split-layout verification.
- **User-facing messages**: `cmd/complyctl/cli/init.go`, `pkg/provider/manager.go`
  -- hardcoded `.complytime` strings updated.
- **Tests**: `internal/complytime/consts_test.go`, `internal/cache/cache_test.go`,
  `internal/cache/complypack_test.go`, `pkg/provider/discovery_test.go`,
  `tests/e2e/`, `tests/behavioral/`, `tests/integration_test.sh`,
  `tests/cross-repo/`.
- **Shell scripts**: `.devcontainer/scripts/post-create.sh`,
  `.github/actions/setup-complyctl/action.yml`.
- **Documentation**: `README.md`, `docs/QUICK_START.md`,
  `docs/TESTING_ENVIRONMENT.md`, `docs/man/complyctl.md`, `AGENTS.md`,
  `CHANGELOG.md`.
- **Cross-repo documentation**: `unbound-force/website` issue filed for
  user-facing path documentation updates.
- **Sibling specs**: `specs/001-gemara-native-workflow/` and other spec artifacts
  referencing `~/.complytime/` are superseded by this change and treated as
  historical records for their path references.
- **Dependencies**: No new dependencies. Uses `os.UserCacheDir()` (stdlib) and
  `os.Getenv("XDG_DATA_HOME")` with manual platform fallback.
