## Context

The `--debug` flag sets the logger level to `hclog.Debug` via `enableDebug()` in
`cmd/complyctl/cli/root.go:80-84`, but the logger writes exclusively to a
`lazyLogWriter` that targets `.complytime/complyctl.log`. There is no path from
the logger to stderr or stdout. The flag description says `"output debug logs"`
which implies terminal visibility.

The logger is a `CharmHclog` adapter (`pkg/log/log.go`) wrapping a
`charmbracelet/log.Logger`. It already produces styled, color-coded output
suitable for terminal display. The styling infrastructure is in place but is
never seen by users because the output writer is a file.

**TTY detection constraint**: The vendored `termenv` v0.16.0 performs TTY
detection via a concrete `*os.File` type assertion (`termenv.go:35`):
```go
if f, ok := o.Writer().(*os.File); ok {
    return isatty.IsTerminal(f.Fd())
}
return false
```
Any custom writer type (including `io.MultiWriter` or a custom `teeWriter`)
will fail this assertion, causing `isTTY()` to return `false` and disabling
color output. The `Fd() uintptr` interface pattern does NOT work here --
`termenv` requires the writer to literally be `*os.File`. However, the charm
logger provides `SetColorProfile(profile)` to override the detected profile,
and `termenv` provides `WithTTY(v bool)` as an `OutputOption`.

## Goals / Non-Goals

**Goals:**

- When `--debug` is active, emit all log messages (debug, info, warn, error)
  to stderr so they are visible in the terminal alongside normal output.
- Print a one-line pointer to the log file path on stderr at startup when debug
  mode is active, so users know where the full log lives.
- Update the `--debug` flag description to accurately reflect the new behavior.
- Preserve existing log file behavior unchanged (all log messages still go to
  the file regardless of debug flag).
- Preserve TTY color detection for stderr output (colors when stderr is a
  terminal, plain text when redirected or `NO_COLOR` is set).

**Non-Goals:**

- Adding a `--verbose` or `--log-level` flag (future work).
- Changing the log format or adding JSON structured logging to stderr.
- Adding stderr output when `--debug` is not set. Normal runs remain quiet.
  The logger is intentionally file-only for normal runs.

## Decisions

### D1: Reconstruct the logger with a teeWriter + force color profile

When `--debug` is active, recreate the logger using `log.NewLogger(teeWriter)`
where `teeWriter` writes to both `os.Stderr` and the `lazyLogWriter` (log file).
After reconstruction, force the correct color profile based on stderr's TTY
status using the charm logger's `SetColorProfile()`.

The `teeWriter` is a simple struct implementing `io.Writer` that writes to a
primary (stderr) and secondary (log file) writer. It does NOT need to implement
`Fd()` -- color detection is handled separately via `SetColorProfile`.

**Why this approach:**

- *Alternative A: `io.MultiWriter` + `SetOutput`* -- `io.MultiWriter` is not
  `*os.File`, so `termenv` detects non-TTY and disables colors. `SetOutput`
  also re-creates the renderer. **Rejected.**
- *Alternative B: Custom writer with `Fd() uintptr`* -- `termenv` uses
  concrete `*os.File` type assertion, not an `Fd()` interface. The `Fd()`
  method would never be called. **Rejected.**
- *Alternative C: Second logger instance* -- Would require duplicating all
  `logger.Debug()` call sites or maintaining two logger references. Fragile.
  **Rejected.**
- *Alternative D: `termenv.WithTTY(true)` option* -- Works but requires
  modifying `NewLogger` to accept termenv options and thread them through
  `charmlog.NewWithOptions` -> `lipgloss.NewRenderer`. More invasive than
  post-construction `SetColorProfile`. **Rejected as unnecessarily complex.**

**Implementation**:

1. Add a `teeWriter` type to `pkg/log/` with `Write(p []byte)` that writes to
   both destinations. Error from the secondary (file) writer MUST NOT prevent
   output to the primary (stderr) writer -- best-effort file logging.
2. Add a `SetColorProfile(profile termenv.Profile)` method to `CharmHclog` that
   delegates to the underlying charm logger's `SetColorProfile`.
3. In `enableDebug()`, reconstruct the logger, set debug level, then probe
   stderr's TTY status:

```go
func enableDebug(opts *Common, lw io.Writer, stderrW *os.File) {
    if opts.Debug {
        tw := log.NewTeeWriter(stderrW, lw)
        logger = log.NewLogger(tw)
        logger.SetLevel(hclog.Debug)
        if isatty.IsTerminal(stderrW.Fd()) && !termenv.EnvNoColor() {
            if cl, ok := logger.(interface {
                SetColorProfile(termenv.Profile)
            }); ok {
                cl.SetColorProfile(termenv.ANSIColor)
            }
        }
    }
}
```

The color profile is only forced when stderr is a real TTY AND the user has not
set `NO_COLOR` or `CLICOLOR=0`. The `termenv.EnvNoColor()` function checks both
`NO_COLOR` (any non-empty value) and `CLICOLOR` (value `"0"`). The comma-ok
type assertion prevents panics if the logger type changes in the future.

Note: `stderrW` is `*os.File` (not `io.Writer`) so we can call `Fd()` on it
directly. In tests, use `os.NewFile(0, "test")` or skip color profile testing
and focus on write behavior.

### D2: Print log file hint to stderr

When debug mode is active and the log file path is resolved, print a single
line: `Debug log: <path>`. This goes to stderr (not stdout) so it does not
interfere with piped output or report formatting.

**Timing**: The hint is printed in `PersistentPreRun` after workspace resolution,
because the log file path depends on the resolved workspace directory. The
`enableDebug()` call is also moved after workspace resolution so the logger is
reconstructed with the correct `lazyLogWriter` before any debug messages are
emitted.

**Format stability**: The `Debug log:` prefix is informational output, not a
stable API contract. Downstream tools should not parse this line.

### D3: Update flag description text

Change from `"output debug logs"` to `"output debug logs to stderr and log file"`
in `options.go:25`. This accurately describes what the flag does.

### D4: Accept parameter for stderr writer to enable test isolation

The `enableDebug()` function currently operates on the package-level `logger`
variable (typed as `hclog.Logger`). To make the write behavior testable, the
function accepts the stderr writer and file writer as parameters. In production,
stderr is `os.Stderr` and the file writer is the `lazyLogWriter`. In tests, the
teeWriter's write-to-both behavior is tested at the `pkg/log` level using
`bytes.Buffer`; the `enableDebug()` integration is tested by verifying the
package-level `logger` produces output to a buffer after the call.

Tests for `enableDebug()` should save and restore the package-level `logger`
via `t.Cleanup` to prevent test pollution.

## Risks / Trade-offs

- **[Risk] Debug output on stderr may be noisy for long scans** -- Mitigation:
  This is the expected behavior when a user explicitly requests `--debug`. Users
  who want quiet operation simply omit the flag. The log file captures everything
  regardless. Debug output volume scales with scan complexity and provider
  verbosity; the log file hint directs users to the full log for post-hoc
  analysis.
- **[Risk] go-plugin provider subprocess logs appear on stderr** -- The
  provider manager passes the logger (now writing to tee-writer) to
  `goplugin.ClientConfig.Logger`. Provider stderr output that goes through
  go-hclog will now also appear on the user's terminal in debug mode. This is
  desirable for debugging provider issues but could include operational details
  (protocol negotiation, subprocess management). Mitigation: `--debug` is an
  explicit user opt-in; users should be aware debug output may contain
  operational details. No credentials flow through the logger -- credential
  errors log the error message, not the credential value.
- **[Risk] Color profile detection in CI** -- The `enableDebug()` function
  probes `isatty.IsTerminal(stderrW.Fd())` AND `termenv.EnvNoColor()` before
  forcing `ANSIColor`. In CI (non-TTY stderr), the probe returns false and no
  profile is forced, so output remains plain text. When `NO_COLOR` is set
  (even if stderr is a TTY), the profile is not forced, respecting the
  `no-color.org` standard. The `CI` environment variable also causes termenv
  to return non-TTY. No regression from current behavior.
- **[Risk] Existing integration test captures stderr with `--debug`** -- The
  integration test at `tests/integration_test.sh:522` uses `2>&1` to capture
  combined output. After this change, debug messages will appear in that
  capture. The test only checks for log file existence, not output content,
  so no functional breakage is expected. A task is included to audit this.
- **[Risk] `isatty` dependency** -- The `isatty` package is already vendored
  as a transitive dependency of `termenv`. Using it directly in
  `cmd/complyctl/cli/` adds a direct import but no new binary dependency.

## Open Questions

(none -- all questions resolved during design)
