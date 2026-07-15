## ADDED Requirements

### FR-009: Deprecation warning for legacy directory
When `~/.complytime/` exists AND (the XDG cache directory does
not exist OR the XDG data directory does not exist), complyctl
MUST print a one-time deprecation warning to stderr on any
command that resolves user-scoped paths. The warning MUST include
the legacy path, the new XDG paths, and explicit instructions
distinguishing between re-fetchable cache data and
non-recoverable state data. The warning MUST instruct users to
copy `state.json` from `~/.complytime/state.json` to the new
data directory before removing the legacy directory, and MUST
warn that verification metadata will be lost if `state.json` is
not preserved.

#### Scenario: Legacy directory detected on first XDG run
- **GIVEN** `~/.complytime/` exists and `~/.cache/complytime/`
  does not exist
- **WHEN** complyctl runs any command
- **THEN** complyctl prints a deprecation warning to stderr
  containing:
  - The legacy path (`~/.complytime/`)
  - The new cache path (e.g. `~/.cache/complytime/`)
  - The new data path (e.g. `~/.local/share/complytime/`)
  - Instructions to copy `state.json` and providers to the
    data directory
  - A note that policies and complypacks are re-fetchable via
    `complyctl get`

#### Scenario: No warning when XDG directories already exist
- **GIVEN** `~/.complytime/` exists and both
  `~/.cache/complytime/` and `~/.local/share/complytime/`
  exist
- **WHEN** complyctl runs any command
- **THEN** complyctl does not print a deprecation warning

#### Scenario: No warning when legacy directory absent
- **GIVEN** `~/.complytime/` does not exist
- **WHEN** complyctl runs any command
- **THEN** complyctl does not print a deprecation warning

#### Scenario: Warning when data directory missing
- **GIVEN** `~/.complytime/` exists and `~/.cache/complytime/`
  exists but `~/.local/share/complytime/` does not exist
- **WHEN** complyctl runs any command
- **THEN** complyctl prints a deprecation warning focused on
  moving state.json and providers to the data directory

#### Scenario: Legacy directory is a symlink
- **GIVEN** `~/.complytime` is a symlink to another location
- **WHEN** complyctl checks for legacy directory
- **THEN** the symlink target is followed (standard `os.Stat`
  behavior) and the deprecation check proceeds normally

#### Scenario: Legacy directory is empty
- **GIVEN** `~/.complytime/` exists but contains no files
- **WHEN** complyctl runs any command
- **THEN** the deprecation warning still fires (the directory
  exists) but users can simply remove the empty directory

### FR-010: Warning follows existing deprecation pattern
The deprecation warning MUST follow the same pattern as the
existing `printDeprecationWarning()` function in `workspace.go`
-- printed to stderr via `fmt.Fprintf(os.Stderr, ...)`, prefixed
with `"WARNING:"`, and emitted only once per invocation.
The once-per-invocation guarantee MUST be achieved structurally
by wiring the check into `PersistentPreRun` in `root.go`, which
executes exactly once per CLI invocation.

#### Scenario: Warning output format
- **GIVEN** the deprecation condition is met
- **WHEN** the deprecation warning is triggered
- **THEN** the message starts with `WARNING:` and is written
  to stderr

#### Scenario: Warning does not repeat in same invocation
- **GIVEN** `PersistentPreRun` executes once per CLI invocation
- **WHEN** complyctl runs a command
- **THEN** the deprecation warning is printed at most once

### FR-011: No automatic file migration
complyctl MUST NOT automatically move, copy, or symlink files
from `~/.complytime/` to XDG directories. Users MUST perform
migration manually based on the deprecation warning instructions.

#### Scenario: Legacy files remain untouched
- **GIVEN** `~/.complytime/` exists with policies, providers,
  and state.json
- **WHEN** complyctl runs any command
- **THEN** no files in `~/.complytime/` are moved, copied,
  or deleted

#### Scenario: Fresh fetch after migration
- **GIVEN** user removes `~/.complytime/` and runs
  `complyctl get`
- **THEN** policies and complypacks are fetched into the new
  XDG cache directory
