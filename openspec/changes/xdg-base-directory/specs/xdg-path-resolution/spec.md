## ADDED Requirements

### FR-001: Cache directory uses XDG cache location
`ResolveCacheDir()` MUST return `os.UserCacheDir()` joined with the
`xdgAppName` constant (`"complytime"`) as the cache root for policies
and complypacks. The function MUST NOT reference `os.UserHomeDir()`
or the legacy `.complytime` directory.

#### Scenario: Default cache path on Linux
- **GIVEN** a Linux system with default XDG configuration
- **WHEN** `$XDG_CACHE_HOME` is unset
- **THEN** `ResolveCacheDir()` returns `$HOME/.cache/complytime`

#### Scenario: Custom XDG_CACHE_HOME on Linux
- **GIVEN** a Linux system
- **WHEN** `$XDG_CACHE_HOME` is set to `/tmp/mycache`
- **THEN** `ResolveCacheDir()` returns `/tmp/mycache/complytime`
  (note: `os.UserCacheDir()` on macOS ignores `$XDG_CACHE_HOME`)

#### Scenario: Cache path on macOS
- **GIVEN** a macOS system with default settings
- **WHEN** `$XDG_CACHE_HOME` is unset
- **THEN** `ResolveCacheDir()` returns
  `$HOME/Library/Caches/complytime`

#### Scenario: Cache resolution error
- **GIVEN** any platform
- **WHEN** `os.UserCacheDir()` returns an error (e.g. `$HOME`
  unset in a containerized environment)
- **THEN** `ResolveCacheDir()` returns `("", error)` with a
  descriptive wrapped error message

### FR-002: Data directory resolves via XDG_DATA_HOME
`ResolveDataDir()` MUST check the `$XDG_DATA_HOME` environment
variable first. If unset or empty, it MUST fall back to the
platform-appropriate default: `$HOME/.local/share` on Linux,
`$HOME/Library/Application Support` on macOS, or `%LocalAppData%`
on Windows. The returned path MUST be the platform-appropriate
data directory joined with the `xdgAppName` constant
(`"complytime"`). Relative paths in `$XDG_DATA_HOME` MUST be
ignored per the XDG specification and treated as unset.

#### Scenario: Default data path on Linux
- **GIVEN** a Linux system with default XDG configuration
- **WHEN** `$XDG_DATA_HOME` is unset
- **THEN** `ResolveDataDir()` returns
  `$HOME/.local/share/complytime`

#### Scenario: Custom XDG_DATA_HOME
- **GIVEN** a Linux or macOS system
- **WHEN** `$XDG_DATA_HOME` is set to `/tmp/mydata`
- **THEN** `ResolveDataDir()` returns `/tmp/mydata/complytime`

#### Scenario: Data path on macOS
- **GIVEN** a macOS system with default settings
- **WHEN** `$XDG_DATA_HOME` is unset
- **THEN** `ResolveDataDir()` returns
  `$HOME/Library/Application Support/complytime`

#### Scenario: Data path on Windows
- **GIVEN** a Windows system
- **WHEN** `$XDG_DATA_HOME` is unset
- **THEN** `ResolveDataDir()` returns
  `%LocalAppData%\complytime`

#### Scenario: Empty XDG_DATA_HOME treated as unset
- **GIVEN** any platform
- **WHEN** `$XDG_DATA_HOME` is set to `""` (empty string)
- **THEN** `ResolveDataDir()` uses the platform default fallback

#### Scenario: Relative XDG_DATA_HOME ignored
- **GIVEN** any platform
- **WHEN** `$XDG_DATA_HOME` is set to a relative path
  (e.g. `relative/path`)
- **THEN** `ResolveDataDir()` ignores it and uses the
  platform default fallback

#### Scenario: Data resolution error
- **GIVEN** any platform
- **WHEN** `$XDG_DATA_HOME` is unset AND `os.UserHomeDir()`
  returns an error
- **THEN** `ResolveDataDir()` returns `("", error)` with a
  descriptive wrapped error message

### FR-003: Provider directory uses data location
`ResolveProviderDir()` MUST return the result of `ResolveDataDir()`
joined with `providers`. It MUST NOT reference `os.UserHomeDir()`
or the legacy `.complytime` directory.

#### Scenario: Default provider path on Linux
- **GIVEN** a Linux system with default XDG configuration
- **WHEN** `$XDG_DATA_HOME` is unset
- **THEN** `ResolveProviderDir()` returns
  `$HOME/.local/share/complytime/providers`

#### Scenario: Provider path with custom XDG_DATA_HOME
- **GIVEN** any platform
- **WHEN** `$XDG_DATA_HOME` is set to `/opt/data`
- **THEN** `ResolveProviderDir()` returns
  `/opt/data/complytime/providers`

### FR-004: State file uses data directory
`cache.LoadState()` and `cache.SaveState()` MUST read and write
`state.json` from the data directory (as returned by
`ResolveDataDir()`), not the cache directory. Callers MUST pass
the data directory for state operations.

#### Scenario: State file location
- **GIVEN** a Linux system with default XDG configuration
- **WHEN** `$XDG_DATA_HOME` is unset
- **THEN** `state.json` is located at
  `$HOME/.local/share/complytime/state.json`

#### Scenario: State survives cache clear
- **GIVEN** state.json exists in the XDG data directory
- **WHEN** a user deletes `$XDG_CACHE_HOME/complytime/`
- **THEN** `state.json` remains at
  `$XDG_DATA_HOME/complytime/state.json`

### FR-005: Workspace-local paths are unchanged
The workspace-local `.complytime/` directory (containing
`complytime.yaml`, `complyctl.log`, and scan outputs) MUST NOT
be affected by XDG path resolution. `WorkspaceDir` MUST remain
`.complytime`.

#### Scenario: Workspace config path unchanged
- **GIVEN** a project directory with a workspace configuration
- **WHEN** user runs any complyctl command
- **THEN** workspace config is read from
  `{workspace}/.complytime/complytime.yaml`

#### Scenario: Log file path unchanged
- **GIVEN** debug logging is enabled
- **WHEN** complyctl writes debug logs
- **THEN** logs are written to
  `{workspace}/.complytime/complyctl.log`

### FR-006: No hardcoded home-relative dotdir in user messages
User-facing error messages and hints MUST NOT contain hardcoded
`~/.complytime` or `.complytime` strings for home-directory paths.
They MUST use resolver functions or reference the workspace-local
path via the `WorkspaceDir` constant.

#### Scenario: Init error message uses constant
- **GIVEN** a project directory
- **WHEN** `complyctl init` fails to create the workspace directory
- **THEN** the error message references `WorkspaceDir` or the
  resolved path, not a hardcoded `.complytime` string

#### Scenario: Provider timeout hint uses resolved path
- **GIVEN** a scan in progress
- **WHEN** a provider times out during scan
- **THEN** the hint message references the workspace-local log
  path using `WorkspaceDir` and `LogFileName` constants

### FR-007: XDG directory creation with secure permissions
When the XDG cache or data directory does not exist, complyctl
MUST create it with `0700` permissions (owner-only access),
consistent with the existing workspace directory creation
pattern in `Workspace.EnsureDir()`.

#### Scenario: First-run directory creation
- **GIVEN** a fresh system with no existing complytime directories
- **WHEN** complyctl runs for the first time
- **THEN** `~/.cache/complytime/` and
  `~/.local/share/complytime/` are created with `0700` permissions

#### Scenario: Subdirectory creation preserves permissions
- **GIVEN** the XDG data directory exists with `0700` permissions
- **WHEN** complyctl creates the providers subdirectory
- **THEN** `~/.local/share/complytime/providers/` is created
  with `0700` permissions

### FR-008: Doctor verifies split directory layout
`complyctl doctor` MUST verify that both the XDG cache directory
and XDG data directory are accessible. If `state.json` is found
in the cache directory but not the data directory, `doctor` MUST
report a warning with instructions to move it.

#### Scenario: Doctor detects misplaced state.json
- **GIVEN** `state.json` exists in `~/.cache/complytime/` but
  not in `~/.local/share/complytime/`
- **WHEN** user runs `complyctl doctor`
- **THEN** doctor reports a warning: state.json should be in
  the data directory, with a command to move it
