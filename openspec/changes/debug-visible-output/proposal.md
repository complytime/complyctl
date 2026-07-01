## Why

The `--debug` flag advertises `output debug logs` in help text, but running any
command with `--debug` produces no additional terminal output. Debug messages are
written only to `.complytime/complyctl.log`, which the flag never mentions. Users
have no way to discover where their diagnostics went. (Ref: complytime/complyctl#614)

## What Changes

- Wire `--debug` to also emit all log messages to stderr so users see
  diagnostics in the terminal when they ask for them.
- After the log file is opened, print a one-line pointer to the log file path on
  stderr (e.g., `Debug log: .complytime/complyctl.log`) so users know where the
  full structured log lives.
- Update the flag description from `"output debug logs"` to
  `"output debug logs to stderr and log file"` for accuracy.

## Capabilities

### New Capabilities

- `debug-stderr-output`: When `--debug` is active, duplicate log output to
  stderr via a TTY-aware tee writer and print a log-file location hint.

### Modified Capabilities

(none)

### Removed Capabilities

(none)

## Impact

- **Code**: `cmd/complyctl/cli/root.go` (lazyLogWriter, enableDebug, PersistentPreRun),
  `cmd/complyctl/cli/options.go` (flag description),
  `pkg/log/log.go` (new teeWriter type for TTY-aware multi-destination output).
- **UX**: Users running `--debug` will see colored debug output on stderr. Normal
  (non-debug) runs are unaffected. Log file behavior is unchanged.
- **Dependencies**: No new dependencies; uses stdlib `io` and `os` packages.
- **Breaking changes**: None. Existing log file content and location are preserved.

## Documentation Impact

- **CHANGELOG.md**: Entry under `### Fixed` for `--debug` flag producing visible
  stderr output (bug fix).
- **AGENTS.md**: Update Recent Changes section with `debug-visible-output` entry.
- **Website issue**: Exempt -- bug fix to existing flag behavior, not a new
  user-facing feature.
